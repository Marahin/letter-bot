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

func (s *Service) Join(ctx context.Context, guildID string, actor reservation.Actor, spotID int64, length time.Duration) (*queue.JoinOutcome, error) {
	if !actor.Caps.Reserve {
		return nil, booking.ErrReserveNotAllowed
	}
	if length == 0 {
		length = DefaultLength
	}
	if length < MinLength || length > MaxLength || length%LengthStep != 0 {
		return nil, ErrInvalidLength
	}
	sp, err := s.spots.SelectGuildSpotByID(ctx, guildID, spotID)
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
	others, err := s.repo.CountUserEntries(ctx, guildID, actor.UserID, spotID, now)
	if err != nil {
		return nil, fmt.Errorf("count queue entries: %w", err)
	}
	if others >= MaxEntriesPerUser {
		return nil, ErrQueueFull
	}
	e, err := s.repo.UpsertEntry(ctx, queue.Entry{
		GuildID:   guildID,
		SpotID:    spotID,
		SpotName:  sp.Name,
		UserID:    actor.UserID,
		Length:    length,
		ExpiresAt: now.Add(EntryTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("upsert queue entry: %w", err)
	}

	free, err := s.isFree(ctx, guildID, e.ID, now)
	if err != nil {
		return nil, err
	}
	outcome := &queue.JoinOutcome{Entry: *e}
	if !free {
		return outcome, nil
	}
	at := now.Truncate(time.Second)
	claimed, err := s.repo.ClaimOffer(ctx, e.ID, at)
	if err != nil {
		return nil, fmt.Errorf("claim offer: %w", err)
	}
	if claimed {
		outcome.Entry.OfferedAt = &at
		outcome.FreeNow = true
	}
	return outcome, nil
}

// isFree asks the same query as DueOffers, so the two agree on "free".
func (s *Service) isFree(ctx context.Context, guildID string, entryID int64, now time.Time) (bool, error) {
	candidates, err := s.repo.SelectGuildCandidates(ctx, guildID, now, BookingStart(now))
	if err != nil {
		return false, fmt.Errorf("select queue candidates: %w", err)
	}
	for _, c := range candidates {
		if c.ID == entryID {
			return c.Free, nil
		}
	}
	return false, nil
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
	if _, err := s.repo.DeleteExpired(ctx, guildID, now); err != nil {
		return nil, fmt.Errorf("delete expired queue entries: %w", err)
	}
	candidates, err := s.repo.SelectGuildCandidates(ctx, guildID, now, BookingStart(now))
	if err != nil {
		return nil, fmt.Errorf("select queue candidates: %w", err)
	}
	var taken []int64
	var due []queue.Entry
	at := now.Truncate(time.Second)
	for _, c := range candidates {
		switch {
		case c.OfferedAt != nil && !c.Free:
			taken = append(taken, c.ID)
		case c.OfferedAt == nil && c.Free:
			claimed, err := s.repo.ClaimOffer(ctx, c.ID, at)
			if err != nil {
				return due, fmt.Errorf("claim offer %d: %w", c.ID, err)
			}
			if claimed {
				e := c.Entry
				e.OfferedAt = &at
				due = append(due, e)
			}
		}
	}
	if len(taken) == 0 {
		return due, nil
	}
	if err := s.repo.ResetOffers(ctx, taken); err != nil {
		return due, fmt.Errorf("reset queue offers: %w", err)
	}
	return due, nil
}

func (s *Service) Unclaim(ctx context.Context, entryID int64) error {
	return s.repo.ResetOffers(ctx, []int64{entryID})
}

// CheckOffer does not need the entry's offer to be current: after the respawn
// was taken again the offer is cleared, and the member gets the booking's own
// error. Only a newer offer replaces an older one.
func (s *Service) CheckOffer(e *queue.Entry, userID string, offeredAt time.Time) error {
	if e.UserID != userID {
		return ErrOfferNotYours
	}
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
	if err := s.CheckOffer(e, actor.UserID, offeredAt); err != nil {
		return nil, err
	}
	start := BookingStart(s.now())
	outcome, err := s.forms.Book(ctx, e.GuildID, actor, reservation.Draft{SpotID: e.SpotID, StartAt: start, EndAt: start.Add(e.Length)})
	if err != nil {
		return outcome, err
	}
	// The booking removes the entry in the background too (Fulfil), so a
	// failure here only delays it and must not report the booking as failed.
	_, _ = s.repo.DeleteEntry(ctx, e.ID, actor.UserID)
	return outcome, nil
}

func (s *Service) Fulfil(ctx context.Context, guildID, userID string, spotID int64) error {
	if _, err := s.repo.DeleteUserSpotEntry(ctx, guildID, userID, spotID); err != nil {
		return fmt.Errorf("delete queue entry: %w", err)
	}
	return nil
}
