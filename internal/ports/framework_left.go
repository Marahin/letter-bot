package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
	"spot-assistant/internal/core/summarytracker"
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
	BaseSummary() *summary.Summary
}

type SummaryTrackerService interface {
	GetTrackedMessages(ctx context.Context, guildID, channelID string) ([]*summarytracker.TrackedMessage, error)
	TrackMessage(ctx context.Context, guildID, channelID, messageID string, messageType summarytracker.MessageType, messageOrder int) error
	DeleteMessage(ctx context.Context, id int64) error
	DeleteAllForChannel(ctx context.Context, guildID, channelID string) error
	UpdateTimestamp(ctx context.Context, id int64) error
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

type OnlineCheckService interface {
	IsOnline(guildID, characterName string) bool
	PlayerStatus(guildID, characterName string) summary.OnlineStatus
	RefreshOnlinePlayers(guildID string) error
	IsConfigured() bool
	TryRefresh(guildID string)
	ConfigureWorldName(guildID, world string)
	SetGuildWorld(guildID, world string) error
	ConfigureWorldNameForGuild(guildID string) error
}
