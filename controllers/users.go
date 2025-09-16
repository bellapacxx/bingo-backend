package controllers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterUser registers a new user (from Telegram)
func RegisterUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Ensure telegram_id is valid
	if user.TelegramID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "telegram_id is required"})
		return
	}

	// Check if already exists
	var existing models.User
	err := config.DB.Where("telegram_id = ?", user.TelegramID).First(&existing).Error
	if err == nil {
		// User exists, return existing data
		c.JSON(http.StatusOK, existing)
		return
	} else if err != gorm.ErrRecordNotFound {
		// Some other DB error
		log.Printf("[ERROR] Failed to check existing user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Create new user
	if err := config.DB.Create(&user).Error; err != nil {
		log.Printf("[ERROR] Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// GetUser fetches a user by telegram_id
func GetUser(c *gin.Context) {
	tidStr := c.Param("telegram_id")
	tid, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid telegram_id, must be a number"})
		return
	}

	var user models.User
	if err := config.DB.Where("telegram_id = ?", tid).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		log.Printf("[ERROR] Failed to fetch user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdatePhone updates a user's phone number
func UpdatePhone(c *gin.Context) {
	tidStr := c.Param("telegram_id")
	tid, err := strconv.ParseInt(tidStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid telegram_id"})
		return
	}

	var req struct {
		Phone string `json:"phone" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Find user
	var user models.User
	if err := config.DB.Where("telegram_id = ?", tid).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		log.Printf("[ERROR] Failed to fetch user for phone update: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Update phone
	if err := config.DB.Model(&user).Update("phone", req.Phone).Error; err != nil {
		log.Printf("[ERROR] Failed to update phone for telegram_id %d: %v", tid, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update phone"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"telegram_id": tid,
		"phone":       req.Phone,
	})
}
