package ports

import (
	"context"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
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
	SendPrivateSummary(request summary.PrivateSummaryRequest, sum *summary.Summary) error
}

type SummaryService interface {
	PrepareSummary(reservations []*reservation.ReservationWithSpot) (*summary.Summary, error)
}

type BookingService interface {
	// Returns available guild spots based on optional filter, or an error.
	FindAvailableSpots(guildID, filter string) ([]string, error)

	// Returns suggested hours based on base time and optional filter.
	GetSuggestedHours(time.Time, string) []string

	// Returns array of conflicting reservations (or removed reservations)
	// and an optional error.
	Book(request book.BookRequest) ([]*reservation.ClippedOrRemovedReservation, error)

	UnbookAutocomplete(g *guild.Guild, m *member.Member, filter string) ([]*reservation.ReservationWithSpot, error)

	Unbook(g *guild.Guild, m *member.Member, reservationID int64) (*reservation.ReservationWithSpot, error)

	// Edit changes a reservation under the booking rules, without overbooking.
	// On booking.ErrConflict it returns the overlapping reservations.
	Edit(ctx context.Context, req book.EditRequest) ([]*reservation.Reservation, error)

	// DeleteForGuild deletes any reservation of the guild. Returns ports.ErrNotFound.
	DeleteForGuild(ctx context.Context, guildID string, id int64) error
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

// NotifyHandler reacts to the signals the web sends to the bot through Postgres NOTIFY.
// Every signal is best-effort: the bot tick recovers a missed one.
type NotifyHandler interface {
	OnSummaryRefresh(ctx context.Context, guildID string)
	OnGuildResync(ctx context.Context, guildID string)
	OnGuildConfig(ctx context.Context, guildID string)
	OnOverbooked(ctx context.Context, request book.BookRequest, res *reservation.ClippedOrRemovedReservation)
}

// ReservationFormService backs the bot's buttons and forms. It applies the same
// rules as the web (ReservationService) to the member who clicked.
type ReservationFormService interface {
	// BookForm reads the form and books. It returns a *reservationforms.AmbiguousSpotError
	// when more than one respawn matches, reservationforms.ErrTimeFormat, and
	// booking.ErrInsufficientPermissions with the conflicts in the outcome.
	BookForm(ctx context.Context, guildID string, actor reservation.Actor, form reservation.Form) (*reservation.FormOutcome, error)
	// Book books a draft whose respawn and times are known. Errors as BookForm.
	Book(ctx context.Context, guildID string, actor reservation.Actor, draft reservation.Draft) (*reservation.FormOutcome, error)
	// EditForm reads the form and changes the reservation, without overbooking.
	// It returns booking.ErrConflict with the conflicts in the outcome.
	EditForm(ctx context.Context, guildID string, actor reservation.Actor, id int64, form reservation.Form) (*reservation.FormOutcome, error)
	// Edit changes the reservation to a draft whose respawn and times are known.
	Edit(ctx context.Context, guildID string, actor reservation.Actor, id int64, draft reservation.Draft) (*reservation.FormOutcome, error)
	// Editable returns the reservation when the actor may edit it, else
	// reservations.ErrForbidden, booking.ErrReservationEnded or ErrNotFound.
	Editable(ctx context.Context, guildID string, actor reservation.Actor, id int64) (*reservation.ReservationWithSpot, error)
	// Cancellable returns the reservation when the actor may cancel it, else
	// reservations.ErrForbidden or ErrNotFound.
	Cancellable(ctx context.Context, guildID string, actor reservation.Actor, id int64) (*reservation.ReservationWithSpot, error)
	// Cancel deletes the reservation and returns it. Errors as Cancellable.
	Cancel(ctx context.Context, guildID string, actor reservation.Actor, id int64) (*reservation.ReservationWithSpot, error)
	// Mine returns the actor's upcoming reservations, the ongoing one first.
	Mine(ctx context.Context, guildID string, actor reservation.Actor, limit int) (*reservation.Page, error)
	// Spot returns an active or archived respawn of the guild, or ErrNotFound.
	Spot(ctx context.Context, guildID string, id int64) (*spot.Spot, error)
}
