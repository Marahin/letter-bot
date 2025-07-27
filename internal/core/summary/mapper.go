package summary

import (
	"spot-assistant/internal/common/collections"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/summary"
)

func (a *Adapter) MapReservation(reservation *reservation.Reservation) *summary.Booking {
	return &summary.Booking{
		Author:          reservation.Author,
		StartAt:         reservation.StartAt,
		EndAt:           reservation.EndAt,
		AuthorDiscordID: reservation.AuthorDiscordID,
		Status:          a.onlineCheck.PlayerStatus(reservation.GuildID, reservation.Author),
	}
}

func (a *Adapter) MapReservations(reservations []*reservation.Reservation) []*summary.Booking {
	return collections.PoorMansMap(reservations, func(res *reservation.Reservation) *summary.Booking {
		return a.MapReservation(res)
	})
}
