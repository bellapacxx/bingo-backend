package controllers

import (
	"net/http"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"

	"github.com/gin-gonic/gin"
)

// Deposit handles adding funds to user wallet
func Deposit(c *gin.Context) {
	var tx models.Transaction
	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tx.Type = "deposit"
	if err := config.DB.Create(&tx).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deposit"})
		return
	}

	c.JSON(http.StatusCreated, tx)
}

// Withdraw handles user withdrawal
func Withdraw(c *gin.Context) {
	// Bind request JSON
	var req struct {
		TelegramID int64   `json:"telegramId"`
		Amount     float64 `json:"amount"`
		Method     string  `json:"method"`  // optional, for tracking method
		Account    string  `json:"account"` // optional, for tracking account
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find the user
	var user models.User
	if err := config.DB.Where("telegram_id = ?", req.TelegramID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if user has enough balance
	if user.Balance < req.Amount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient balance"})
		return
	}

	// Start DB transaction for safety
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Subtract balance
	user.Balance -= req.Amount
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update balance"})
		return
	}

	// Create the transaction record
	withdrawTx := models.Transaction{
		UserID: uint(req.TelegramID),
		Amount: req.Amount,
		Type:   "withdraw",
	}
	if err := tx.Create(&withdrawTx).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}

	// Commit transaction
	tx.Commit()
	c.JSON(http.StatusCreated, withdrawTx)
}
