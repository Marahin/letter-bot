package communication

import (
	"github.com/sirupsen/logrus"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/ports"
)

type Adapter struct {
	log       *logrus.Entry
	bot       ports.BotPort
	formatter ports.TextFormatter
}

func NewAdapter(bot ports.BotPort) *Adapter {
	return &Adapter{
		bot: bot,
		log: logrus.WithFields(logrus.Fields{"type": "core", "name": "communication"}),
	}
}

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
