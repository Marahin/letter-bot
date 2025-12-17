package communication

import (
	"spot-assistant/internal/core/dto/member"

	"time"
)

func (a *Adapter) NotifyUpcomingReservation(member *member.Member, spotName string, startAt time.Time) error {
	return a.bot.SendDMUpcomingReservationNotification(member, spotName, startAt)
}
