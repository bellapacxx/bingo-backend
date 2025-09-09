package config

import (
	"fmt"
	"log"
	"os"

	"github.com/bellapacxx/bingo-backend/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("[FATAL] DATABASE_URL is required in .env")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("[FATAL] Failed to connect to database: %v", err)
	}

	// Auto migrate models
	err = db.AutoMigrate(&models.User{}, &models.Game{}, &models.Card{}, &models.Transaction{})
	if err != nil {
		log.Fatalf("[FATAL] AutoMigrate failed: %v", err)
	}

	DB = db
	fmt.Println("✅ Database connected and migrated")
}
