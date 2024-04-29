package summary

import (
	"spot-assistant/internal/ports"
)

type Adapter struct {
	service ports.ChartAdapter
	//log     *zap.SugaredLogger
}

func NewAdapter(srv ports.ChartAdapter) *Adapter {
	return &Adapter{
		service: srv,
	}
}

//func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
//	a.log = log.With("layer", "infrastructure", "name", "summaryService")
//	return a
//}
