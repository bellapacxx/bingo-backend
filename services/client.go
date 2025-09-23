package services

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	userID uint
	conn   *websocket.Conn
	lobby  *Lobby
	send   chan []byte
	once   sync.Once
}

func (c *Client) Close() {
	c.once.Do(func() {
		close(c.send)
		c.conn.Close()
	})
}

// --------------------
// Client read/write pumps
// --------------------
func (c *Client) readPump() {
	defer func() {
		c.lobby.removeClient(c.userID)
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("[Client %d] disconnected normally", c.userID)
			} else {
				log.Printf("[Client %d] read error: %v", c.userID, err)
			}
			return
		}

		func(msg []byte) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Client %d] recovered from panic: %v", c.userID, r)
				}
			}()

			log.Printf("[Client %d] raw message: %s", c.userID, string(msg))
			var data map[string]any
			if err := json.Unmarshal(msg, &data); err != nil {
				log.Printf("[Client %d] invalid message: %v", c.userID, err)
				return
			}

			switch data["action"] {
			case "select_card":
				cardIDFloat, ok := data["card_id"].(float64)
				if !ok {
					log.Printf("[Client %d] invalid card_id: %v", c.userID, data["card_id"])
					return
				}
				cardID := int(cardIDFloat)
				if c.lobby.SelectCard(c.userID, cardID) {
					log.Printf("[Client %d] selected card %d", c.userID, cardID)
				} else {
					log.Printf("[Client %d] failed to select card %d", c.userID, cardID)
				}
			case "bingo":
				c.lobby.CheckBingo(c.userID)
			default:
				log.Printf("[Client %d] unknown action: %v", c.userID, data["action"])
			}
		}(message)
	}
}

func (c *Client) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("[Client %d] write error: %v", c.userID, err)
			return
		}
	}
}
