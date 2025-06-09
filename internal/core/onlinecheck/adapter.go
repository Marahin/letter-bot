package onlinecheck

import (
	"sync"

	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	log     *zap.SugaredLogger
	api     ports.WorldApi
	world   string
	players []string
	mutex   sync.RWMutex
}

func NewAdapter(api ports.WorldApi, world string) *Adapter {
	return &Adapter{
		api:   api,
		world: world,
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "onlineCheckService")
	return a
}

func (a *Adapter) IsConfigured() bool {
	return a.api != nil && a.world != "" && a.api.GetBaseURL() != ""
}
