package book

import (
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

type UnbookAutocompleteRequest struct {
	Member *discord.Member
	Guild  *discord.Guild
	Value  string
}

type UnbookAutocompleteChoice struct {
	Label string
	Value int64
}

type UnbookAutocompleteResponse struct {
	Choices []*reservation.ReservationWithSpot
}

type UnbookRequest struct {
	Member        *discord.Member
	Guild         *discord.Guild
	ReservationID int64
}
