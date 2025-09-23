package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/routes"
	"github.com/bellapacxx/bingo-backend/services"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// initEnv loads .env file and validates required vars
func initEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("[INFO] No .env file found, reading environment variables")
	}

	if os.Getenv("DATABASE_URL") == "" {
		log.Fatal("[FATAL] DATABASE_URL is required in .env or environment")
	}
}

// setupRouter initializes Gin routes and middleware
func setupRouter() *gin.Engine {
	r := gin.Default()

	// Middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Setup REST routes
	routes.SetupRoutes(r)

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "timestamp": time.Now()})
	})

	// WebSocket lobby endpoint
	r.GET("/ws/:stake", services.HandleWebSocket)

	return r
}

func main() {
	r := gin.Default()
	// Load env variables
	initEnv()

	// Connect to database
	config.SetupDatabase()

	// Initialize in-memory lobby service
	services.InitLobbyService()
	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000"}, // your frontend origin
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	// Setup Gin router
	router := setupRouter()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "4000" // default from config
	}

	log.Printf("🚀 Bingo Backend server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("[FATAL] Failed to start server: %v", err)
	}
}
