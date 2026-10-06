// Package reservationforms backs the bot's buttons and booking wizard and runs
// them through the reservations service, under the same rules as the web.
package reservationforms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/ports"
)

// Service implements ports.ReservationFormService.
type Service struct {
	reservations ports.ReservationService
	spots        ports.SpotRepository
	now          func() time.Time
}

func New(service ports.ReservationService, spots ports.SpotRepository) *Service {
	return &Service{reservations: service, spots: spots, now: time.Now}
}

func (s *Service) Book(ctx context.Context, guildID string, actor reservation.Actor, draft reservation.Draft) (*reservation.FormOutcome, error) {
	outcome := &reservation.FormOutcome{Draft: draft}
	sp, err := s.spotByID(ctx, guildID, draft.SpotID)
	if errors.Is(err, ports.ErrNotFound) {
		return outcome, booking.ErrSpotNotFound
	}
	if err != nil {
		return outcome, err
	}
	outcome.SpotName = sp.Name
	return s.submitBooking(ctx, guildID, actor, outcome)
}

func (s *Service) submitBooking(ctx context.Context, guildID string, actor reservation.Actor, outcome *reservation.FormOutcome) (*reservation.FormOutcome, error) {
	draft := outcome.Draft
	draft.Author, draft.AuthorDiscordID = "", ""
	res, err := s.reservations.Create(ctx, guildID, actor, draft)
	if errors.Is(err, booking.ErrInsufficientPermissions) {
		for _, r := range res {
			outcome.Conflicts = append(outcome.Conflicts, r.Original)
		}
		outcome.CanOverbook = !draft.Overbook && booking.OverbookAllowed(actor.Caps.Overbook, actor.UserID, outcome.Conflicts, s.now())
		return outcome, err
	}
	if err != nil {
		return outcome, err
	}
	outcome.Overbooked = res
	return outcome, nil
}

func (s *Service) edit(ctx context.Context, guildID string, actor reservation.Actor, id int64, draft reservation.Draft) (*reservation.FormOutcome, error) {
	outcome := &reservation.FormOutcome{Draft: draft}
	sp, err := s.spotByID(ctx, guildID, draft.SpotID)
	if errors.Is(err, ports.ErrNotFound) {
		return outcome, booking.ErrSpotNotFound
	}
	if err != nil {
		return outcome, err
	}
	outcome.SpotName = sp.Name
	return s.submitEdit(ctx, guildID, actor, id, outcome)
}

func (s *Service) submitEdit(ctx context.Context, guildID string, actor reservation.Actor, id int64, outcome *reservation.FormOutcome) (*reservation.FormOutcome, error) {
	draft := outcome.Draft
	draft.Author, draft.AuthorDiscordID, draft.Overbook = "", "", false
	conflicts, err := s.reservations.Edit(ctx, guildID, actor, id, draft)
	outcome.Conflicts = conflicts
	return outcome, err
}

func (s *Service) Editable(ctx context.Context, guildID string, actor reservation.Actor, id int64) (*reservation.ReservationWithSpot, error) {
	r, err := s.reservations.Get(ctx, guildID, id)
	if err != nil {
		return nil, err
	}
	if !r.EndAt.After(s.now()) {
		return nil, booking.ErrReservationEnded
	}
	if !reservations.CanEdit(actor, r.Reservation, s.now()) {
		return nil, reservations.ErrForbidden
	}
	return r, nil
}

func (s *Service) Cancellable(ctx context.Context, guildID string, actor reservation.Actor, id int64) (*reservation.ReservationWithSpot, error) {
	r, err := s.reservations.Get(ctx, guildID, id)
	if err != nil {
		return nil, err
	}
	if !reservations.CanDelete(actor, r.Reservation, s.now()) {
		return nil, reservations.ErrForbidden
	}
	return r, nil
}

func (s *Service) Cancel(ctx context.Context, guildID string, actor reservation.Actor, id int64) (*reservation.ReservationWithSpot, error) {
	r, err := s.Cancellable(ctx, guildID, actor, id)
	if err != nil {
		return nil, err
	}
	if err := s.reservations.Delete(ctx, guildID, actor, id); err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Service) Mine(ctx context.Context, guildID string, actor reservation.Actor, limit int) (*reservation.Page, error) {
	if actor.UserID == "" {
		return &reservation.Page{Items: []*reservation.ReservationWithSpot{}}, nil
	}
	return s.reservations.Search(ctx, reservation.SearchFilter{
		GuildID:         guildID,
		AuthorDiscordID: actor.UserID,
		Scope:           reservation.ScopeUpcoming,
		Limit:           limit,
	}, 1)
}

func (s *Service) spotByID(ctx context.Context, guildID string, id int64) (*spot.Spot, error) {
	sp, err := s.spots.SelectGuildSpotByID(ctx, guildID, id)
	if err != nil {
		return nil, fmt.Errorf("select spot: %w", err)
	}
	return sp, nil
}
