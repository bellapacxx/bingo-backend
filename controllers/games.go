package controllers

import (
	"net/http"
	"strconv"

	"github.com/bellapacxx/bingo-backend/config"
	"github.com/bellapacxx/bingo-backend/game"
	"github.com/bellapacxx/bingo-backend/models"
	"github.com/bellapacxx/bingo-backend/utils/logger"
	"github.com/gin-gonic/gin"
)

// In-memory lobbies map (gameID -> Lobby)
var Lobbies = make(map[uint]*game.Lobby)

// ListGames returns all active games
func ListGames(c *gin.Context) {
	var games []models.Game
	config.DB.Find(&games)
	c.JSON(http.StatusOK, games)
}

// GetGame returns single game info
func GetGame(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	var game models.Game
	if err := config.DB.First(&game, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Game not found"})
		return
	}

	c.JSON(http.StatusOK, game)
}

// JoinGame adds a player to a lobby
func JoinGame(c *gin.Context) {
	gameIDStr := c.Param("id")
	gameID, _ := strconv.Atoi(gameIDStr)

	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create lobby if not exists
	lobby, ok := Lobbies[uint(gameID)]
	if !ok {
		lobby = game.NewLobby(uint(gameID), 2, 10, func(gid uint, players []*models.User) {
			// TODO: Start game logic here
			logger.Infof("Game %d started with %d players", gid, len(players))
		})
		Lobbies[uint(gameID)] = lobby
	}

	lobby.AddPlayer(&user)
	c.JSON(http.StatusOK, gin.H{"message": "Joined lobby", "players": len(lobby.Players)})
}

// LobbyStatus returns current lobby info
func LobbyStatus(c *gin.Context) {
	gameIDStr := c.Param("id")
	gameID, _ := strconv.Atoi(gameIDStr)

	lobby, ok := Lobbies[uint(gameID)]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Lobby not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"game_id":     lobby.GameID,
		"players":     len(lobby.Players),
		"min_players": lobby.MinPlayers,
		"countdown":   lobby.Countdown,
		"started":     lobby.Started,
	})
}
