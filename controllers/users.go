package controllers

import (
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

	// check if already exists
	var existing models.User
	if err := config.DB.Where("telegram_id = ?", user.TelegramID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
		return
	}

	if err := config.DB.Create(&user).Error; err != nil {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid telegram_id"})
		return
	}

	var user models.User
	if err := config.DB.Where("telegram_id = ?", tid).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
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

	// check if user exists
	var user models.User
	if err := config.DB.Where("telegram_id = ?", tid).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// update phone
	if err := config.DB.Model(&user).Update("phone", req.Phone).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update phone"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"telegram_id": tid,
		"phone":       req.Phone,
	})
}
