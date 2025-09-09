package controllers

import (
	"net/http"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/models"

	"github.com/gin-gonic/gin"
)

// BuyTicket creates a ticket for a user in a game
func BuyTicket(c *gin.Context) {
	var ticket models.Card
	if err := c.ShouldBindJSON(&ticket); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := config.DB.Create(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to buy ticket"})
		return
	}

	c.JSON(http.StatusCreated, ticket)
}

// GetTicketsByUser fetches all tickets of a user
func GetTicketsByUser(c *gin.Context) {
	tidStr := c.Param("telegram_id")
	var tickets []models.Card
	if err := config.DB.Where("telegram_id = ?", tidStr).Find(&tickets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch tickets"})
		return
	}

	c.JSON(http.StatusOK, tickets)
}
