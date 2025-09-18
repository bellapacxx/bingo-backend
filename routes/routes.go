package routes

import (
	"github.com/bellapacxx/bingo-backend/controllers"
	"github.com/bellapacxx/bingo-backend/services"
	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	api := r.Group("/api")

	// ----------------------
	// User routes
	// ----------------------
	api.POST("/users", controllers.RegisterUser)                  // Register user
	api.GET("/users/:telegram_id", controllers.GetUser)           // Get user by Telegram ID
	api.PUT("/users/:telegram_id/phone", controllers.UpdatePhone) // Update phone number

	// ----------------------
	// Game routes
	// ----------------------
	api.GET("/games", controllers.ListGames)             // List all games
	api.GET("/games/:id", controllers.GetGame)           // Get single game info
	api.POST("/games/:id/join", controllers.JoinGame)    // Join a game
	api.GET("/games/:id/lobby", controllers.LobbyStatus) // Get lobby status

	// ----------------------
	// Card/Ticket routes
	// ----------------------
	api.POST("/tickets", controllers.BuyTicket)                         // Buy bingo card/ticket
	api.GET("/tickets/user/:telegram_id", controllers.GetTicketsByUser) // Get user's tickets

	// ----------------------
	// Transaction routes
	// ----------------------
	api.POST("/deposit", controllers.Deposit)   // Deposit funds
	api.POST("/withdraw", controllers.Withdraw) // Withdraw funds

	// ----------------------
	// Lobby WebSocket
	// ----------------------
	api.GET("/lobby/:stake", services.HandleWebSocket)

	// ----------------------
	// Health check
	// ----------------------
	api.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})
}
