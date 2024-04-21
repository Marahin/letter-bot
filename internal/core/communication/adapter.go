package communication

import (
	"github.com/sirupsen/logrus"

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
