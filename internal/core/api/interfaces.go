package api

import (
	"spot-assistant/internal/core/dto/book"
	"time"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

type summaryService interface {
	PrepareSummary(reservations []*reservation.ReservationWithSpot) (*summary.Summary, error)
}

type communicationService interface {
	NotifyOverbookedMember(member *discord.Member,
		request book.BookRequest,
		res *reservation.ClippedOrRemovedReservation)
	SendGuildSummary(guild *discord.Guild, summary *summary.Summary) error
	SendPrivateSummary(request summary.PrivateSummaryRequest, summary *summary.Summary) error
}

type bookingService interface {
	// Returns available spots based on optional filter, or an error.
	FindAvailableSpots(filter string) ([]string, error)

	// Returns suggested hours based on base time and optional filter.
	GetSuggestedHours(time.Time, string) []string

	// Returns array of conflicting reservations (or removed reservations)
	// and an optional error.
	Book(member *discord.Member, guild *discord.Guild, spot string, startAt time.Time, endAt time.Time, overbook bool, hasPermissions bool) ([]*reservation.ClippedOrRemovedReservation, error)

	UnbookAutocomplete(g *discord.Guild, m *discord.Member, filter string) ([]*reservation.ReservationWithSpot, error)

	Unbook(g *discord.Guild, m *discord.Member, reservationId int64) (*reservation.ReservationWithSpot, error)
}
