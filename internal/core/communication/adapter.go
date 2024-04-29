package communication

import (
	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	log       *zap.SugaredLogger
	bot       ports.BotPort
	formatter ports.TextFormatter
}

func NewAdapter(bot ports.BotPort, formatter ports.TextFormatter) *Adapter {
	return &Adapter{
		bot:       bot,
		formatter: formatter,
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "communicationService")
	return a
}
