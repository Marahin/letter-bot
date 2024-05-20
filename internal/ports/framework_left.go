package ports

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	"time"
)

type APIPort interface {
	OnReady()
	OnGuildCreate(*guild.Guild)
	OnTick()
	OnBook(book.BookRequest) (book.BookResponse, error)
	OnBookAutocomplete(book.BookAutocompleteRequest) (book.BookAutocompleteResponse, error)
	OnUnbook(request book.UnbookRequest) (*reservation.ReservationWithSpot, error)
	OnUnbookAutocomplete(request book.UnbookAutocompleteRequest) (book.UnbookAutocompleteResponse, error)
	OnPrivateSummary(summary.PrivateSummaryRequest) error
}

type CommunicationService interface {
	NotifyOverbookedMember(
		request book.BookRequest,
		res *reservation.ClippedOrRemovedReservation)
	SendGuildSummary(guild *guild.Guild, summary *summary.Summary) error
	SendPrivateSummary(request summary.PrivateSummaryRequest, summary *summary.Summary) error
}

type SummaryService interface {
	PrepareSummary(reservations []*reservation.ReservationWithSpot) (*summary.Summary, error)
}

type BookingService interface {
	// Returns available spots based on optional filter, or an error.
	FindAvailableSpots(filter string) ([]string, error)

	// Returns suggested hours based on base time and optional filter.
	GetSuggestedHours(time.Time, string) []string

	// Returns array of conflicting reservations (or removed reservations)
	// and an optional error.
	Book(request book.BookRequest) ([]*reservation.ClippedOrRemovedReservation, error)

	UnbookAutocomplete(g *guild.Guild, m *member.Member, filter string) ([]*reservation.ReservationWithSpot, error)

	Unbook(g *guild.Guild, m *member.Member, reservationId int64) (*reservation.ReservationWithSpot, error)
}
