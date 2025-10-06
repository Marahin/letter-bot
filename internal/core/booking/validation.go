package booking

import (
	"errors"
	"time"

	"spot-assistant/internal/core/dto/member"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/reservation"
)

const MaximumReservationLength = 3 * time.Hour

func validateHuntLength(t time.Duration) error {
	if t > MaximumReservationLength {
		return errors.New("reservation cannot take more than 3 hours")
	}

	return nil
}

func validateNoSelfOverbook(member *member.Member, conflictingReservations []*reservation.Reservation) error {
	authorsConflictingReservations, _ := collections.PoorMansFind(conflictingReservations, func(r *reservation.Reservation) bool {
		return r.AuthorDiscordID == member.ID
	})

	if authorsConflictingReservations != nil {
		return errors.New("you cannot overbook yourself")
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
		return errors.New("you can only book 3 hours of reservations within 24 hour window")
	}

	return nil
}
