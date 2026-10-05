package fxmodule

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/db/postgresql"
	experiencesqlc "spot-assistant/internal/infrastructure/experience/postgresql/sqlc"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	reservationsqlc "spot-assistant/internal/infrastructure/reservation/postgresql/sqlc"
	spotsqlc "spot-assistant/internal/infrastructure/spot/postgresql/sqlc"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/worldapi"
	worldnamesqlc "spot-assistant/internal/infrastructure/worldname/postgresql/sqlc"
)

// hooks records the lifecycle hooks a provider appends.
type hooks struct{ appended []fx.Hook }

func (h *hooks) Append(hook fx.Hook) { h.appended = append(h.appended, hook) }

// unreachable is a database nothing listens on; every connect fails at once.
var unreachable = postgresql.Specification{Host: "127.0.0.1", Port: 1, User: "u", Password: "p", Name: "d", SSL: "disable"}

func TestLoad_WrapsTheLoaderError(t *testing.T) {
	// given
	boom := errors.New("boom")
	failing := Load("worldapi", func() (worldapi.Config, error) { return worldapi.Config{}, boom })
	passing := Load("worldapi", func() (worldapi.Config, error) { return worldapi.Config{BaseURL: "x"}, nil })

	// when
	_, err := failing()
	cfg, passErr := passing()

	// then
	require.ErrorIs(t, err, boom)
	assert.ErrorContains(t, err, "load worldapi config")
	require.NoError(t, passErr)
	assert.Equal(t, "x", cfg.BaseURL)
}

func TestNewLogger_SyncsOnStop(t *testing.T) {
	// given
	lc := &hooks{}

	// when
	logger, err := newLogger(lc)

	// then
	require.NoError(t, err)
	require.NotNil(t, logger)
	require.Len(t, lc.appended, 1)
	assert.NotNil(t, lc.appended[0].OnStop)
	_ = lc.appended[0].OnStop(context.Background())
	assert.NotNil(t, newEventLogger(zap.NewNop()))
}

func TestLogger_StartsTheApp(t *testing.T) {
	// given
	app := fx.New(fx.Supply(Binary("test")), Logger)
	require.NoError(t, app.Err())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// when
	require.NoError(t, app.Start(ctx))

	// then
	assert.NoError(t, app.Stop(ctx))
}

func TestDatabase_FailsFastOnAnUnreachableDatabase(t *testing.T) {
	// given
	lc := &hooks{}

	// when
	_, err := connect(lc, unreachable, Migrated{})

	// then
	assert.ErrorContains(t, err, "db connect failed")
	assert.Empty(t, lc.appended, "a failed pool leaves no close hook")
}

func TestMigrate_FailsFastOnAnUnreachableDatabase(t *testing.T) {
	// when
	_, err := migrate(unreachable, zap.NewNop().Sugar())

	// then
	assert.ErrorContains(t, err, "migrations failed")
}

func TestDatabase_RejectsABadDSN(t *testing.T) {
	// given
	lc := &hooks{}
	bad := unreachable
	bad.Port = -1
	bad.SSL = "nonsense"

	// when
	_, err := connect(lc, bad, Migrated{})

	// then
	assert.ErrorContains(t, err, "db config")
}

func TestMetrics_ServesForTheAppLifetime(t *testing.T) {
	// given
	app := fx.New(
		fx.NopLogger,
		fx.Supply(MetricsAddr("127.0.0.1:0"), HealthChecks{}),
		fx.Provide(zap.NewNop, (*zap.Logger).Sugar),
		Registry,
		Metrics,
	)
	require.NoError(t, app.Err())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// when
	require.NoError(t, app.Start(ctx))

	// then
	assert.NoError(t, app.Stop(ctx))
}

func TestMetrics_FailsTheStartOnABadAddress(t *testing.T) {
	// given
	app := fx.New(
		fx.NopLogger,
		fx.Supply(MetricsAddr("127.0.0.1:-1"), HealthChecks{}, prometheus.NewRegistry()),
		fx.Provide(zap.NewNop, (*zap.Logger).Sugar),
		Metrics,
	)
	require.NoError(t, app.Err())

	// when
	err := app.Start(context.Background())

	// then
	assert.Error(t, err)
}

func TestRepositories_BuildOverAPool(t *testing.T) {
	// given
	pool, err := pgxpool.New(context.Background(), unreachable.DSN())
	require.NoError(t, err)
	defer pool.Close()
	var reservations *reservationsqlc.ReservationRepository

	// when
	app := fx.New(
		fx.NopLogger,
		fx.Supply(pool, worldapi.Config{}),
		fx.Provide(zap.NewNop, (*zap.Logger).Sugar),
		Repositories,
		WorldAPI,
		fx.Populate(&reservations),
		fx.Invoke(func(
			*guildsqlc.GuildConfigRepository, *guildsqlc.GuildChannelRepository, *guildsqlc.GuildRoleRepository,
			*worldnamesqlc.WorldNameRepository, *spotsqlc.SpotRepository, *webusersqlc.WebUserRepository,
			*experiencesqlc.ExperienceRepository, *worldapi.HTTPWorldService,
		) {
		}),
	)

	// then
	require.NoError(t, app.Err())
	assert.NotNil(t, reservations)
}

func TestSharedModules_ValidateTogether(t *testing.T) {
	err := fx.ValidateApp(
		fx.Supply(Binary("test")),
		Logger,
		Config,
		Database,
		Repositories,
		WorldAPI,
		Registry,
		fx.Invoke(func(*pgxpool.Pool, *worldapi.HTTPWorldService, *prometheus.Registry) {}),
	)

	require.NoError(t, err)
}

func TestLoopHook_StopWaitsForTheLoop(t *testing.T) {
	// given
	stopped := false
	hook := LoopHook(func(ctx context.Context) {
		<-ctx.Done()
		stopped = true
	})
	require.NoError(t, hook.OnStart(context.Background()))

	// when
	err := hook.OnStop(context.Background())

	// then
	require.NoError(t, err)
	assert.True(t, stopped)
}

func TestLoopHook_StopGivesUpWhenItsContextEnds(t *testing.T) {
	// given
	release := make(chan struct{})
	defer close(release)
	hook := LoopHook(func(context.Context) { <-release })
	require.NoError(t, hook.OnStart(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when
	err := hook.OnStop(ctx)

	// then
	assert.True(t, errors.Is(err, context.Canceled))
}
