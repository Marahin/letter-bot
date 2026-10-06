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

// ReservationFormService backs the bot's buttons and booking wizard. It applies the same
// rules as the web (ReservationService) to the member who clicked.
type ReservationFormService interface {
	// RespawnPicker returns step 1 of the booking wizard: the actor's usual
	// respawns and one page (0-based, clamped) of the active respawns.
	RespawnPicker(ctx context.Context, guildID string, actor reservation.Actor, page int) (*reservation.RespawnPicker, error)
	// FindRespawn returns the active respawn with the name, or the only one that
	// contains it. It returns a *reservationforms.AmbiguousSpotError when more
	// match, and booking.ErrSpotNotFound when none does.
	FindRespawn(ctx context.Context, guildID, name string) (*spot.Spot, error)
	// TimePicker returns step 2 of the wizard. With a reservation id in the
	// choice it checks the edit right (errors as Editable) and fills the unchosen
	// fields from the reservation. It returns booking.ErrSpotNotFound,
	// booking.ErrSpotArchived and booking.ErrSpotLocked.
	TimePicker(ctx context.Context, guildID string, actor reservation.Actor, choice reservation.TimeChoice) (*reservation.TimePicker, error)
	// BookChoice books the choice. It returns reservationforms.ErrChoiceIncomplete,
	// booking.ErrStartInPast for an old slot, and the errors of Book.
	BookChoice(ctx context.Context, guildID string, actor reservation.Actor, choice reservation.TimeChoice) (*reservation.FormOutcome, error)
	// Book books a draft whose respawn and times are known. It returns
	// booking.ErrInsufficientPermissions with the conflicts in the outcome.
	Book(ctx context.Context, guildID string, actor reservation.Actor, draft reservation.Draft) (*reservation.FormOutcome, error)
	// EditChoice changes the reservation of the choice, without overbooking.
	// It returns booking.ErrConflict with the conflicts in the outcome.
	EditChoice(ctx context.Context, guildID string, actor reservation.Actor, choice reservation.TimeChoice) (*reservation.FormOutcome, error)
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
}
