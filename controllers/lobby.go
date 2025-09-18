package controllers

import (
	"net/http"
	"strconv"

	"github.com/bellapacxx/bingo-backend/services"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// TODO: restrict this in production to your domains
		return true
	},
}

// LobbyWebSocket handles user joining a stake lobby via WebSocket
func LobbyWebSocket(c *gin.Context) {
	// --- Parse stake ---
	stakeParam := c.Param("stake")
	stake, err := strconv.Atoi(stakeParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stake"})
		return
	}

	lobby, ok := services.Lobbies[stake]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "stake not supported"})
		return
	}

	// --- Parse user ID ---
	userIDStr := c.Query("user")
	if userIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user query param"})
		return
	}
	userID64, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user ID"})
		return
	}
	userID := uint(userID64)

	// --- Upgrade to WebSocket ---
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade"})
		return
	}

	// --- Join lobby ---
	lobby.Join(userID, conn)

	// --- Keep connection alive ---
	for {
		// We don’t really care about incoming messages yet,
		// but we must read them to keep the connection open.
		_, _, err := conn.ReadMessage()
		if err != nil {
			lobby.Leave(userID)
			conn.Close()
			break
		}
	}
}
