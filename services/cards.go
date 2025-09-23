package services

import (
	"encoding/json"
	"log"
	"os"
	"sync"
)

// BingoCard definition
type BingoCard struct {
	B      []int `json:"B"`
	I      []int `json:"I"`
	N      []int `json:"N"`
	G      []int `json:"G"`
	O      []int `json:"O"`
	CardID int   `json:"card_id"`
}

var (
	Cards   []BingoCard
	cardsMu sync.RWMutex
)

// LoadCards loads bingo cards from JSON file
func LoadCards() {
	data, err := os.ReadFile("cards.json")
	if err != nil {
		log.Fatalf("Failed to read cards.json: %v", err)
	}
	if err := json.Unmarshal(data, &Cards); err != nil {
		log.Fatalf("Failed to unmarshal cards.json: %v", err)
	}
	log.Printf("[Init] Loaded %d bingo cards", len(Cards))
}
