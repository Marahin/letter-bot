package webapp

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/core/auth"
	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/characters"
	"spot-assistant/internal/core/guildaccess"
	"spot-assistant/internal/core/premium"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/core/spots"
	corestats "spot-assistant/internal/core/stats"
	adminhttp "spot-assistant/internal/infrastructure/admin-http"
	channelshttp "spot-assistant/internal/infrastructure/channels-http"
	"spot-assistant/internal/infrastructure/discord/oauth"
	experiencesqlc "spot-assistant/internal/infrastructure/experience/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/fxmodule"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/guildsettings"
	notifypg "spot-assistant/internal/infrastructure/notify/postgresql"
	"spot-assistant/internal/infrastructure/notify/webcomm"
	reservationsqlc "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	reservationshttp "spot-assistant/internal/infrastructure/reservations-http"
	settingshttp "spot-assistant/internal/infrastructure/settings-http"
	spotsqlc "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
	spotshttp "spot-assistant/internal/infrastructure/spots-http"
	statshttp "spot-assistant/internal/infrastructure/stats-http"
	statssqlc "spot-assistant/internal/infrastructure/stats/postgresql/sqlc"
	toolshttp "spot-assistant/internal/infrastructure/tools-http"
	"spot-assistant/internal/infrastructure/web"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/worldapi"
	worldnamesqlc "spot-assistant/internal/infrastructure/worldname/postgresql/sqlc"
)

// readyTimeout bounds the database ping of /readyz.
const readyTimeout = 2 * time.Second

// TimeZone is the IANA zone the stats bucket days in, read from TZ.
type TimeZone string

func loadTimeZone() (TimeZone, error) {
	tz := os.Getenv("TZ")
	if tz == "" {
		return "", errors.New("TZ must be set")
	}
	return TimeZone(tz), nil
}

// newDiscordOAuth caches the rate-limited user-guilds endpoint. The web user
// repository stores the tokens.
func newDiscordOAuth(cfg web.Config, users *webusersqlc.WebUserRepository, log *zap.SugaredLogger) *oauth.Caching {
	return oauth.NewCaching(oauth.New(cfg.Discord.ClientID, cfg.Discord.ClientSecret, cfg.CallbackURL(), users), log)
}

func newNotifier(pool *pgxpool.Pool) *notifypg.Notifier {
	return notifypg.NewNotifier(pool)
}

func newStatsRepository(pool *pgxpool.Pool, tz TimeZone) (*statssqlc.StatsRepository, error) {
	return statssqlc.NewStatsRepository(pool, string(tz))
}

// newBooking books on the web and signals the bot, which posts on Discord.
func newBooking(spotRepo *spotsqlc.SpotRepository, reservationRepo *reservationsqlc.ReservationRepository, notifier *notifypg.Notifier, log *zap.SugaredLogger) *booking.Adapter {
	return booking.NewAdapter(spotRepo, reservationRepo, webcomm.New(notifier, log)).WithLogger(log)
}

func newAuthService(discord *oauth.Caching, users *webusersqlc.WebUserRepository) *auth.Service {
	return auth.New(discord, users)
}

func newAccessService(discord *oauth.Caching, configs *guildsqlc.GuildConfigRepository, roles *guildsqlc.GuildRoleRepository, cfg web.Config, log *zap.SugaredLogger) *guildaccess.Service {
	return guildaccess.New(discord, configs, roles, cfg.AdminDiscordIDs, log)
}

func newPremiumService(configs *guildsqlc.GuildConfigRepository, notifier *notifypg.Notifier, log *zap.SugaredLogger) *premium.Service {
	return premium.New(configs, notifier, log)
}

func newSettingsService(configs *guildsqlc.GuildConfigRepository, channels *guildsqlc.GuildChannelRepository, roles *guildsqlc.GuildRoleRepository, worlds *worldnamesqlc.WorldNameRepository, notifier *notifypg.Notifier, log *zap.SugaredLogger) *guildsettings.Service {
	return guildsettings.New(configs, channels, roles, worlds, notifier, log)
}

func newSpotService(spotRepo *spotsqlc.SpotRepository, notifier *notifypg.Notifier, log *zap.SugaredLogger) *spots.Service {
	return spots.New(spotRepo, notifier, log)
}

func newReservationService(booker *booking.Adapter, reservationRepo *reservationsqlc.ReservationRepository, spotRepo *spotsqlc.SpotRepository, notifier *notifypg.Notifier, log *zap.SugaredLogger) *reservations.Service {
	return reservations.New(booker, reservationRepo, spotRepo, notifier, log)
}

func newStatsService(stats *statssqlc.StatsRepository, spotRepo *spotsqlc.SpotRepository) *corestats.Service {
	return corestats.New(stats, spotRepo)
}

func newCharacterService(api *worldapi.HTTPWorldService, worlds *worldnamesqlc.WorldNameRepository, exp *experiencesqlc.ExperienceRepository, stats *statssqlc.StatsRepository, log *zap.SugaredLogger) *characters.Service {
	return characters.New(worldapi.NewCachedCharacters(api), worlds, exp, stats, log)
}

type serverParams struct {
	fx.In

	Cfg          web.Config
	Log          *zap.SugaredLogger
	Pool         *pgxpool.Pool
	Auth         *auth.Service
	Access       *guildaccess.Service
	Premium      *premium.Service
	Settings     *guildsettings.Service
	Spots        *spots.Service
	Reservations *reservations.Service
	Stats        *corestats.Service
	Characters   *characters.Service
}

// features are the feature packages' route registrars, mounted in this order.
var features = []func(*web.Router, *web.Deps){
	adminhttp.Register,
	settingshttp.Register,
	channelshttp.Register,
	spotshttp.Register,
	reservationshttp.Register,
	statshttp.Register,
	toolshttp.Register,
}

func newServer(p serverParams) *web.Server {
	server := web.NewServer(p.Cfg, p.Log, p.Pool).WithServices(web.Services{
		Auth:         p.Auth,
		Access:       p.Access,
		Premium:      p.Premium,
		Settings:     p.Settings,
		Spots:        p.Spots,
		Reservations: p.Reservations,
		Stats:        p.Stats,
		Characters:   p.Characters,
	})
	for _, register := range features {
		server.Mount(register)
	}
	return server.WithLandingTool(toolshttp.LandingCalculator())
}

func metricsAddr(cfg web.Config) fxmodule.MetricsAddr {
	return fxmodule.MetricsAddr(cfg.MetricsAddr)
}

// healthChecks keeps /livez unconditional: the web holds no gateway. /readyz pings the database.
func healthChecks(pool *pgxpool.Pool) fxmodule.HealthChecks {
	return fxmodule.HealthChecks{
		Ready: func() error {
			ctx, cancel := context.WithTimeout(context.Background(), readyTimeout)
			defer cancel()
			return pool.Ping(ctx)
		},
	}
}
