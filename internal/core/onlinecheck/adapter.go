package onlinecheck

import (
	"sync"

	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	log     *zap.SugaredLogger
	api     ports.WorldApi
	worlds  map[string]string
	players map[string][]string
	mutex   sync.RWMutex
}

func NewAdapter(api ports.WorldApi, world string) *Adapter {
	return &Adapter{
		api:     api,
		worlds:  make(map[string]string),
		players: make(map[string][]string),
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "onlineCheckService")
	return a
}

func (a *Adapter) IsConfigured() bool {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	if a.api == nil || a.api.GetBaseURL() == "" {
		return false
	}
	// At least one world must be configured
	for _, world := range a.worlds {
		if world != "" {
			return true
		}
	}
	return false
}
