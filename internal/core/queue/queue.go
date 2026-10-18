// Package queue is the respawn queue of the bot. A member waits for a taken
// respawn; when it is free, the bot offers it, and the first member to book
// gets it.
package queue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/ports"
)

const (
	// OfferTTL is how long the Book and Cancel buttons of an offer work.
	OfferTTL = 10 * time.Minute
	// EntryTTL is how long a member stays in a queue.
	EntryTTL      = 24 * time.Hour
	DefaultLength = 2 * time.Hour
	MinLength     = 30 * time.Minute
	MaxLength     = booking.MaximumReservationLength
	LengthStep    = 30 * time.Minute
	// MaxEntriesPerUser is the most respawns a member waits for in one guild.
	MaxEntriesPerUser = 3
)

var (
	ErrQueueFull     = fmt.Errorf("you can wait for at most %d respawns at a time", MaxEntriesPerUser)
	ErrInvalidLength = errors.New("the length must be 30 minutes to 3 hours, in steps of 30 minutes")
	ErrOfferNotYours = errors.New("this offer is for another member")
	ErrOfferExpired  = errors.New("this offer has expired")
)

// Service implements ports.SpotQueueService.
type Service struct {
	repo  ports.SpotQueueRepository
	spots ports.SpotRepository
	forms ports.ReservationFormService
	now   func() time.Time
}

func New(repo ports.SpotQueueRepository, spots ports.SpotRepository, forms ports.ReservationFormService) *Service {
	return &Service{repo: repo, spots: spots, forms: forms, now: time.Now}
}

// BookingStart is the start of a booking made from the queue at now: the next
// full minute. Booking counts the edges of a reservation, so the minute in
// which the previous hunt ended would still overlap it.
func BookingStart(now time.Time) time.Time {
	start := now.Truncate(time.Minute)
	if start.Before(now) {
		start = start.Add(time.Minute)
	}
	return start
}

func (s *Service) Join(ctx context.Context, req queue.JoinRequest) (*queue.JoinOutcome, error) {
	if !req.Actor.Caps.Reserve {
		return nil, booking.ErrReserveNotAllowed
	}
	length := req.Length
	if length == 0 {
		length = DefaultLength
	}
	if length < MinLength || length > MaxLength || length%LengthStep != 0 {
		return nil, ErrInvalidLength
	}
	sp, err := s.spots.SelectGuildSpotByID(ctx, req.GuildID, req.SpotID)
	if errors.Is(err, ports.ErrNotFound) {
		return nil, booking.ErrSpotNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select spot: %w", err)
	}
	if sp.IsArchived() {
		return nil, booking.ErrSpotArchived
	}

	now := s.now()
	others, err := s.repo.CountUserEntries(ctx, queue.UserCount{GuildID: req.GuildID, UserID: req.Actor.UserID, ExceptSpotID: req.SpotID, Now: now})
	if err != nil {
		return nil, fmt.Errorf("count queue entries: %w", err)
	}
	if others >= MaxEntriesPerUser {
		return nil, ErrQueueFull
	}
	e, err := s.repo.UpsertEntry(ctx, queue.Entry{
		GuildID:   req.GuildID,
		SpotID:    req.SpotID,
		SpotName:  sp.Name,
		UserID:    req.Actor.UserID,
		Length:    length,
		ExpiresAt: now.Add(EntryTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert queue entry: %w", err)
	}

	// The upsert cleared the offer, so the claim succeeds exactly when the respawn is free.
	at, claimed, err := s.claim(ctx, e.ID, now)
	if err != nil {
		return nil, err
	}
	outcome := &queue.JoinOutcome{Entry: *e}
	if claimed {
		outcome.Entry.OfferedAt = &at
		outcome.FreeNow = true
	}
	return outcome, nil
}

func (s *Service) Leave(ctx context.Context, userID string, entryID int64) error {
	deleted, err := s.repo.DeleteEntry(ctx, entryID, userID)
	if err != nil {
		return fmt.Errorf("delete queue entry: %w", err)
	}
	if !deleted {
		return ports.ErrNotFound
	}
	return nil
}

func (s *Service) Mine(ctx context.Context, guildID, userID string) ([]queue.Entry, error) {
	return s.repo.ListUserEntries(ctx, guildID, userID, s.now())
}

func (s *Service) Entry(ctx context.Context, id int64) (*queue.Entry, error) {
	return s.repo.GetEntry(ctx, id)
}

func (s *Service) DueOffers(ctx context.Context, guildID string) ([]queue.Entry, error) {
	now := s.now()
	if _, err := s.repo.DeleteExpired(ctx, now); err != nil {
		return nil, fmt.Errorf("delete expired queue entries: %w", err)
	}
	start := BookingStart(now)
	if err := s.repo.ResetTakenOffers(ctx, guildID, start); err != nil {
		return nil, fmt.Errorf("reset queue offers: %w", err)
	}
	due, err := s.repo.SelectDueEntries(ctx, queue.DueQuery{GuildID: guildID, Now: now, StartAt: start})
	if err != nil {
		return nil, fmt.Errorf("select due queue entries: %w", err)
	}
	return due, nil
}

func (s *Service) ClaimOffer(ctx context.Context, entryID int64) (time.Time, bool, error) {
	return s.claim(ctx, entryID, s.now())
}

// claim stamps the offer to the second: the offer's buttons carry it in Unix seconds.
func (s *Service) claim(ctx context.Context, entryID int64, now time.Time) (time.Time, bool, error) {
	at := now.Truncate(time.Second)
	claimed, err := s.repo.ClaimOffer(ctx, queue.Claim{EntryID: entryID, OfferedAt: at, StartAt: BookingStart(now)})
	if err != nil {
		return time.Time{}, false, fmt.Errorf("claim offer: %w", err)
	}
	if !claimed {
		return time.Time{}, false, nil
	}
	return at, true, nil
}

func (s *Service) Unclaim(ctx context.Context, entryID int64) error {
	return s.repo.ResetOffer(ctx, entryID)
}

// OfferUsable does not need the entry's offer to be current: after the respawn
// was taken again the offer is cleared, and the member gets the booking's own
// error. Only a newer offer replaces an older one.
func (s *Service) OfferUsable(e *queue.Entry, offeredAt time.Time) error {
	now := s.now()
	if offeredAt.After(now) || now.Sub(offeredAt) > OfferTTL {
		return ErrOfferExpired
	}
	if e.OfferedAt != nil && e.OfferedAt.Unix() != offeredAt.Unix() {
		return ErrOfferExpired
	}
	return nil
}

func (s *Service) Book(ctx context.Context, actor reservation.Actor, entryID int64, offeredAt time.Time) (*reservation.FormOutcome, error) {
	e, err := s.repo.GetEntry(ctx, entryID)
	if err != nil {
		return nil, err
	}
	if e.UserID != actor.UserID {
		return nil, ErrOfferNotYours
	}
	if err := s.OfferUsable(e, offeredAt); err != nil {
		return nil, err
	}
	start := BookingStart(s.now())
	return s.forms.Book(ctx, e.GuildID, actor, reservation.Draft{SpotID: e.SpotID, StartAt: start, EndAt: start.Add(e.Length)})
}
