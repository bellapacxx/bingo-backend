package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/bingo-backend/config"
	"github.com/bingo-backend/routes"
	"github.com/bingo-backend/services"

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
	r.GET("/ws/:game_id", func(c *gin.Context) {
		gameID := c.Param("game_id")
		services.HandleWebSocket(c.Writer, c.Request, gameID)
	})

	return r
}

func main() {
	// Load env variables
	initEnv()

	// Connect to database
	config.ConnectDB()

	// Initialize in-memory lobby service
	services.InitLobbyService()

	// Setup Gin router
	router := setupRouter()

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = config.PORT // default from config
	}

	log.Printf("🚀 Bingo Backend server starting on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("[FATAL] Failed to start server: %v", err)
	}
}
