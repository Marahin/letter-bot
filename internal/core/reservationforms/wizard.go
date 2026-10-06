package reservationforms

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/ports"
)

// bookedLimit is far above the reservations that fit in one 12-hour window.
const bookedLimit = 100

func (s *Service) RespawnPicker(ctx context.Context, guildID string, actor reservation.Actor, page int) (*reservation.RespawnPicker, error) {
	spots, err := s.spots.SelectGuildSpots(ctx, guildID, false)
	if err != nil {
		return nil, fmt.Errorf("select spots: %w", err)
	}
	now := s.now()
	var mine []spot.Ranked
	if actor.UserID != "" {
		mine, err = s.spots.SelectTopGuildSpots(ctx, guildID, actor.UserID, now.Add(-UsualPeriod), SelectOptions)
		if err != nil {
			return nil, fmt.Errorf("select usual spots: %w", err)
		}
	}
	popular, err := s.spots.SelectTopGuildSpots(ctx, guildID, "", now.Add(-PopularPeriod), SelectOptions)
	if err != nil {
		return nil, fmt.Errorf("select popular spots: %w", err)
	}
	picker := PageRespawns(spots, page)
	picker.Usual = UsualRespawns(mine, popular, SelectOptions)
	return &picker, nil
}

func (s *Service) TimePicker(ctx context.Context, guildID string, actor reservation.Actor, choice reservation.TimeChoice) (*reservation.TimePicker, error) {
	now := s.now()
	picker := &reservation.TimePicker{Windows: Windows}
	var currentStart time.Time
	var currentLength time.Duration
	if choice.ReservationID != 0 {
		editing, err := s.Editable(ctx, guildID, actor, choice.ReservationID)
		if err != nil {
			return nil, err
		}
		choice = withEditDefaults(choice, editing)
		picker.Editing = editing
		picker.Ongoing = !editing.StartAt.After(now)
		currentStart, currentLength = editing.StartAt.Truncate(time.Second), editing.EndAt.Sub(editing.StartAt)
		if picker.Ongoing {
			if choice.SpotID != editing.SpotID {
				return nil, booking.ErrSpotLocked
			}
			choice.Now, choice.StartAt, choice.Window = false, currentStart, 0
		}
	}
	if choice.SpotID == 0 {
		return nil, booking.ErrSpotNotFound
	}
	sp, err := s.activeSpot(ctx, guildID, choice.SpotID)
	if err != nil {
		return nil, err
	}
	picker.Spot = reservation.Spot{ID: sp.ID, Name: sp.Name}

	if choice.Window == reservation.AutoWindow {
		choice.Window = 0
		if !choice.Now && !choice.StartAt.IsZero() {
			choice.Window = WindowOf(choice.StartAt, now)
		}
	}
	choice.Window = min(max(choice.Window, 0), Windows-1)
	picker.Choice = choice

	from, to := now, now.Add(booking.MaximumReservationLength)
	if !picker.Ongoing {
		slots := StartSlots(now, choice.Window)
		from, to = slots[0], slots[len(slots)-1].Add(SlotStep)
		if choice.Window == 0 {
			from = now
		}
	}
	picker.Booked, err = s.booked(ctx, guildID, sp.ID, choice.ReservationID, from, to)
	if err != nil {
		return nil, err
	}

	if picker.Ongoing {
		picker.Starts = []reservation.StartOption{{StartAt: currentStart, Keep: true, Selected: true}}
		picker.Lengths = lengthOptions(choice.Length, currentLength, currentStart, now)
		return picker, nil
	}
	if currentStart.Before(now) {
		currentStart = time.Time{}
	}
	picker.Starts = startOptions(now, choice.Window, choice, currentStart, picker.Booked)
	picker.Lengths = lengthOptions(choice.Length, currentLength, time.Time{}, time.Time{})
	return picker, nil
}

// withEditDefaults fills what the member did not change with the reservation.
func withEditDefaults(choice reservation.TimeChoice, editing *reservation.ReservationWithSpot) reservation.TimeChoice {
	if choice.SpotID == 0 {
		choice.SpotID = editing.SpotID
	}
	if !choice.HasStart() {
		choice.StartAt = editing.StartAt.Truncate(time.Second)
	}
	if choice.Length == 0 {
		choice.Length = editing.EndAt.Sub(editing.StartAt)
	}
	return choice
}

func (s *Service) activeSpot(ctx context.Context, guildID string, id int64) (*spot.Spot, error) {
	sp, err := s.spotByID(ctx, guildID, id)
	if errors.Is(err, ports.ErrNotFound) {
		return nil, booking.ErrSpotNotFound
	}
	if err != nil {
		return nil, err
	}
	if sp.IsArchived() {
		return nil, booking.ErrSpotArchived
	}
	return sp, nil
}

func (s *Service) booked(ctx context.Context, guildID string, spotID, excludeID int64, from, to time.Time) ([]*reservation.Reservation, error) {
	page, err := s.reservations.Search(ctx, reservation.SearchFilter{
		GuildID: guildID,
		SpotID:  &spotID,
		From:    &from,
		To:      &to,
		Scope:   reservation.ScopeAll,
		Limit:   bookedLimit,
	}, 1)
	if err != nil {
		return nil, fmt.Errorf("search reservations: %w", err)
	}
	booked := make([]*reservation.Reservation, 0, len(page.Items))
	for _, r := range page.Items {
		// The search takes the edges: a reservation that ends at the first slot does not hold it.
		if r.Reservation.ID == excludeID || !r.EndAt.After(from) {
			continue
		}
		res := r.Reservation
		booked = append(booked, &res)
	}
	slices.SortFunc(booked, func(a, b *reservation.Reservation) int { return a.StartAt.Compare(b.StartAt) })
	return booked, nil
}

func (s *Service) BookChoice(ctx context.Context, guildID string, actor reservation.Actor, choice reservation.TimeChoice) (*reservation.FormOutcome, error) {
	if !choice.Complete() {
		return nil, ErrChoiceIncomplete
	}
	startAt, err := ResolveStart(choice, s.now())
	if err != nil {
		return nil, err
	}
	return s.Book(ctx, guildID, actor, reservation.Draft{SpotID: choice.SpotID, StartAt: startAt, EndAt: startAt.Add(choice.Length)})
}

func (s *Service) EditChoice(ctx context.Context, guildID string, actor reservation.Actor, choice reservation.TimeChoice) (*reservation.FormOutcome, error) {
	existing, err := s.Editable(ctx, guildID, actor, choice.ReservationID)
	if err != nil {
		return nil, err
	}
	if !choice.Complete() {
		return nil, ErrChoiceIncomplete
	}
	// A custom id holds Unix seconds; the kept start is the stored one.
	startAt := existing.StartAt
	if choice.Now || choice.StartAt.Unix() != existing.StartAt.Unix() {
		if startAt, err = ResolveStart(choice, s.now()); err != nil {
			return nil, err
		}
	}
	draft := reservation.Draft{SpotID: choice.SpotID, StartAt: startAt, EndAt: startAt.Add(choice.Length)}
	return s.edit(ctx, guildID, actor, choice.ReservationID, draft)
}
