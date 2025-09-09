package models

import "time"

type TransactionType string

const (
	DepositTransaction  TransactionType = "deposit"
	WithdrawTransaction TransactionType = "withdraw"
)

type Transaction struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	UserID       uint            `json:"user_id"`
	Type         TransactionType `json:"type"`
	Amount       float64         `json:"amount"`
	BalanceAfter float64         `json:"balance_after"`
	CreatedAt    time.Time       `json:"created_at"`
}
