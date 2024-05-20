package communication

import (
	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	log        *zap.SugaredLogger
	bot        ports.BotPort
	memberRepo ports.MemberRepository
}

func NewAdapter(bot ports.BotPort, memberRepo ports.MemberRepository) *Adapter {
	return &Adapter{
		bot:        bot,
		memberRepo: memberRepo,
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "communicationService")
	return a
}
