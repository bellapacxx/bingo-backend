package services

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/gorilla/websocket"
	"gorm.io/datatypes"
)

// ------------------------
// Bingo card definitions
// ------------------------
type BingoCard struct {
	B      []int `json:"B"`
	I      []int `json:"I"`
	N      []int `json:"N"`
	G      []int `json:"G"`
	O      []int `json:"O"`
	CardID int   `json:"card_id"`
}

var (
	Cards       []BingoCard
	cardsMu     sync.Mutex
	selectedIDs map[int]bool
)

func LoadCards() {
	data, err := os.ReadFile("cards.json")
	if err != nil {
		log.Fatalf("Failed to read cards.json: %v", err)
	}
	if err := json.Unmarshal(data, &Cards); err != nil {
		log.Fatalf("Failed to unmarshal cards.json: %v", err)
	}
	selectedIDs = make(map[int]bool)
	log.Printf("Loaded %d bingo cards", len(Cards))
}

func GetAvailableCards() []BingoCard {
	cardsMu.Lock()
	defer cardsMu.Unlock()

	available := []BingoCard{}
	for _, card := range Cards {
		if !selectedIDs[card.CardID] {
			available = append(available, card)
		}
	}
	return available
}

func MarkCardSelected(cardID int) {
	cardsMu.Lock()
	defer cardsMu.Unlock()
	selectedIDs[cardID] = true
}

// ------------------------
// Lobby service
// ------------------------
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
	LoadCards() // Load cards on startup
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
func (l *Lobby) SelectCard(userID uint, cardID int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Status != "waiting" && l.Status != "countdown" {
		return
	}

	// find card
	var numbers []int
	found := false
	for _, c := range Cards {
		if c.CardID == cardID && !selectedIDs[cardID] {
			numbers = append(numbers, c.B...)
			numbers = append(numbers, c.I...)
			numbers = append(numbers, c.N...)
			numbers = append(numbers, c.G...)
			numbers = append(numbers, c.O...)
			found = true
			break
		}
	}
	if !found {
		return
	}

	l.Cards[userID] = numbers
	MarkCardSelected(cardID)
	l.sendState()
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

	var lastGame models.Game
	result := config.DB.Where("stake = ?", l.Stake).Order("round_number DESC").First(&lastGame)
	nextRound := 1
	if result.Error == nil {
		nextRound = lastGame.RoundNumber + 1
	}

	game := models.Game{
		Stake:       l.Stake,
		Status:      "in_progress",
		StartTime:   time.Now(),
		RoundNumber: nextRound,
		NumbersJSON: datatypes.JSON([]byte("[]")),
	}
	if err := config.DB.Create(&game).Error; err != nil {
		log.Printf("[Lobby] Failed to create game: %v", err)
		return
	}

	l.mu.Lock()
	l.currentGame = &game
	l.sendState()
	l.mu.Unlock()

	go func() {
		bingoNumbers := generateBingoNumbers()
		for _, num := range bingoNumbers {
			l.mu.Lock()
			l.NumbersDrawn = append(l.NumbersDrawn, num)

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
	availableCards := GetAvailableCards()
	for _, conn := range l.Clients {
		if conn == nil {
			continue
		}
		if err := conn.WriteJSON(map[string]interface{}{
			"stake":          l.Stake,
			"status":         l.Status,
			"countdown":      l.Countdown,
			"cards":          l.Cards,
			"numbersDrawn":   l.NumbersDrawn,
			"selectedCards":  l.Cards,        // already selected by users
			"availableCards": availableCards, // cards available for selection
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
