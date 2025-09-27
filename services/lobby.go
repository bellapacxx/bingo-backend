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
    drawCancel chan struct{} // <- new field

	mu          sync.RWMutex
	currentGame *models.Game
	// New: store current round winner
	BingoWinner       *uint
	 BingoWinnerName   *string
	BingoWinnerCardID *int // cardID ✅
	CheckedUsers      map[uint]bool
	roundPot float64 // store total pot for the current round
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
			drawCancel:  make(chan struct{}), // ← initialize here
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
	// --- Step 1: Initialize CheckedUsers map and check if user already checked ---
	l.mu.Lock()
	if l.CheckedUsers == nil {
		l.CheckedUsers = make(map[uint]bool)
	}

	if l.CheckedUsers[userID] {
		l.mu.Unlock() // unlock first
		l.notifyUser(userID, "⚠️ You cannot check again this round.ይሄ ካርቴላ ታስረዋል.")
		log.Printf("[Lobby %d] User %d already checked Bingo this round", l.Stake, userID)
		return false
	}

	// Mark as checked
	l.CheckedUsers[userID] = true

	// Copy numbers and drawn numbers safely
	numbers, ok := l.Cards[userID]
	drawnNums := append([]string(nil), l.NumbersDrawn...)
	l.mu.Unlock() // unlock ASAP

	// --- Step 2: Validate user has a card ---
	if !ok {
		log.Printf("[Lobby %d] User %d tried Bingo without a card", l.Stake, userID)
		return false
	}

	log.Printf("[Lobby %d] checking bingo for user %d", l.Stake, userID)

	// --- Step 3: Build drawn set ---
	drawnSet := make(map[int]bool, len(drawnNums))
	for _, n := range drawnNums {
		if num, err := strconv.Atoi(n); err == nil {
			drawnSet[num] = true
		}
	}

	// --- Step 4: Build 5x5 grid ---
	grid := make([][]int, 5)
	for i := 0; i < 5; i++ {
		grid[i] = numbers[i*5 : (i+1)*5]
	}

	// --- Step 5: Check bingo patterns ---
	if hasBingo(grid, drawnSet) {
		log.Printf("[Lobby %d] User %d claims BINGO!", l.Stake, userID)
        // Stop number drawing immediately
    l.mu.Lock()
    if l.drawCancel != nil {
        close(l.drawCancel)           // signal cancel
        l.drawCancel = make(chan struct{}) // recreate for next round
    }
		// --- Store winner safely ---
		
		l.BingoWinner = &userID
		if cid, ok := l.CardIDs[userID]; ok {
			l.BingoWinnerCardID = &cid
		}
		joinedUsers := len(l.Cards)
		l.mu.Unlock()

		// --- Payout ---
		totalPot := float64(l.Stake * joinedUsers)
		winnings := totalPot * 0.8

		// Async DB update, notification, broadcast
		go l.handleBingoWinner(userID, winnings)

		// End round after slight delay
		go func() {
			time.Sleep(7 * time.Second)
			l.endRound()
		}()

		return true
	}

	// ❌ Bingo failed, user is already marked as checked
	log.Printf("[Lobby %d] User %d checked Bingo and failed", l.Stake, userID)
	return false
}

// -----------------
// Extracted helper
// -----------------
func hasBingo(grid [][]int, drawnSet map[int]bool) bool {
	const freeRow, freeCol = 2, 2 // center free space

	isDrawn := func(n int, row, col int) bool {
		if row == freeRow && col == freeCol {
			return true
		}
		return drawnSet[n]
	}

	checkLine := func(cells [][2]int) bool {
		for _, cell := range cells {
			row, col := cell[0], cell[1]
			if !isDrawn(grid[row][col], row, col) {
				return false
			}
		}
		return true
	}

	// 1️⃣ Corners
	corners := [][2]int{{0, 0}, {0, 4}, {4, 0}, {4, 4}}
	if checkLine(corners) {
		return true
	}

	// 2️⃣ Horizontal lines
	for row := 0; row < 5; row++ {
		cells := [][2]int{}
		for col := 0; col < 5; col++ {
			cells = append(cells, [2]int{row, col})
		}
		if checkLine(cells) {
			return true
		}
	}

	// 3️⃣ Vertical lines
	for col := 0; col < 5; col++ {
		cells := [][2]int{}
		for row := 0; row < 5; row++ {
			cells = append(cells, [2]int{row, col})
		}
		if checkLine(cells) {
			return true
		}
	}

	// 4️⃣ Cross (middle row + middle column)
	cross := [][2]int{}
	for i := 0; i < 5; i++ {
		cross = append(cross, [2]int{2, i}) // middle row
		cross = append(cross, [2]int{i, 2}) // middle column
	}
	if checkLine(cross) {
		return true
	}

	// 5️⃣ Diagonals
	diag1 := [][2]int{}
	diag2 := [][2]int{}
	for i := 0; i < 5; i++ {
		diag1 = append(diag1, [2]int{i, i})
		diag2 = append(diag2, [2]int{i, 4 - i})
	}
	if checkLine(diag1) || checkLine(diag2) {
		return true
	}

	// 6️⃣ Full card
	fullCard := [][2]int{}
	for r := 0; r < 5; r++ {
		for c := 0; c < 5; c++ {
			fullCard = append(fullCard, [2]int{r, c})
		}
	}
	if checkLine(fullCard) {
		return true
	}

	return false
}

// -----------------
// Async handler
// -----------------
func (l *Lobby) handleBingoWinner(userID uint, winnings float64) {
	log.Printf("db save")
	// Update balance
	var winner models.User
	if err := config.DB.First(&winner, userID).Error; err == nil {
		winner.Balance += winnings
		if err := config.DB.Save(&winner).Error; err != nil {
			log.Printf("[Lobby %d] failed to update balance for user %d: %v", l.Stake, userID, err)
		} else {
			l.notifyUser(userID, fmt.Sprintf("🎉 You won BINGO! Winnings: %.2f", winnings))
			// ✅ Save winner name for broadcast
			l.mu.Lock()
			l.BingoWinnerName = &winner.Name
			log.Printf("%s", *l.BingoWinnerName)
			l.mu.Unlock()
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

		if cardCount < 1 {

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
	joinedUsers := len(l.Cards) // number of users at start
    l.roundPot = float64(l.Stake * joinedUsers) * 0.8
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
        // endRound will be called by CheckBingo, so no need here
		
    }()

    numbers := generateBingoNumbers()

    for _, n := range numbers {
        select {
        case <-l.drawCancel:
            log.Printf("[Lobby %d] Number draw canceled", l.Stake)
            return // stop drawing numbers
        case <-time.After(6 * time.Second):
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
    }
	l.endRound()
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
    l.roundPot = 0
    l.BingoWinnerName = nil
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
	BingoWinnerName   *string         `json:"bingoWinnerName"` 
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
	
	potentialWinnings := l.roundPot

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
		BingoWinnerName:   l.BingoWinnerName,  // ✅ now works
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
