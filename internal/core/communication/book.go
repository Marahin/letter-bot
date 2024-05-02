package communication

import (
	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/reservation"
)

// NotifyOverbookedMember gets a member from the repository,
// and sends a DM to the member about overbook.
func (a *Adapter) NotifyOverbookedMember(
	request book.BookRequest,
	res *reservation.ClippedOrRemovedReservation,
) {
	member, err := a.memberRepo.GetMemberByGuildAndId(request.Guild, res.Original.AuthorDiscordID)
	if err != nil {
		a.log.Error("something went wrong when fetching member to notify about overbooking: ", err)
		return
	}

	err = a.bot.SendDMOverbookedNotification(member, request, res)
	if err != nil {
		a.log.Errorf("error sending DM: %s", err)
	}
}
