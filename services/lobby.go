package services

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	DefaultCountdownSec = 30
	DrawIntervalMS      = 200 // 1 number per 200ms
)

type Lobby struct {
	Stake        int
	clients      map[uint]*Client
	Cards        map[uint][]int
	CardIDs      map[uint]int
	selectedIDs  map[int]bool
	Status       string
	Countdown    int
	NumbersDrawn []string
	roundDone    chan struct{}

	mu          sync.RWMutex
	currentGame *models.Game
	// New: store current round winner
	BingoWinner       *uint
	BingoWinnerCardID *int // cardID ✅
	CheckedUsers      map[uint]bool
}

var (
	Lobbies   = make(map[int]*Lobby)
	LobbiesMu sync.Mutex
	Stakes    = []int{10, 20, 50, 100}
)

func InitLobbyService() {
	LoadCards()
	for _, stake := range Stakes {
		l := &Lobby{
			Stake:       stake,
			clients:     make(map[uint]*Client),
			Cards:       make(map[uint][]int),
			CardIDs:     make(map[uint]int),
			selectedIDs: make(map[int]bool),
			Status:      "waiting",
			Countdown:   DefaultCountdownSec,
			roundDone:   make(chan struct{}, 1),
		}
		Lobbies[stake] = l
		go l.RunAutoRounds()
	}
	log.Printf("[Init] Started %d lobbies", len(Lobbies))
}

// -------------------- Client management --------------------
func (l *Lobby) addClient(c *Client) {
	l.mu.Lock()
	if old, ok := l.clients[c.userID]; ok {
		old.Close() // safe closure
	}
	l.clients[c.userID] = c
	l.mu.Unlock()

	go c.writePump()
	go c.readPump()

	log.Printf("[Lobby %d] user %d joined (total=%d)", l.Stake, c.userID, l.clientCount())
	go l.broadcastState()
}

func (l *Lobby) removeClient(userID uint) {
	l.mu.Lock()
	client, ok := l.clients[userID]
	if ok {
		delete(l.clients, userID)
		client.Close() // safe closure
	}
	if cardID, ok := l.CardIDs[userID]; ok {
		delete(l.selectedIDs, cardID)
		delete(l.CardIDs, userID)
	}
	delete(l.Cards, userID)
	l.mu.Unlock()

	l.broadcastState()
}

func (l *Lobby) clientCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.clients)
}

// -------------------- Card selection --------------------
func (l *Lobby) canSelectCard() bool {
	return l.Status == "waiting" || l.Status == "countdown"
}

func (l *Lobby) SelectCard(userID uint, cardID int) bool {
	// Step 1: Read the global Cards slice safely
	// 1️⃣ Fetch user from DB
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("[Lobby %d] User %d not found", l.Stake, userID)
			return false
		}
		log.Printf("[Lobby %d] DB error fetching user %d: %v", l.Stake, userID, err)
		return false
	}

	// 2️⃣ Check balance
	if user.Balance < float64(l.Stake) {
		l.notifyUser(userID, "Insufficient balance to select this card.")
		log.Printf("[Lobby %d] User %d cannot select card %d: insufficient balance %.2f < %d", l.Stake, userID, cardID, user.Balance, l.Stake)
		return false
	}
	var numbers []int
	cardsMu.RLock()
	for _, c := range Cards {
		if c.CardID == cardID {
			numbers = append(numbers, c.B...)
			numbers = append(numbers, c.I...)
			numbers = append(numbers, c.N...)
			numbers = append(numbers, c.G...)
			numbers = append(numbers, c.O...)
			break
		}
	}
	cardsMu.RUnlock()

	if len(numbers) == 0 {
		log.Printf("[Lobby %d] invalid cardID %d", l.Stake, cardID)
		return false
	}

	// Step 2: Lock the lobby to safely update internal state
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if card selection is allowed
	if !l.canSelectCard() {
		log.Printf("[Lobby %d] User %d tried to select card %d but round in progress", l.Stake, userID, cardID)
		return false
	}

	// Check if the card is already taken
	if l.selectedIDs[cardID] {
		log.Printf("[Lobby %d] Card %d already taken", l.Stake, cardID)
		return false
	}

	// Update lobby maps
	l.Cards[userID] = numbers
	l.CardIDs[userID] = cardID
	l.selectedIDs[cardID] = true

	log.Printf("[Lobby %d] User %d selected card %d", l.Stake, userID, cardID)

	// Step 3: Unlock before broadcasting to prevent deadlocks
	l.mu.Unlock()
	l.broadcastState()
	l.mu.Lock() // relock to satisfy defer
	return true
}

func (l *Lobby) CheckBingo(userID uint) bool {
	l.mu.Lock()
	if l.CheckedUsers == nil {
		l.CheckedUsers = make(map[uint]bool)
	}

	// ✅ Block if user already checked this round
	if l.CheckedUsers[userID] {
		l.notifyUser(userID, "⚠️ You already checked Bingo this round. You cannot check again.")
		log.Printf("[Lobby %d] User %d already checked Bingo this round", l.Stake, userID)
		l.mu.Unlock()
		return false
	}

	// mark as checked
	l.CheckedUsers[userID] = true
	l.mu.Unlock()

	l.mu.RLock()
	numbers, ok := l.Cards[userID]
	drawnNums := append([]string(nil), l.NumbersDrawn...) // copy safely
	l.mu.RUnlock()

	log.Printf("[Lobby %d] checking bingo for user %d", l.Stake, userID)
	if !ok {
		log.Printf("[Lobby %d] User %d tried Bingo without a card", l.Stake, userID)
		return false
	}

	// --- Build drawn set ---
	drawnSet := make(map[int]bool, len(drawnNums))
	for _, n := range drawnNums {
		if num, err := strconv.Atoi(n); err == nil {
			drawnSet[num] = true
		}
	}

	// --- Build 5x5 grid ---
	grid := make([][]int, 5)
	for i := 0; i < 5; i++ {
		grid[i] = numbers[i*5 : (i+1)*5]
	}

	// --- Check patterns ---
	if hasBingo(grid, drawnSet) {
		log.Printf("[Lobby %d] User %d claims BINGO!", l.Stake, userID)

		// ✅ store winner quickly
		l.mu.Lock()
		l.BingoWinner = &userID
		if cid, ok := l.CardIDs[userID]; ok {
			l.BingoWinnerCardID = &cid
		}
		joinedUsers := len(l.Cards)
		l.mu.Unlock()

		// payout calculation
		totalPot := float64(l.Stake * joinedUsers)
		winnings := totalPot * 0.8

		// ✅ async DB update + notify + broadcast
		go l.handleBingoWinner(userID, winnings)

		// delay round ending slightly
		go func() {
			time.Sleep(5 * time.Second)
			l.endRound()
		}()

		return true
	}

	// ❌ If not Bingo, user is locked automatically by CheckedUsers map
	log.Printf("[Lobby %d] User %d checked Bingo and failed", l.Stake, userID)
	return false
}

// -----------------
// Extracted helper
// -----------------
func hasBingo(grid [][]int, drawnSet map[int]bool) bool {
	// Full card
	full := true
	for _, row := range grid {
		for _, n := range row {
			if !drawnSet[n] {
				full = false
				break
			}
		}
		if !full {
			break
		}
	}
	if full {
		return true
	}

	// Horizontal
	for _, row := range grid {
		complete := true
		for _, n := range row {
			if !drawnSet[n] {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}

	// Vertical
	for col := 0; col < 5; col++ {
		complete := true
		for row := 0; row < 5; row++ {
			if !drawnSet[grid[row][col]] {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}

	// Corners
	corners := []int{grid[0][0], grid[0][4], grid[4][0], grid[4][4]}
	for _, n := range corners {
		if !drawnSet[n] {
			return false
		}
	}
	return true
}

// -----------------
// Async handler
// -----------------
func (l *Lobby) handleBingoWinner(userID uint, winnings float64) {
	// Update balance
	var winner models.User
	if err := config.DB.First(&winner, userID).Error; err == nil {
		winner.Balance += winnings
		if err := config.DB.Save(&winner).Error; err != nil {
			log.Printf("[Lobby %d] failed to update balance for user %d: %v", l.Stake, userID, err)
		} else {
			l.notifyUser(userID, fmt.Sprintf("🎉 You won BINGO! Winnings: %.2f", winnings))
		}
	} else {
		log.Printf("[Lobby %d] failed to fetch winner user %d: %v", l.Stake, userID, err)
	}

	// Broadcast state (async, doesn’t block CheckBingo)
	l.broadcastState()
}

func (l *Lobby) notifyUser(userID uint, message string) {
	l.mu.RLock()
	client, ok := l.clients[userID]
	l.mu.RUnlock()

	if !ok {
		log.Printf("[Lobby %d] Cannot notify user %d: client not found", l.Stake, userID)
		return
	}

	payload := map[string]string{
		"type":    "notification",
		"message": message,
	}

	b, _ := json.Marshal(payload)

	select {
	case client.send <- b:
	default:
		log.Printf("[Lobby %d] dropping notification to user %d", l.Stake, userID)
	}
}

// -------------------- Auto Rounds --------------------
func (l *Lobby) RunAutoRounds() {
	for {
		// Skip if round already in progress
		l.mu.RLock()
		inProgress := l.Status == "in_progress"
		l.mu.RUnlock()
		if inProgress {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Countdown
		l.mu.Lock()
		l.Status = "countdown"
		l.Countdown = DefaultCountdownSec
		l.mu.Unlock()
		l.broadcastState()

		for i := DefaultCountdownSec; i > 0; i-- {
			l.mu.Lock()
			l.Countdown = i
			l.mu.Unlock()
			l.broadcastState()
			time.Sleep(1 * time.Second)
		}

		// ✅ Require at least 2 selected cards
		l.mu.RLock()
		cardCount := len(l.CardIDs)
		l.mu.RUnlock()

		if cardCount < 2 {

			l.mu.Lock()
			l.Status = "waiting"
			l.Countdown = DefaultCountdownSec
			l.mu.Unlock()
			l.broadcastState()
			continue // skip starting the round
		}

		// Start round safely
		l.startRound()

		// Wait for round to finish
		<-l.roundDone
	}
}

func (l *Lobby) startRound() {
	// 1️⃣ Set round status
	l.mu.Lock()
	l.Status = "in_progress"
	l.NumbersDrawn = []string{}
	l.CheckedUsers = make(map[uint]bool) // ✅ reset checked users
	l.mu.Unlock()
	l.broadcastState()

	// 1.5️⃣ Deduct stake from all users who selected a card
	l.mu.RLock()
	selectedUsers := make(map[uint]int, len(l.CardIDs)) // userID -> cardID
	for userID, cardID := range l.CardIDs {
		selectedUsers[userID] = cardID
	}
	l.mu.RUnlock()

	for userID, cardID := range selectedUsers {
		var user models.User
		if err := config.DB.First(&user, userID).Error; err != nil {
			log.Printf("[Lobby %d] failed to fetch user %d for stake deduction: %v", l.Stake, userID, err)
			continue
		}

		if user.Balance >= float64(l.Stake) {
			user.Balance -= float64(l.Stake)
			if err := config.DB.Save(&user).Error; err != nil {
				log.Printf("[Lobby %d] failed to deduct stake from user %d: %v", l.Stake, userID, err)
				continue
			}

			// Notify the user
			//l.notifyUser(userID, fmt.Sprintf("Your stake of %d has been deducted for this round.", l.Stake))
		} else {
			log.Printf("[Lobby %d] user %d has insufficient balance during startRound", l.Stake, userID)
			l.notifyUser(userID, "Insufficient balance for this round. Your card has been removed.")

			// Remove card safely
			l.mu.Lock()
			delete(l.Cards, userID)
			delete(l.CardIDs, userID)
			delete(l.selectedIDs, cardID)
			l.mu.Unlock()
		}
	}

	// 2️⃣ Create a new game
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
		log.Printf("[Lobby %d] failed to create game: %v", l.Stake, err)
	} else {
		l.mu.Lock()
		l.currentGame = &game
		l.mu.Unlock()
	}

	// 3️⃣ Draw numbers in a goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[Lobby %d] startRound panic: %v", l.Stake, r)
			}
			l.endRound()
		}()

		numbers := generateBingoNumbers()

		for _, n := range numbers {
			time.Sleep(5 * time.Second) // 🔹 delay 1s per number

			l.mu.Lock()
			l.NumbersDrawn = append(l.NumbersDrawn, strconv.Itoa(n))

			if l.currentGame != nil {
				if jsonBytes, err := json.Marshal(l.NumbersDrawn); err == nil {
					l.currentGame.NumbersJSON = datatypes.JSON(jsonBytes)
					_ = config.DB.Save(l.currentGame).Error
				}
			}
			l.mu.Unlock()

			// Broadcast after unlocking to avoid deadlock
			l.broadcastState()
		}
	}()

}

func (l *Lobby) endRound() {
	l.mu.Lock()
	if l.currentGame != nil {
		l.currentGame.Status = "finished"
		l.currentGame.EndTime = time.Now()
		_ = config.DB.Save(l.currentGame).Error
	}
	log.Printf("ending")
	// Reset state
	l.Cards = make(map[uint][]int)
	l.CardIDs = make(map[uint]int)
	l.selectedIDs = make(map[int]bool)
	l.Status = "waiting"
	l.Countdown = DefaultCountdownSec
	l.NumbersDrawn = []string{}
	l.currentGame = nil
	l.BingoWinner = nil
	l.BingoWinnerCardID = nil

	l.mu.Unlock() // unlock before broadcast and channel send

	l.broadcastState()

	// Signal auto-round loop
	l.roundDone <- struct{}{} // **blocking send** guarantees next round starts
}

// -------------------- Broadcast --------------------
type broadcastState struct {
	Stake             int             `json:"stake"`
	Status            string          `json:"status"`
	Countdown         int             `json:"countdown"`
	NumbersDrawn      []string        `json:"numbersDrawn"`
	Cards             map[uint][]int  `json:"cards"`
	Selected          map[uint]int    `json:"selected"`
	AvailableCards    []CardBroadcast `json:"availableCards"` // send full cards
	BingoWinner       *uint
	BingoWinnerCardID *int             `json:"bingoWinnerCardId"`
	Balances          map[uint]float64 `json:"balances"`
	PotentialWinnings float64          `json:"potentialWinnings,omitempty"`
}
type CardBroadcast struct {
	CardID int   `json:"card_id"`
	B      []int `json:"B"`
	I      []int `json:"I"`
	N      []int `json:"N"`
	G      []int `json:"G"`
	O      []int `json:"O"`
	Taken  bool  `json:"taken"`
}

func (l *Lobby) broadcastState() {
	l.mu.RLock()
	balances := make(map[uint]float64, len(l.clients))
	for userID := range l.clients {
		var user models.User
		if err := config.DB.First(&user, userID).Error; err == nil {
			telegramID := uint(user.TelegramID) // convert int64 → uint
			balances[telegramID] = user.Balance
		} else {
			log.Printf("[Lobby %d] failed to fetch balance for user %d: %v", l.Stake, userID, err)
		}
	}
	// ✅ Calculate potential winnings dynamically based on current selected users
	joinedUsers := len(l.Cards)
	potentialWinnings := float64(l.Stake*joinedUsers) * 0.8

	state := broadcastState{
		Stake:             l.Stake,
		Status:            l.Status,
		Countdown:         l.Countdown,
		NumbersDrawn:      append([]string(nil), l.NumbersDrawn...),
		Cards:             copyCardsMap(l.Cards),
		Selected:          copySelectedMap(l.CardIDs),
		AvailableCards:    copyCardsMapWithTaken(l.selectedIDs), // all cards
		BingoWinner:       l.BingoWinner,
		BingoWinnerCardID: l.BingoWinnerCardID, // automatically included
		Balances:          balances,            // ✅ include balances
		PotentialWinnings: potentialWinnings,
	}
	clients := make([]*Client, 0, len(l.clients))
	for _, c := range l.clients {
		clients = append(clients, c)
	}
	l.mu.RUnlock()

	b, _ := json.Marshal(state)
	for _, c := range clients {
		func(c *Client) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Lobby %d] recovered broadcast to user %d: %v", l.Stake, c.userID, r)
				}
			}()
			select {
			case c.send <- b:
			default:
				log.Printf("[Lobby %d] dropping msg to user %d", l.Stake, c.userID)
			}
		}(c)
	}

}
func copyCardsMapWithTaken(selectedIDs map[int]bool) []CardBroadcast {
	cardsMu.RLock()
	defer cardsMu.RUnlock()

	out := make([]CardBroadcast, len(Cards))
	for i, c := range Cards {
		out[i] = CardBroadcast{
			CardID: c.CardID,
			B:      append([]int(nil), c.B...),
			I:      append([]int(nil), c.I...),
			N:      append([]int(nil), c.N...),
			G:      append([]int(nil), c.G...),
			O:      append([]int(nil), c.O...),
			Taken:  selectedIDs[c.CardID],
		}
	}
	return out
}

func copyCardsMap(in map[uint][]int) map[uint][]int {
	out := make(map[uint][]int, len(in))
	for k, v := range in {
		out[k] = append([]int(nil), v...)
	}
	return out
}

func copySelectedMap(in map[uint]int) map[uint]int {
	out := make(map[uint]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// -------------------- Helpers --------------------
func generateBingoNumbers() []int {
	nums := make([]int, 75)
	for i := range nums {
		nums[i] = i + 1
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	r.Shuffle(len(nums), func(i, j int) { nums[i], nums[j] = nums[j], nums[i] })
	return nums
}
