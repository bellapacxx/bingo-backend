package services

import (
	"encoding/json"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/gorilla/websocket"
	"gorm.io/datatypes"
)

type Lobby struct {
	Stake        int
	Clients      map[uint]*websocket.Conn
	Cards        map[uint][]int
	Status       string // "waiting" | "countdown" | "in_progress"
	Countdown    int
	NumbersDrawn []string
	mu           sync.Mutex
	currentGame  *models.Game
}

// Predefined stakes
var Stakes = []int{10, 20, 50, 100}
var Lobbies = map[int]*Lobby{}

func InitLobbyService() {
	for _, stake := range Stakes {
		lobby := &Lobby{
			Stake:   stake,
			Clients: make(map[uint]*websocket.Conn),
			Cards:   make(map[uint][]int),
			Status:  "waiting",
		}
		Lobbies[stake] = lobby
		lobby.RunAutoRounds()
	}
}

// ------------------------
// User joins/leaves
// ------------------------
func (l *Lobby) Join(userID uint, conn *websocket.Conn) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.Clients[userID] = conn
	l.sendState()
}

func (l *Lobby) Leave(userID uint) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.Clients, userID)
	delete(l.Cards, userID)
	l.sendState()
}

// ------------------------
// Card selection
// ------------------------
func (l *Lobby) SelectCard(userID uint, numbers []int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Status == "waiting" || l.Status == "countdown" {
		l.Cards[userID] = numbers
		l.sendState()
	}
}

// ------------------------
// Automatic rounds
// ------------------------
func (l *Lobby) RunAutoRounds() {
	go func() {
		for {
			l.startCountdown(30)

			for {
				l.mu.Lock()
				count := l.Countdown
				l.mu.Unlock()
				if count <= 0 {
					break
				}
				time.Sleep(time.Second)
			}

			l.startRound()

			for {
				l.mu.Lock()
				status := l.Status
				l.mu.Unlock()
				if status != "in_progress" {
					break
				}
				time.Sleep(time.Second)
			}
		}
	}()
}

// ------------------------
// Countdown
// ------------------------
func (l *Lobby) startCountdown(seconds int) {
	l.mu.Lock()
	l.Status = "countdown"
	l.Countdown = seconds
	l.sendState()
	l.mu.Unlock()

	ticker := time.NewTicker(time.Second)
	go func() {
		for range ticker.C {
			l.mu.Lock()
			l.Countdown--
			l.sendState()
			if l.Countdown <= 0 {
				ticker.Stop()
				l.mu.Unlock()
				break
			}
			l.mu.Unlock()
		}
	}()
}

// ------------------------
// Start round
// ------------------------
func (l *Lobby) startRound() {
	l.mu.Lock()
	l.Status = "in_progress"
	l.NumbersDrawn = []string{}
	l.mu.Unlock()

	// Determine next round number
	var lastGame models.Game
	result := config.DB.Where("stake = ?", l.Stake).Order("round_number DESC").First(&lastGame)
	nextRound := 1
	if result.Error == nil {
		nextRound = lastGame.RoundNumber + 1
	}

	// Create new game (NumbersJSON for DB storage)
	game := models.Game{
		Stake:       l.Stake,
		Status:      "in_progress",
		StartTime:   time.Now(),
		RoundNumber: nextRound,
		NumbersJSON: datatypes.JSON([]byte("[]")), // store empty JSON
	}
	if err := config.DB.Create(&game).Error; err != nil {
		log.Printf("[Lobby] Failed to create game: %v", err)
		return
	}

	l.mu.Lock()
	l.currentGame = &game
	l.sendState()
	l.mu.Unlock()

	// Draw numbers asynchronously
	go func() {
		bingoNumbers := generateBingoNumbers()
		for _, num := range bingoNumbers {
			l.mu.Lock()
			l.NumbersDrawn = append(l.NumbersDrawn, num)

			// Marshal slice to JSON before saving in DB
			numJSON, _ := json.Marshal(l.NumbersDrawn)
			if l.currentGame != nil {
				l.currentGame.NumbersJSON = datatypes.JSON(numJSON)
				if err := config.DB.Save(l.currentGame).Error; err != nil {
					log.Printf("[Lobby] Failed to update game numbers: %v", err)
				}
			}

			l.sendState()
			l.mu.Unlock()
			time.Sleep(50 * time.Millisecond)
		}
		l.endRound()
	}()
}

// ------------------------
// End round
// ------------------------
func (l *Lobby) endRound() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.currentGame != nil {
		l.currentGame.Status = "finished"
		l.currentGame.EndTime = time.Now()
		if err := config.DB.Save(l.currentGame).Error; err != nil {
			log.Printf("[Lobby] Failed to finish game: %v", err)
		}

		// Save cards
		for userID, numbers := range l.Cards {
			card := models.Card{
				UserID:  userID,
				GameID:  l.currentGame.ID,
				Numbers: numbers,
			}
			if err := config.DB.Create(&card).Error; err != nil {
				log.Printf("[Lobby] Failed to save card: %v", err)
			}
		}
	}

	l.Cards = make(map[uint][]int)
	l.Status = "waiting"
	l.Countdown = 30
	l.NumbersDrawn = []string{}
	l.currentGame = nil
	l.sendState()
}

// ------------------------
// Broadcast
// ------------------------
func (l *Lobby) sendState() {
	for _, conn := range l.Clients {
		if conn == nil {
			continue
		}
		if err := conn.WriteJSON(map[string]interface{}{
			"stake":        l.Stake,
			"status":       l.Status,
			"countdown":    l.Countdown,
			"cards":        l.Cards,
			"numbersDrawn": l.NumbersDrawn,
		}); err != nil {
			log.Printf("[Lobby] Failed to send state: %v", err)
		}
	}
}

// ------------------------
// Generate bingo numbers
// ------------------------
func generateBingoNumbers() []string {
	letters := []string{"B", "I", "N", "G", "O"}
	numbers := make([]int, 75)
	for i := 0; i < 75; i++ {
		numbers[i] = i + 1
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(numbers), func(i, j int) { numbers[i], numbers[j] = numbers[j], numbers[i] })

	result := make([]string, 0, 75)
	for _, n := range numbers {
		letter := letters[(n-1)/15]
		result = append(result, letter+strconv.Itoa(n))
	}
	return result
}
