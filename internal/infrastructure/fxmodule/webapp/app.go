// Package webapp composes the web binary.
package webapp

import (
	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/fxmodule"
	"spot-assistant/internal/infrastructure/web"
)

// App is the whole web app. cmd/web runs it.
func App() fx.Option {
	return fx.Options(
		fx.Supply(fxmodule.Binary("web")),
		fxmodule.Logger,
		fxmodule.Config,
		fx.Provide(fxmodule.Load("web", web.LoadConfig), loadTimeZone),
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
			newDiscordOAuth,
			newOAuthPort,
			newNotifier,
			newStatsRepository,
			newBooking,
			newAuthService,
			newAccessService,
			newPremiumService,
			newSettingsService,
			newSpotService,
			newReservationService,
			newStatsService,
			newCharacterService,
			newServer,
			metricsAddr,
			healthChecks,
		),
		// Metrics is invoked first, so it stops after the job and serve hooks.
		fxmodule.Metrics,
		fx.Invoke(runExperienceJob, serve),
	)
}
