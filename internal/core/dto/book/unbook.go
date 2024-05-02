package book

import (
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
)

type UnbookAutocompleteRequest struct {
	Member *member.Member
	Guild  *guild.Guild
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
	Member        *member.Member
	Guild         *guild.Guild
	ReservationID int64
}
