package communication

import (
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"

	"time"
)

func (a *Adapter) NotifyUpcomingReservation(guild *guild.Guild, member *member.Member, spotName string, startAt time.Time) error {
	return a.bot.SendDMUpcomingReservationNotification(guild, member, spotName, startAt)
}
