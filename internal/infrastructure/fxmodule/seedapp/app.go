//go:build devauth

package seedapp

import (
	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/fxmodule"
)

// App migrates the database, writes the dev seed and stops. cmd/seed runs it.
func App() fx.Option {
	return fx.Options(
		fx.Supply(fxmodule.Binary("seed")),
		fxmodule.Logger,
		fxmodule.Config,
		fxmodule.Database,
		fxmodule.Repositories,
		fx.Invoke(run),
	)
}
