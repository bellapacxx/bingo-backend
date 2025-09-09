package models

import "time"

type Card struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	GameID    uint      `json:"game_id"`
	UserID    uint      `json:"user_id"`
	Numbers   []int     `gorm:"type:json" json:"numbers"` // store as JSON array
	Marked    []int     `gorm:"type:json" json:"marked"`  // marked numbers
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
