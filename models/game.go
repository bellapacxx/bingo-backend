package models

import (
	"time"

	"gorm.io/datatypes"
)

type Game struct {
	ID           uint   `gorm:"primaryKey"`
	Stake        int    // 10, 20, 50, 100
	Status       string // waiting | in_progress | finished
	RoundNumber  int
	NumbersDrawn []string `gorm:"type:json"` // store drawn numbers as JSON array
	StartTime    time.Time
	EndTime      time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
	NumbersJSON  datatypes.JSON // stores drawn numbers in DB

}
