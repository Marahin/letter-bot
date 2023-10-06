package book

import (
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

type UnbookAutocompleteFocus int

const (
	UnbookAutocompleteReservation UnbookAutocompleteFocus = iota
)

type UnbookAutocompleteRequest struct {
	Member *discord.Member
	Guild  *discord.Guild
	Field  UnbookAutocompleteFocus
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
