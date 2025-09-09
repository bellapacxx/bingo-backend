package controllers

import (
	"net/http"
	"strconv"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/gin-gonic/gin"
)

// RegisterUser registers a new user (from Telegram)
func RegisterUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
	tid, _ := strconv.ParseInt(tidStr, 10, 64)

	var user models.User
	if err := config.DB.First(&user, "telegram_id = ?", tid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdatePhone updates a user phone number
func UpdatePhone(c *gin.Context) {
	tidStr := c.Param("telegram_id")
	tid, _ := strconv.ParseInt(tidStr, 10, 64)

	var req struct {
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Model(&models.User{}).Where("telegram_id = ?", tid).Update("phone", req.Phone).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update phone"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"telegram_id": tid, "phone": req.Phone})
}
