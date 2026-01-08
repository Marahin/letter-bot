package upcomingreservation

import (
	"context"

	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/reservation"
)

func (a *Adapter) NotifyUpcomingReservations(ctx context.Context, g *guild.Guild) error {
	reservations, err := a.reservationRepo.SelectReservationsForReservationStartsNotification(ctx, g.ID)
	if err != nil {
		return err
	}

	for _, res := range reservations {
		go a.processReservationNotification(g, res)
	}

	return nil
}

func (a *Adapter) processReservationNotification(g *guild.Guild, res *reservation.ReservationWithSpot) {
	member, err := a.memberRepo.GetMemberByGuildAndId(g, res.Reservation.AuthorDiscordID)
	if err != nil {
		a.log.Errorf("could not fetch member %s for notification: %s", res.Reservation.AuthorDiscordID, err)
		return
	}

	if err := a.commService.NotifyUpcomingReservation(g, member, res.Spot.Name, res.Reservation.StartAt); err != nil {
		a.log.Errorf("could not send DM to %s: %s", member.Username, err)
		return
	}

	if err := a.reservationRepo.UpdateReservationStartsNotificationSent(context.Background(), res.Reservation.ID); err != nil {
		a.log.Errorf("could not update reservation %d notification sent status: %s", res.Reservation.ID, err)
	}
}
