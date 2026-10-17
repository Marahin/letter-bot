package summary

import (
	"time"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	service     ports.ChartAdapter
	onlineCheck ports.OnlineCheckService
	now         func() time.Time
}

func NewAdapter(srv ports.ChartAdapter, onlineCheck ports.OnlineCheckService) *Adapter {
	return &Adapter{
		service:     srv,
		onlineCheck: onlineCheck,
		now:         time.Now,
	}
}
