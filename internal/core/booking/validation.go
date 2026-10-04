package booking

import (
	"errors"
	"time"

	"spot-assistant/internal/core/dto/member"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/reservation"
)

const MaximumReservationLength = 3 * time.Hour

var (
	ErrReservationTooLong = errors.New("reservation cannot take more than 3 hours")
	ErrSelfOverbook       = errors.New("you cannot overbook yourself")
	ErrQuotaExceeded      = errors.New("you can only book 3 hours of reservations within 24 hour window")
)

func validateHuntLength(t time.Duration) error {
	if t > MaximumReservationLength {
		return ErrReservationTooLong
	}

	return nil
}

func validateNoSelfOverbook(m *member.Member, conflictingReservations []*reservation.Reservation) error {
	// A free-text author booked in the web has no Discord id to compare.
	if m.ID == "" {
		return nil
	}
	authorsConflictingReservations, _ := collections.PoorMansFind(conflictingReservations, func(r *reservation.Reservation) bool {
		return r.AuthorDiscordID == m.ID
	})

	if authorsConflictingReservations != nil {
		return ErrSelfOverbook
	}

	return nil
}

// Check for potentially exceeding maximum hours, with an exception for multi-floor respawns
func validateHuntLengthForMultiFloorRespawns(spotName string, upcomingAuthorReservations []*reservation.ReservationWithSpot, startAt, endAt time.Time) error {
	tempReservation := reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{
			ID:      -1,
			StartAt: startAt,
			EndAt:   endAt,
		},
		Spot: reservation.Spot{
			Name: spotName,
		},
	}
	upcomingAuthorReservations = append(upcomingAuthorReservations, &tempReservation)

	reducedReservations := reduceAllAuthorReservationsByLongestPerSpot(upcomingAuthorReservations)
	totalReservationsTime := collections.PoorMansSum(reducedReservations, func(reservation *reservation.ReservationWithSpot) time.Duration {
		return reservation.EndAt.Sub(reservation.StartAt)
	})

	if totalReservationsTime > MaximumReservationLength {
		return ErrQuotaExceeded
	}

	return nil
}
