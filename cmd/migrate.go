package main

import (
	"log"

	"github.com/bellapacxx/bingo-backend/config"
)

func main() {
	db := config.SetupDatabase() // connects + migrates
	_ = db
	log.Println("✅ Database migration completed successfully")
}
