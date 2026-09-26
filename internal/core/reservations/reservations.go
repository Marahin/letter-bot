// Package reservations is the web side of a guild's reservations: search, and
// create, edit and delete under the bot's booking rules and the actor's rights.
package reservations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/ports"
)

const (
	DefaultPerPage = 50
	MaxPerPage     = 200
	// MaxAuthorLength is the web_reservation.author column limit, in characters.
	MaxAuthorLength = 200
	// MinAuthorQuery is the shortest query the trigram index on the author can
	// serve (one trigram); a shorter one scans every reservation of the guild.
	MinAuthorQuery = 3
)

// MaxSnowflakeLength is the longest decimal Discord id (2^64 has 20 digits).
const MaxSnowflakeLength = 20

var (
	ErrForbidden       = errors.New("not allowed to change this reservation")
	ErrAuthorTooLong   = errors.New("author is too long")
	ErrAuthorIDInvalid = errors.New("author Discord id is not a Discord id")
	// ErrAuthorUnknown means the author of a new reservation could not be resolved.
	ErrAuthorUnknown = errors.New("author is unknown")
)

// Service implements ports.ReservationService.
type Service struct {
	booker       ports.BookingService
	reservations ports.ReservationRepository
	spots        ports.SpotRepository
	notifier     ports.BotNotifier
	log          *zap.SugaredLogger
	now          func() time.Time
}

func New(booker ports.BookingService, reservations ports.ReservationRepository, spots ports.SpotRepository, notifier ports.BotNotifier, log *zap.SugaredLogger) *Service {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Service{booker: booker, reservations: reservations, spots: spots, notifier: notifier, log: log, now: time.Now}
}

// CanEdit reports whether the actor may change r: a manager any reservation, a
// member with the reserve right their own. Nobody edits an ended reservation.
func CanEdit(actor reservation.Actor, r reservation.Reservation, now time.Time) bool {
	if !r.EndAt.After(now) {
		return false
	}
	return actor.Caps.Manage || isOwner(actor, r)
}

// CanDelete reports whether the actor may delete r: a manager any reservation,
// even a past one, a member with the reserve right their own upcoming one.
func CanDelete(actor reservation.Actor, r reservation.Reservation, now time.Time) bool {
	return actor.Caps.Manage || (isOwner(actor, r) && r.EndAt.After(now))
}

func isOwner(actor reservation.Actor, r reservation.Reservation) bool {
	return actor.Caps.Reserve && actor.UserID != "" && r.AuthorDiscordID == actor.UserID
}

// NormalizeFilter fixes the scope, the author text and the date order.
func NormalizeFilter(f reservation.SearchFilter) reservation.SearchFilter {
	switch f.Scope {
	case reservation.ScopeAll, reservation.ScopePast, reservation.ScopeUpcoming:
	default:
		f.Scope = reservation.ScopeUpcoming
	}
	f.Author = strings.TrimSpace(f.Author)
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		f.From, f.To = f.To, f.From
	}
	switch {
	case f.Limit <= 0:
		f.Limit = DefaultPerPage
	case f.Limit > MaxPerPage:
		f.Limit = MaxPerPage
	}
	return f
}

func (s *Service) Search(ctx context.Context, filter reservation.SearchFilter, page int) (*reservation.Page, error) {
	f := NormalizeFilter(filter)
	total, err := s.reservations.CountReservations(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("count reservations: %w", err)
	}
	pages := int((total + int64(f.Limit) - 1) / int64(f.Limit))
	if page > pages {
		page = pages
	}
	if page < 1 {
		page = 1
	}
	out := &reservation.Page{Items: []*reservation.ReservationWithSpot{}, Total: total, Page: page, Pages: pages, PerPage: f.Limit}
	if total == 0 {
		return out, nil
	}
	f.Offset = (page - 1) * f.Limit
	items, err := s.reservations.SearchReservationsWithSpot(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("search reservations: %w", err)
	}
	out.Items = items
	return out, nil
}

func (s *Service) KnownAuthors(ctx context.Context, guildID, query string) ([]*reservation.KnownAuthor, error) {
	query = strings.TrimSpace(query)
	if utf8.RuneCountInString(query) < MinAuthorQuery {
		return []*reservation.KnownAuthor{}, nil
	}
	authors, err := s.reservations.SelectKnownAuthors(ctx, guildID, query)
	if err != nil {
		return nil, fmt.Errorf("select known authors: %w", err)
	}
	return authors, nil
}

func (s *Service) Get(ctx context.Context, guildID string, id int64) (*reservation.ReservationWithSpot, error) {
	r, err := s.reservations.SelectGuildReservationWithSpot(ctx, guildID, id)
	if err != nil {
		return nil, fmt.Errorf("select reservation: %w", err)
	}
	return r, nil
}

func (s *Service) Create(ctx context.Context, guildID string, actor reservation.Actor, draft reservation.Draft) ([]*reservation.ClippedOrRemovedReservation, error) {
	if !actor.Caps.Reserve {
		return nil, ErrForbidden
	}
	author, authorID, err := authorOf(actor, draft, actor.Name, actor.UserID)
	if err != nil {
		return nil, err
	}
	if author == "" {
		return nil, ErrAuthorUnknown
	}
	if err := booking.CheckNewWindow(draft.StartAt, draft.EndAt, s.now()); err != nil {
		return nil, err
	}

	res, err := s.booker.Book(book.BookRequest{
		Guild:          &guild.Guild{ID: guildID},
		Member:         &member.Member{ID: authorID, Nick: author, Username: author},
		SpotID:         draft.SpotID,
		StartAt:        draft.StartAt,
		EndAt:          draft.EndAt,
		Overbook:       draft.Overbook,
		HasPermissions: actor.Caps.Overbook,
	})
	if err != nil {
		return res, err
	}
	s.summaryChanged(ctx, guildID)
	return res, nil
}

// Edit leaves the load, the ended check and the booking rules to the booking
// service, and only adds the actor's rights. An empty author keeps the current one.
func (s *Service) Edit(ctx context.Context, guildID string, actor reservation.Actor, id int64, draft reservation.Draft) ([]*reservation.Reservation, error) {
	author, authorID, err := authorOf(actor, draft, "", "")
	if err != nil {
		return nil, err
	}
	conflicts, err := s.booker.Edit(ctx, book.EditRequest{
		GuildID:         guildID,
		ReservationID:   id,
		SpotID:          draft.SpotID,
		StartAt:         draft.StartAt,
		EndAt:           draft.EndAt,
		Author:          author,
		AuthorDiscordID: authorID,
		Authorize: func(existing reservation.Reservation) error {
			if !CanEdit(actor, existing, s.now()) {
				return ErrForbidden
			}
			return nil
		},
	})
	if err != nil {
		return conflicts, err
	}
	s.summaryChanged(ctx, guildID)
	return nil, nil
}

func (s *Service) Delete(ctx context.Context, guildID string, actor reservation.Actor, id int64) error {
	existing, err := s.Get(ctx, guildID, id)
	if err != nil {
		return err
	}
	if !CanDelete(actor, existing.Reservation, s.now()) {
		return ErrForbidden
	}
	err = s.booker.DeleteForGuild(ctx, guildID, id)
	if err != nil {
		return fmt.Errorf("delete reservation: %w", err)
	}
	s.summaryChanged(ctx, guildID)
	return nil
}

func (s *Service) Spots(ctx context.Context, guildID string) ([]*spot.Spot, error) {
	spots, err := s.spots.SelectGuildSpots(ctx, guildID, true)
	if err != nil {
		return nil, fmt.Errorf("select spots: %w", err)
	}
	return spots, nil
}

func (s *Service) SpotOverview(ctx context.Context, guildID string, spotID int64) (*spot.Listed, error) {
	sp, err := s.spots.SelectGuildSpotByID(ctx, guildID, spotID)
	if err != nil {
		return nil, fmt.Errorf("select spot: %w", err)
	}
	counts, err := s.spots.SelectSpotReservationCounts(ctx, guildID, spotID)
	if err != nil {
		return nil, fmt.Errorf("count spot reservations: %w", err)
	}
	return &spot.Listed{Spot: *sp, Reservations: counts}, nil
}

// authorOf picks the author of a reservation. Only a manager chooses one; an
// empty author keeps the fallback. A free-text author has no Discord id.
func authorOf(actor reservation.Actor, draft reservation.Draft, fallback, fallbackID string) (string, string, error) {
	if !actor.Caps.Manage {
		return fallback, fallbackID, nil
	}
	author := strings.TrimSpace(draft.Author)
	if author == "" {
		return fallback, fallbackID, nil
	}
	if utf8.RuneCountInString(author) > MaxAuthorLength {
		return "", "", ErrAuthorTooLong
	}
	authorID := strings.TrimSpace(draft.AuthorDiscordID)
	if authorID != "" && !isSnowflake(authorID) {
		return "", "", ErrAuthorIDInvalid
	}
	return author, authorID, nil
}

func isSnowflake(id string) bool {
	if len(id) > MaxSnowflakeLength {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *Service) summaryChanged(ctx context.Context, guildID string) {
	if err := s.notifier.SummaryChanged(ctx, guildID); err != nil {
		s.log.Warnw("notify summary refresh", "guild", guildID, "error", err)
	}
}
