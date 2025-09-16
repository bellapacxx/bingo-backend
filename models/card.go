package models

import "time"

type Card struct {
	ID        uint  `gorm:"primaryKey"`
	UserID    uint  `gorm:"not null"`
	GameID    uint  `gorm:"not null"`
	Numbers   []int `gorm:"type:jsonb" json:"numbers"` // store as JSONB array
	CreatedAt time.Time
	UpdatedAt time.Time
}
