package services

import (
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Lobby struct {
	GameID       uint
	Players      map[int64]*websocket.Conn // TelegramID -> connection
	Status       string                    // waiting, countdown, playing
	Mutex        sync.Mutex
	Countdown    int // seconds
	DrawnNumbers []int
}

var Lobbies map[uint]*Lobby
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func InitLobbyService() {
	Lobbies = make(map[uint]*Lobby)
	log.Println("[INFO] Lobby service initialized")
}

// HandleWebSocket upgrades HTTP to WebSocket and registers player
func HandleWebSocket(w http.ResponseWriter, r *http.Request, gameIDStr string) {
	gameID, _ := strconv.Atoi(gameIDStr)
	tidStr := r.URL.Query().Get("telegram_id")
	telegramID, _ := strconv.ParseInt(tidStr, 10, 64)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	lobby := getOrCreateLobby(uint(gameID))
	lobby.Mutex.Lock()
	lobby.Players[telegramID] = conn
	lobby.Mutex.Unlock()

	log.Printf("Player %d joined lobby %d", telegramID, gameID)

	// Start countdown if min players reached
	go lobby.checkStartCountdown()

	// Listen to client (optional, can receive chat or commands)
	go listenPlayer(lobby, telegramID, conn)
}

// getOrCreateLobby returns existing or creates new lobby
func getOrCreateLobby(gameID uint) *Lobby {
	if l, ok := Lobbies[gameID]; ok {
		return l
	}
	l := &Lobby{
		GameID:       gameID,
		Players:      make(map[int64]*websocket.Conn),
		Status:       "waiting",
		Countdown:    10, // default countdown in seconds
		DrawnNumbers: []int{},
	}
	Lobbies[gameID] = l
	return l
}

// checkStartCountdown auto-starts game if min players reached
func (l *Lobby) checkStartCountdown() {
	l.Mutex.Lock()
	if l.Status != "waiting" || len(l.Players) < 2 { // min players
		l.Mutex.Unlock()
		return
	}
	l.Status = "countdown"
	l.Mutex.Unlock()

	for i := l.Countdown; i > 0; i-- {
		l.broadcast(map[string]interface{}{
			"event":   "countdown",
			"seconds": i,
		})
		time.Sleep(1 * time.Second)
	}

	// Start game
	l.Mutex.Lock()
	l.Status = "playing"
	l.Mutex.Unlock()

	l.startGame()
}

// startGame draws numbers every 2 seconds
func (l *Lobby) startGame() {
	allNumbers := rand.Perm(75) // Bingo numbers 0..74
	for _, n := range allNumbers {
		time.Sleep(2 * time.Second)
		l.Mutex.Lock()
		l.DrawnNumbers = append(l.DrawnNumbers, n+1)
		l.broadcast(map[string]interface{}{
			"event":         "number_drawn",
			"number":        n + 1,
			"drawn_numbers": l.DrawnNumbers,
		})
		l.Mutex.Unlock()
	}
	l.Mutex.Lock()
	l.Status = "finished"
	l.broadcast(map[string]interface{}{
		"event": "game_finished",
	})
	l.Mutex.Unlock()
}

// broadcast sends a message to all connected players
func (l *Lobby) broadcast(msg interface{}) {
	for tid, conn := range l.Players {
		if conn != nil {
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("Error broadcasting to player %d: %v", tid, err)
			}
		}
	}
}

// listenPlayer reads messages from player (optional)
func listenPlayer(l *Lobby, telegramID int64, conn *websocket.Conn) {
	defer func() {
		conn.Close()
		l.Mutex.Lock()
		delete(l.Players, telegramID)
		l.Mutex.Unlock()
	}()

	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			log.Printf("Player %d disconnected: %v", telegramID, err)
			break
		}
		// handle client messages if needed
	}
}
