// Package botapp composes the bot binary. It is the only package that imports
// internal/infrastructure/bot.
package botapp

import (
	"time"

	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/infrastructure/chart"
	"spot-assistant/internal/infrastructure/fxmodule"
)

// startTimeout bounds OnStart, which opens the gateway connections.
const startTimeout = time.Minute

// App is the whole bot app. cmd/bot runs it.
func App() fx.Option {
	return fx.Options(
		fx.StartTimeout(startTimeout),
		fx.Supply(fxmodule.Binary("bot")),
		fxmodule.Logger,
		fxmodule.Config,
		fx.Provide(fxmodule.Load("bot", bot.LoadConfig)),
		fxmodule.Database,
		wiring(),
	)
}

// wiring holds every provider and invoke that needs no environment.
func wiring() fx.Option {
	return fx.Options(
		fxmodule.Repositories,
		fxmodule.WorldAPI,
		fxmodule.Registry,
		fx.Provide(
			newShards,
			newOnlineChecker,
			chart.NewAdapter,
			newSummary,
			formatter.NewFormatter,
			newBot,
			newCommunication,
			newBooking,
			newEventHandler,
			newLocalNotifier,
			newReservationService,
			newReservationForms,
			newMetrics,
			newNotifyHandler,
			newListener,
			metricsAddr,
			healthChecks,
		),
		// Metrics is invoked first, so it stops after the run hook.
		fxmodule.Metrics,
		fx.Invoke(wireBot, run),
	)
}
