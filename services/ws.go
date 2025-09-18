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

// HandleWebSocket handles the lobby WebSocket connection
func HandleWebSocket(c *gin.Context) {
	stakeStr := c.Param("stake")
	stake, err := strconv.Atoi(stakeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid stake"})
		return
	}

	lobby, ok := Lobbies[stake] // Lobbies from the same package
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "lobby not found"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close()

	// Lookup user from database using Telegram ID query param
	userTelegramIDStr := c.Query("telegram_id")
	if userTelegramIDStr == "" {
		log.Println("[WS] Missing telegram_id")
		return
	}

	userTelegramID, err := strconv.ParseInt(userTelegramIDStr, 10, 64)
	if err != nil {
		log.Printf("[WS] Invalid telegram_id: %v", err)
		return
	}

	var user models.User
	if err := config.DB.Where("telegram_id = ?", userTelegramID).First(&user).Error; err != nil {
		log.Printf("[WS] User not found in DB: %v", err)
		return
	}

	// Join lobby using the database user ID
	lobby.Join(user.ID, conn)
	log.Printf("[WS] User %d (%s) joined lobby %d", user.ID, user.Name, stake)

	// Listen for messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("[WS] User %d disconnected: %v", user.ID, err)
			lobby.Leave(user.ID)
			break
		}

		// Handle card selection by card_id
		if action, ok := msg["action"].(string); ok && action == "select_card" {
			if cardIDRaw, ok := msg["card_id"].(float64); ok {
				cardID := int(cardIDRaw)

				// Find card from preloaded Cards in the same package
				var selectedCard []int
				found := false
				for _, card := range Cards { // <- Cards is from the merged lobby.go
					if card.CardID == cardID {
						// Flatten B-I-N-G-O columns into a single slice
						selectedCard = append(selectedCard, card.B...)
						selectedCard = append(selectedCard, card.I...)
						selectedCard = append(selectedCard, card.N...)
						selectedCard = append(selectedCard, card.G...)
						selectedCard = append(selectedCard, card.O...)
						found = true
						break
					}
				}

				if found {

					log.Printf("[WS] User %d selected card %d", user.ID, cardID)
				} else {
					log.Printf("[WS] Card ID %d not found", cardID)
				}
			}
		}
	}
}
