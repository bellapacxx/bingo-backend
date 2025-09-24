package controllers

import (
	"log"
	"net/http"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type VerifyDepositRequest struct {
	UserID         int    `json:"userId" binding:"required"`         // Telegram ID
	ExpectedAmount int    `json:"expectedAmount" binding:"required"` // Amount expected
	Reference      string `json:"reference" binding:"required"`      // Reference code
}

// VerifyDeposit updates user balance after deposit verification
func VerifyDeposit(c *gin.Context) {
	log.Printf("masgds")
	var req VerifyDepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := config.DB.Where("telegram_id = ?", req.UserID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		log.Printf("[ERROR] Failed to fetch user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Prevent double deposits
	var depositExists int64
	config.DB.Model(&models.Deposit{}).Where("reference = ?", req.Reference).Count(&depositExists)
	if depositExists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "deposit with this reference already processed"})
		return
	}

	// Update balance
	user.Balance += float64(req.ExpectedAmount)
	if err := config.DB.Save(&user).Error; err != nil {
		log.Printf("[ERROR] Failed to update balance: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update balance"})
		return
	}

	// Record the deposit
	if err := config.DB.Create(&models.Deposit{
		UserID:    user.ID,
		Amount:    float64(req.ExpectedAmount),
		Reference: req.Reference,
	}).Error; err != nil {
		log.Printf("[ERROR] Failed to record deposit: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record deposit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Balance updated successfully",
	})
}
