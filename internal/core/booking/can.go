package booking

import (
	"fmt"
	"time"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

var ErrInsufficientPermissions = fmt.Errorf("there are conflicting reservations which prevented booking this reservation. If you would like to overbook them, ensure you have a @%s role, then repeat the command and set 'overbook' parameter to 'true'", discord.PrivilegedRole)

func canOverbook(attemptsToOverbook bool, hasPermissions bool, conflictingReservations []*reservation.Reservation) bool {
	return (attemptsToOverbook && isPotentiallyAbandonedReservation(conflictingReservations)) ||
		(attemptsToOverbook && hasPermissions)

}

// This is an edge case, where we check:
// if there is only one overlapping reservation,
// and if it started,
// and if it hasn't ended,
// and it contains our reservation request and time
func isPotentiallyAbandonedReservation(overlappingReservations []*reservation.Reservation) bool {
	return len(overlappingReservations) == 1 &&
		overlappingReservations[0].StartAt.Before(time.Now()) &&
		(overlappingReservations[0].EndAt.After(time.Now()) ||
			overlappingReservations[0].EndAt.Equal(time.Now()))
}
