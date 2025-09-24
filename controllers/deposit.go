package controllers

import (
	"net/http"

	"github.com/bellapacxx/bingo-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type VerifyDepositRequest struct {
	UserID         int    `json:"userId" binding:"required"`         // Telegram ID
	ExpectedAmount int    `json:"expectedAmount" binding:"required"` // Amount expected
	Reference      string `json:"reference" binding:"required"`      // Reference code
}

func VerifyDeposit(c *gin.Context) {
	var req VerifyDepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	db, _ := c.MustGet("db").(*gorm.DB) // assuming *gorm.DB is stored in context
	var user models.User
	if err := db.Where("telegram_id = ?", req.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	// Optional: prevent double deposits by checking reference
	var depositExists int64
	db.Model(&models.Deposit{}).Where("reference = ?", req.Reference).Count(&depositExists)
	if depositExists > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "deposit with this reference already processed"})
		return
	}

	// Update balance
	user.Balance += float64(req.ExpectedAmount)
	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update balance"})
		return
	}

	// Record the deposit
	db.Create(&models.Deposit{
		UserID:    user.ID,
		Amount:    float64(req.ExpectedAmount),
		Reference: req.Reference,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Balance updated successfully",
	})
}
