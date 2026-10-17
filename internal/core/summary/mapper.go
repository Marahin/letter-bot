package summary

import (
	"time"

	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

// MapReservation shows the online status only for a reservation in progress.
// A status on every upcoming one would change the summary at each login or logout.
func (a *Adapter) MapReservation(res *reservation.Reservation) *summary.Booking {
	status := summary.Unknown
	if ongoing(res, a.now()) {
		status = a.onlineCheck.PlayerStatus(res.GuildID, res.Author)
	}
	return &summary.Booking{
		Author:          res.Author,
		StartAt:         res.StartAt,
		EndAt:           res.EndAt,
		AuthorDiscordID: res.AuthorDiscordID,
		Status:          status,
	}
}

func (a *Adapter) MapReservations(reservations []*reservation.Reservation) []*summary.Booking {
	return collections.PoorMansMap(reservations, func(res *reservation.Reservation) *summary.Booking {
		return a.MapReservation(res)
	})
}

func ongoing(res *reservation.Reservation, now time.Time) bool {
	return !res.StartAt.After(now) && now.Before(res.EndAt)
}
