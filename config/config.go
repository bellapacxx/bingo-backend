package config

import (
	"log"
	"os"

	"github.com/bellapacxx/bingo-backend/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SetupDatabase connects to DB and runs migrations
func SetupDatabase() *gorm.DB {
	// Load env
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] No .env file found, reading environment variables")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("[FATAL] DATABASE_URL is required in .env")
	}

	// Connect to DB
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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

	log.Println("✅ Database migration completed")
	return db
}
