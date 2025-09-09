package game

import (
	"log"

	"sync"
	"time"

	"github.com/bellapacxx/bingo-backend/models"
)

// Lobby represents a waiting room for a bingo game
type Lobby struct {
	GameID      uint
	Players     map[int64]*models.User // key = telegram_id
	MinPlayers  int
	Countdown   int // seconds
	Started     bool
	mu          sync.Mutex
	onGameStart func(gameID uint, players []*models.User)
}

// NewLobby creates a new lobby
func NewLobby(gameID uint, minPlayers, countdown int, onStart func(gameID uint, players []*models.User)) *Lobby {
	return &Lobby{
		GameID:      gameID,
		Players:     make(map[int64]*models.User),
		MinPlayers:  minPlayers,
		Countdown:   countdown,
		onGameStart: onStart,
	}
}

// AddPlayer adds a player to the lobby
func (l *Lobby) AddPlayer(user *models.User) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.Started {
		return
	}

	l.Players[user.TelegramID] = user
	log.Printf("[Lobby] Player %s joined game %d (%d/%d)", user.Phone, l.GameID, len(l.Players), l.MinPlayers)

	if len(l.Players) >= l.MinPlayers && !l.Started {
		l.Started = true
		go l.startCountdown()
	}
}

// startCountdown triggers the countdown and calls onGameStart
func (l *Lobby) startCountdown() {
	log.Printf("[Lobby] Minimum players reached. Starting game %d in %d seconds...", l.GameID, l.Countdown)
	time.Sleep(time.Duration(l.Countdown) * time.Second)

	l.mu.Lock()
	defer l.mu.Unlock()

	players := []*models.User{}
	for _, u := range l.Players {
		players = append(players, u)
	}

	log.Printf("[Lobby] Game %d started with %d players", l.GameID, len(players))
	if l.onGameStart != nil {
		l.onGameStart(l.GameID, players)
	}
}
