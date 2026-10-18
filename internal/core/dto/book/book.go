package book

import (
	"time"

	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"

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
	GuildID string
	Field   BookAutocompleteFocus
	Value   string
}

// Response for autocompletion during booking process
type BookAutocompleteResponse []string

// Booking request
type BookRequest struct {
	*guild.Guild
	*member.Member

	Spot string
	// SpotID, when set, picks the spot by id instead of by the Spot name.
	SpotID int64

	StartAt        time.Time
	EndAt          time.Time
	Overbook       bool
	HasPermissions bool
}

type BookResponse struct {
	Request *BookRequest

	ConflictingReservations []*reservation.ClippedOrRemovedReservation
}
