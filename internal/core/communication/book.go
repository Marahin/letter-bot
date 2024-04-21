package communication

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
)

func (a *Adapter) NotifyOverbookedMember(
	member *discord.Member,
	request book.BookRequest,
	res *reservation.ClippedOrRemovedReservation,
) {
	err := a.bot.SendDM(member, a.formatter.FormatOverbookedMemberNotification(member, request, res))
	if err != nil {
		a.log.Errorf("error sending DM: %s", err)
	}
}
