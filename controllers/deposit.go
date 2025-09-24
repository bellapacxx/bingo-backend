package controllers

import (
	"net/http"

	"github.com/bellapacxx/bingo-backend/models"
	"github.com/bellapacxx/bingo-backend/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type VerifyDepositRequest struct {
	UserID         int    `json:"userId" binding:"required"`         // Telegram ID
	SMS            string `json:"sms" binding:"required"`            // Copied SMS text
	ExpectedAmount int    `json:"expectedAmount" binding:"required"` // Amount expected
	Reference      string `json:"reference" binding:"required"`      // Reference code
}

func VerifyDeposit(c *gin.Context) {
	var req VerifyDepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Call the service to verify SMS
	verified, err := services.VerifyDeposit(req.SMS)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// If verified, update user balance
	if verified {
		db, _ := c.MustGet("db").(*gorm.DB) // assuming *gorm.DB is stored in context
		var user models.User
		if err := db.Where("telegram_id = ?", req.UserID).First(&user).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		user.Balance += float64(req.ExpectedAmount)
		if err := db.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update balance"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": verified,
	})
}
