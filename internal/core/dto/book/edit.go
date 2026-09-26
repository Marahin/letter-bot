package book

import "time"

// EditRequest changes the respawn, times and author of one guild reservation.
type EditRequest struct {
	GuildID         string
	ReservationID   int64
	SpotID          int64
	StartAt         time.Time
	EndAt           time.Time
	Author          string
	AuthorDiscordID string
}
