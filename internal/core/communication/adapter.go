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

func NewAdapter(bot ports.BotPort, formatter ports.TextFormatter) *Adapter {
	return &Adapter{
		bot:       bot,
		formatter: formatter,
		log:       logrus.WithFields(logrus.Fields{"type": "core", "name": "communication"}),
	}
}
