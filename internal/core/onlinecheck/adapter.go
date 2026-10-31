package onlinecheck

import (
	"go.uber.org/zap"

	cmap "github.com/orcaman/concurrent-map/v2"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	log            *zap.SugaredLogger
	api            ports.WorldAPI
	worldNameRepo  ports.WorldNameRepository
	guildIDToWorld cmap.ConcurrentMap[string, string]
	players        cmap.ConcurrentMap[string, map[string]struct{}]
}

func NewAdapter(api ports.WorldAPI, worldNameRepo ports.WorldNameRepository) *Adapter {
	return &Adapter{
		api:            api,
		worldNameRepo:  worldNameRepo,
		guildIDToWorld: cmap.New[string](),
		players:        cmap.New[map[string]struct{}](),
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "onlineCheckService")
	return a
}

func (a *Adapter) IsConfigured() bool {
	return a.api != nil && a.api.GetBaseURL() != ""
}
