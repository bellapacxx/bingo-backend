package config

import (
	"log"
	"os"

	"github.com/bellapacxx/bingo-backend/config/configdb"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

func main() {
	// Load env
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] No .env file found, reading environment variables")
	}

	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("[FATAL] DATABASE_URL is required")
	}

	// Connect to DB
	configdb.ConnectDB()
	db := configdb.DB

	migrate(db)
}

func migrate(db *gorm.DB) {
	err := db.AutoMigrate(
		&models.User{},
		&models.Game{},
		&models.Card{},
		&models.Transaction{},
	)
	if err != nil {
		log.Fatalf("[FATAL] Migration failed: %v", err)
	}
	log.Println("✅ Database migration completed")
}
