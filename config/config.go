package config

import (
	"log"
	"os"
	"sync"

	"github.com/bellapacxx/bingo-backend/models"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	DB   *gorm.DB
	once sync.Once
)

// SetupDatabase initializes the DB only once and runs migrations
func SetupDatabase() *gorm.DB {
	once.Do(func() {
		// Load .env
		if err := godotenv.Load(); err != nil {
			log.Println("[INFO] No .env file found, reading environment variables")
		}

		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Fatal("[FATAL] DATABASE_URL is required in .env or environment")
		}

		// Connect to DB
		db, err := gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true,
		}), &gorm.Config{
			PrepareStmt: false,
		})
		if err != nil {
			log.Fatalf("[FATAL] Failed to connect to DB: %v", err)
		}
		DB = db

		// Run migrations
		if err := db.AutoMigrate(
			&models.User{},
			&models.Game{},
			&models.Card{},
			&models.Transaction{},
		); err != nil {
			log.Fatalf("[FATAL] Migration failed: %v", err)
		}

		log.Println("✅ Database connected and migration completed")
	})

	return DB
}
