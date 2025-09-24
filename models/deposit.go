package models

import (
	"time"

	"gorm.io/gorm"
)

type Deposit struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `gorm:"not null" json:"userId"`
	Amount    float64        `gorm:"not null" json:"amount"`
	Reference string         `gorm:"uniqueIndex;not null" json:"reference"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
