package models

type Lobby struct {
	GameID    uint
	Players   map[int64]bool // TelegramID -> joined
	Status    string         // waiting, countdown, playing
	Broadcast chan interface{}
}
