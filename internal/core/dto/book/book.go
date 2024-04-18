package book

import (
	"time"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

type BookAutocompleteFocus int

const (
	BookAutocompleteSpot BookAutocompleteFocus = iota
	BookAutocompleteStartAt
	BookAutocompleteEndAt
	BookAutocompleteOverbook
)

// Request for autocompletion during Booking process
type BookAutocompleteRequest struct {
	Field BookAutocompleteFocus
	Value string
}

// Response for autocompletion during booking process
type BookAutocompleteResponse []string

// Booking request
type BookRequest struct {
	*discord.Guild
	*discord.Channel
	*discord.Message
	*discord.Member

	Spot     string
	StartAt  time.Time
	EndAt    time.Time
	Overbook bool
}

type BookResponse struct {
	Spot    string
	StartAt time.Time
	EndAt   time.Time

	ConflictingReservations []*reservation.Reservation
}
