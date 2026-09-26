package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/ports"
)

var (
	ErrInvalidRange     = errors.New("the reservation must end after it starts")
	ErrStartInPast      = errors.New("the reservation cannot start in the past")
	ErrReservationEnded = errors.New("the reservation has already ended")
	ErrSpotArchived     = errors.New("the respawn is archived")
	ErrSpotNotFound     = errors.New("the respawn does not exist")
	// ErrConflict means another reservation of the respawn overlaps the new times.
	// An edit never overbooks.
	ErrConflict = errors.New("another reservation of the respawn overlaps these times")
)

// CheckNewWindow validates the times of a new reservation: it ends after it
// starts and does not start before the current minute.
func CheckNewWindow(startAt, endAt, now time.Time) error {
	if !endAt.After(startAt) {
		return ErrInvalidRange
	}
	if startAt.Before(now.Truncate(time.Minute)) {
		return ErrStartInPast
	}
	return nil
}

// Edit applies the bot's booking rules (maximum length, per-author quota) to a
// changed reservation and refuses any overlap. On ErrConflict it returns the
// overlapping reservations.
func (a *Adapter) Edit(ctx context.Context, req book.EditRequest) ([]*reservation.Reservation, error) {
	existing, err := a.reservationRepo.SelectGuildReservationWithSpot(ctx, req.GuildID, req.ReservationID)
	if err != nil {
		return nil, err
	}
	now := a.now()
	if !existing.EndAt.After(now) {
		return nil, ErrReservationEnded
	}
	if !req.EndAt.After(req.StartAt) {
		return nil, ErrInvalidRange
	}
	// An ongoing reservation keeps its start; a moved start must not be in the past.
	if !req.StartAt.Equal(existing.StartAt) && req.StartAt.Before(now.Truncate(time.Minute)) {
		return nil, ErrStartInPast
	}
	if err := validateHuntLength(req.EndAt.Sub(req.StartAt)); err != nil {
		return nil, err
	}

	spotName := existing.Spot.Name
	if req.SpotID != existing.SpotID {
		sp, err := a.spotRepo.SelectGuildSpotByID(ctx, req.GuildID, req.SpotID)
		if errors.Is(err, ports.ErrNotFound) {
			return nil, ErrSpotNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("select spot: %w", err)
		}
		if sp.IsArchived() {
			return nil, ErrSpotArchived
		}
		spotName = sp.Name
	}

	g := &guild.Guild{ID: req.GuildID}
	m := &member.Member{ID: req.AuthorDiscordID}
	if err := a.validateAuthorQuota(ctx, g, m, spotName, req.StartAt, req.EndAt, existing.Reservation.ID); err != nil {
		return nil, err
	}

	conflicts, err := a.reservationRepo.SelectOverlappingReservationsBySpotID(ctx, req.GuildID, req.SpotID, req.StartAt, req.EndAt, existing.Reservation.ID)
	if err != nil {
		return nil, fmt.Errorf("select overlapping reservations: %w", err)
	}
	if len(conflicts) > 0 {
		return conflicts, ErrConflict
	}

	err = a.reservationRepo.UpdateReservation(ctx, req.GuildID, reservation.Reservation{
		ID:              existing.Reservation.ID,
		SpotID:          req.SpotID,
		StartAt:         req.StartAt,
		EndAt:           req.EndAt,
		Author:          req.Author,
		AuthorDiscordID: req.AuthorDiscordID,
	})
	if errors.Is(err, ports.ErrConflict) {
		return nil, ErrConflict
	}
	return nil, err
}

// DeleteForGuild deletes any reservation of the guild. Returns ports.ErrNotFound.
func (a *Adapter) DeleteForGuild(ctx context.Context, guildID string, id int64) error {
	return a.reservationRepo.DeleteGuildReservation(ctx, guildID, id)
}

// validateAuthorQuota checks the 3 h per author limit over their upcoming
// reservations, leaving out excludeID (0 = none). A free-text author booked in
// the web has no Discord id, so there is nothing to count.
func (a *Adapter) validateAuthorQuota(ctx context.Context, g *guild.Guild, m *member.Member, spotName string, startAt, endAt time.Time, excludeID int64) error {
	if m.ID == "" {
		return nil
	}
	upcoming, err := a.reservationRepo.SelectUpcomingMemberReservationsWithSpots(ctx, g, m)
	if err != nil {
		return fmt.Errorf("could not select upcoming member reservations: %w", err)
	}
	others := make([]*reservation.ReservationWithSpot, 0, len(upcoming))
	for _, r := range upcoming {
		if r.Reservation.ID != excludeID {
			others = append(others, r)
		}
	}
	return validateHuntLengthForMultiFloorRespawns(spotName, others, startAt, endAt)
}
