package summary

import (
	"spot-assistant/internal/ports"
)

type Adapter struct {
	service     ports.ChartAdapter
	onlineCheck ports.OnlineCheckService
	//log     *zap.SugaredLogger
}

func NewAdapter(srv ports.ChartAdapter, onlineCheck ports.OnlineCheckService) *Adapter {
	return &Adapter{
		service:     srv,
		onlineCheck: onlineCheck,
	}
}

//func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
//	a.log = log.With("layer", "infrastructure", "name", "summaryService")
//	return a
//}
