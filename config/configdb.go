package config

import "os"

var (
	DB_URL = os.Getenv("DATABASE_URL")
	PORT   = "4000"
)
