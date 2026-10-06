package book

import (
	"time"

	"spot-assistant/internal/core/dto/reservation"
)

// EditRequest changes the respawn, times and author of one guild reservation.
type EditRequest struct {
	GuildID       string
	ReservationID int64
	SpotID        int64
	StartAt       time.Time
	EndAt         time.Time
	// An empty Author keeps the current author and Discord id.
	Author          string
	AuthorDiscordID string
	// Authorize, when set, may refuse the change once the reservation is loaded.
	Authorize func(existing reservation.Reservation) error
}
