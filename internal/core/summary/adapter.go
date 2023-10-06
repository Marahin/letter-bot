package summary

import (
	"github.com/sirupsen/logrus"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	service ports.ChartAdapter
	log     *logrus.Entry
}

func NewAdapter(srv ports.ChartAdapter) *Adapter {
	return &Adapter{
		service: srv,
		log: logrus.WithFields(logrus.Fields{
			"type": "core",
			"name": "summary",
		}),
	}
}
