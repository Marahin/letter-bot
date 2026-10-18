package summary

import (
	"time"

	"spot-assistant/internal/core/dto/reservation"
)

// NextRefresh returns the earliest start or end of the reservations after now.
// The summary changes at each of them: a reservation ends, or its online status shows.
func NextRefresh(reservations []*reservation.ReservationWithSpot, now time.Time) (time.Time, bool) {
	var next time.Time
	for _, r := range reservations {
		for _, t := range []time.Time{r.StartAt, r.EndAt} {
			if t.After(now) && (next.IsZero() || t.Before(next)) {
				next = t
			}
		}
	}
	return next, !next.IsZero()
}
