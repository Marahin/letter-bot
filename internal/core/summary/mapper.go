package summary

import (
	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func (a *Adapter) MapReservation(res *reservation.Reservation) *summary.Booking {
	return &summary.Booking{
		Author:          res.Author,
		StartAt:         res.StartAt,
		EndAt:           res.EndAt,
		AuthorDiscordID: res.AuthorDiscordID,
		Status:          a.onlineCheck.PlayerStatus(res.GuildID, res.Author),
	}
}

func (a *Adapter) MapReservations(reservations []*reservation.Reservation) []*summary.Booking {
	return collections.PoorMansMap(reservations, func(res *reservation.Reservation) *summary.Booking {
		return a.MapReservation(res)
	})
}
