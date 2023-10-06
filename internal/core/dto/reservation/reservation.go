package reservation

import "time"

type Reservation struct {
	ID              int64
	Author          string
	CreatedAt       time.Time
	StartAt         time.Time
	EndAt           time.Time
	SpotID          int64
	GuildID         string
	AuthorDiscordID string
}

type Spot struct {
	ID   int64
	Name string
}

type ReservationWithSpot struct {
	Reservation
	Spot
}
