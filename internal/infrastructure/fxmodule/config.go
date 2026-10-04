package fxmodule

import (
	"fmt"

	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/worldapi"
)

// Config provides the environment config loaders both binaries share. The
// binary's own config is loaded in its app, so this package never imports the
// bot adapter.
var Config = fx.Provide(
	Load("database", postgresql.LoadConfig),
	Load("worldapi", worldapi.LoadConfig),
)

// Load wraps a config loader so its error names the config. fx fails the start on the error.
func Load[T any](name string, loader func() (T, error)) func() (T, error) {
	return func() (T, error) {
		cfg, err := loader()
		if err != nil {
			return cfg, fmt.Errorf("load %s config: %w", name, err)
		}
		return cfg, nil
	}
}
