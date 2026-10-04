package botapp

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/servusdei2018/shards/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/communication"
	"spot-assistant/internal/core/onlinecheck"
	"spot-assistant/internal/core/summary"
	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/bot/formatter"
	"spot-assistant/internal/infrastructure/chart"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/eventhandler"
	"spot-assistant/internal/infrastructure/fxmodule"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	healthadapter "spot-assistant/internal/infrastructure/health"
	prommetrics "spot-assistant/internal/infrastructure/metrics/prometheus"
	notifypg "spot-assistant/internal/infrastructure/notify/postgresql"
	reservationsqlc "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	spotsqlc "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/worldapi"
	worldnamesqlc "spot-assistant/internal/infrastructure/worldname/postgresql/sqlc"
)

// newShards asks Discord for the gateway at construction, with retries.
func newShards(cfg bot.Config) (*shards.Manager, error) {
	return bot.ConnectShards(cfg.Token)
}

func newOnlineChecker(api *worldapi.HTTPWorldService, worlds *worldnamesqlc.WorldNameRepository, log *zap.SugaredLogger) *onlinecheck.Adapter {
	checker := onlinecheck.NewAdapter(api, worlds).WithLogger(log)
	if !checker.IsConfigured() {
		log.Warn("Online checker is disabled: TIBIA_WORLD_API_BASE_URL not set")
	}
	return checker
}

func newSummary(charter *chart.Adapter, online *onlinecheck.Adapter) *summary.Adapter {
	return summary.NewAdapter(charter, online)
}

type botParams struct {
	fx.In

	Shards       *shards.Manager
	Cfg          bot.Config
	Summary      *summary.Adapter
	Reservations *reservationsqlc.ReservationRepository
	Online       *onlinecheck.Adapter
	Configs      *guildsqlc.GuildConfigRepository
	Channels     *guildsqlc.GuildChannelRepository
	Roles        *guildsqlc.GuildRoleRepository
	Formatter    *formatter.DiscordFormatter
	Log          *zap.SugaredLogger
}

func newBot(p botParams) *bot.Bot {
	return bot.NewManager(p.Shards, p.Cfg.WebBaseURL, p.Summary, p.Reservations, p.Online).
		WithGuildRepositories(p.Configs, p.Channels, p.Roles).
		WithFormatter(p.Formatter).
		WithLogger(p.Log)
}

func newCommunication(b *bot.Bot, log *zap.SugaredLogger) *communication.Adapter {
	return communication.NewAdapter(b, b).WithLogger(log)
}

func newBooking(spots *spotsqlc.SpotRepository, reservations *reservationsqlc.ReservationRepository, comm *communication.Adapter, log *zap.SugaredLogger) *booking.Adapter {
	return booking.NewAdapter(spots, reservations, comm).WithLogger(log)
}

func newEventHandler(booker *booking.Adapter, reservations *reservationsqlc.ReservationRepository, comm *communication.Adapter, summaries *summary.Adapter) *eventhandler.Handler {
	return eventhandler.NewHandler(booker, reservations, comm, summaries)
}

func newMetrics(reg *prometheus.Registry) (*prommetrics.PromMetrics, error) {
	return prommetrics.New(reg)
}

func newNotifyHandler(b *bot.Bot, comm *communication.Adapter, log *zap.SugaredLogger) *bot.NotifyHandler {
	return bot.NewNotifyHandler(b, comm).WithLogger(log)
}

// newListener receives the web's NOTIFY signals on its own connection.
func newListener(cfg postgresql.Specification, log *zap.SugaredLogger) *notifypg.Listener {
	return notifypg.NewListener(cfg.DSN(), log)
}

func metricsAddr(cfg bot.Config) fxmodule.MetricsAddr {
	return fxmodule.MetricsAddr(cfg.MetricsAddr)
}

// healthChecks fails /livez on a stopped bot or a stale gateway; /readyz also pings the database.
func healthChecks(pool *pgxpool.Pool, b *bot.Bot, log *zap.SugaredLogger) fxmodule.HealthChecks {
	health := healthadapter.NewAdapter(pool, b).WithLogger(log)
	return fxmodule.HealthChecks{Live: health.Live, Ready: health.Ready}
}
