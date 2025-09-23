package services

import (
	"log"
	"net/http"
	"strconv"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func HandleWebSocket(c *gin.Context) {
	stake, _ := strconv.Atoi(c.Param("stake"))
	lobby, ok := Lobbies[stake]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "lobby not found"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] upgrade error: %v", err)
		return
	}

	userTelegramIDStr := c.Query("telegram_id")
	if userTelegramIDStr == "" {
		log.Println("[WS] missing telegram_id")
		conn.Close()
		return
	}
	userTelegramID, _ := strconv.ParseInt(userTelegramIDStr, 10, 64)

	var user models.User
	if err := config.DB.Where("telegram_id = ?", userTelegramID).First(&user).Error; err != nil {
		log.Printf("[WS] user not found: %v", err)
		conn.Close()
		return
	}

	client := &Client{
		userID: user.ID,
		conn:   conn,
		lobby:  lobby,
		send:   make(chan []byte, 32),
	}
	log.Printf("[WS] New client: userID=%d, telegramID=%d, lobby=%d", user.ID, userTelegramID, lobby.Stake)

	lobby.addClient(client)
}
