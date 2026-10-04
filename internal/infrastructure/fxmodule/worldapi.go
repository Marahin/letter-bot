package fxmodule

import (
	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/worldapi"
)

// WorldAPI provides the TibiaData client. An empty base URL keeps it unconfigured.
var WorldAPI = fx.Provide(newWorldAPI)

func newWorldAPI(cfg worldapi.Config) *worldapi.HttpWorldService {
	return worldapi.NewHttpWorldService(cfg.BaseURL)
}
