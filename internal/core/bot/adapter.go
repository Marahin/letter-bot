package bot

import (
	"github.com/sirupsen/logrus"
	"spot-assistant/internal/ports"
)

type Adapter struct {
	service ports.BotPort
	log     *logrus.Entry
}

func NewAdapter(srv ports.BotPort) *Adapter {
	return &Adapter{
		service: srv,
		log: logrus.WithFields(logrus.Fields{
			"type": "core",
			"name": "bot",
		}),
	}
}
