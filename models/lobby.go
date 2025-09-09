package models

import (
	"sync"
)

type Lobby struct {
	GameID      uint
	Players     map[int64]*User // key = telegram_id
	MinPlayers  int
	Countdown   int // seconds
	Started     bool
	mu          sync.Mutex
	OnGameStart func(gameID uint, players []*User)
}
