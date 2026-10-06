package booking

import (
	"errors"
	"fmt"
	"time"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

var ErrInsufficientPermissions = fmt.Errorf("there are conflicting reservations which prevented booking this reservation. If you would like to overbook them, ensure you hold an overbook rank (or the @%s role), then repeat the command and set 'overbook' parameter to 'true'", discord.PrivilegedRole)

var ErrReserveNotAllowed = errors.New("you do not hold a rank that is allowed to book respawns on this server")

// OverbookAllowed reports whether a booking that asks to overbook the conflicts
// would pass: never over the member's own reservation, else with the overbook
// right or over one reservation that looks abandoned.
func OverbookAllowed(hasPermissions bool, userID string, conflicts []*reservation.Reservation, now time.Time) bool {
	if ownsAny(userID, conflicts) {
		return false
	}
	return hasPermissions || isPotentiallyAbandonedReservation(conflicts, now)
}

// ownsAny is false for an empty userID: a free-text author booked in the web has
// no Discord id to compare.
func ownsAny(userID string, reservations []*reservation.Reservation) bool {
	if userID == "" {
		return false
	}
	for _, r := range reservations {
		if r.AuthorDiscordID == userID {
			return true
		}
	}
	return false
}

// This is an edge case, where we check:
// if there is only one overlapping reservation,
// and if it started at least 10 minutes ago,
// and if it hasn't ended,
// and it contains our reservation request and time
func isPotentiallyAbandonedReservation(overlappingReservations []*reservation.Reservation, now time.Time) bool {
	return len(overlappingReservations) == 1 &&
		overlappingReservations[0].StartAt.Add(10*time.Minute).Before(now) &&
		(overlappingReservations[0].EndAt.After(now) ||
			overlappingReservations[0].EndAt.Equal(now))
}
