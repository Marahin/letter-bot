package onlinecheck

import (
	"go.uber.org/zap"

	cmap "github.com/orcaman/concurrent-map/v2"

	"spot-assistant/internal/ports"
)

type Adapter struct {
	log            *zap.SugaredLogger
	api            ports.WorldApi
	worldNameRepo  ports.WorldNameRepository
	guildIdToWorld cmap.ConcurrentMap[string, string]
	players        cmap.ConcurrentMap[string, []string]
}

func NewAdapter(api ports.WorldApi, worldNameRepo ports.WorldNameRepository) *Adapter {
	return &Adapter{
		api:            api,
		worldNameRepo:  worldNameRepo,
		guildIdToWorld: cmap.New[string](),
		players:        cmap.New[[]string](),
	}
}

func (a *Adapter) WithLogger(log *zap.SugaredLogger) *Adapter {
	a.log = log.With("layer", "core", "name", "onlineCheckService")
	return a
}

func (a *Adapter) IsConfigured() bool {
	return a.api != nil && a.api.GetBaseURL() != ""
}
