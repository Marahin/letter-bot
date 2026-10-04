package summary

import (
	"spot-assistant/internal/ports"
)

type Adapter struct {
	service     ports.ChartAdapter
	onlineCheck ports.OnlineCheckService
}

func NewAdapter(srv ports.ChartAdapter, onlineCheck ports.OnlineCheckService) *Adapter {
	return &Adapter{
		service:     srv,
		onlineCheck: onlineCheck,
	}
}
