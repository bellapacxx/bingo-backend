package routes

import (
	"github.com/bellapacxx/bingo-backend/controllers"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	api := r.Group("/api")

	// ----------------------
	// User routes
	// ----------------------
	api.POST("/users", controllers.RegisterUser)        // Register user
	api.GET("/users/:telegram_id", controllers.GetUser) // Get user by Telegram ID

	// ----------------------
	// Game routes
	// ----------------------
	api.GET("/games", controllers.ListGames)             // List all games
	api.POST("/games/:id/join", controllers.JoinGame)    // Join a game
	api.GET("/games/:id/lobby", controllers.LobbyStatus) // Get lobby status

	// ----------------------
	// Card routes
	// ----------------------
	api.POST("/cards", controllers.BuyCard) // Buy bingo card

	// ----------------------
	// Transaction routes
	// ----------------------
	api.POST("/deposit", controllers.Deposit)   // Deposit funds
	api.POST("/withdraw", controllers.Withdraw) // Withdraw funds
}
