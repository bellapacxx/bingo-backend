package models

import "time"

type User struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TelegramID int64     `gorm:"uniqueIndex" json:"telegram_id"`
	Name       string    `json:"name"`
	Phone      string    `json:"phone"`
	Balance    float64   `json:"balance"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
