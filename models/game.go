package models

import "time"

type GameStatus string

const (
	GameWaiting   GameStatus = "waiting"
	GameCountdown GameStatus = "countdown"
	GamePlaying   GameStatus = "playing"
	GameFinished  GameStatus = "finished"
)

type Game struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Status     GameStatus `json:"status"`
	MinPlayers int        `json:"min_players"`
	Countdown  int        `json:"countdown"` // seconds
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
