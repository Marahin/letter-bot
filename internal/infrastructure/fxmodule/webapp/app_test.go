package webapp

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/fxmodule"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/infrastructure/worldapi"
)

// unreachableDSN is a database nothing listens on. pgxpool connects lazily, so
// a pool over it builds and fails only when used.
const unreachableDSN = "host=127.0.0.1 port=1 user=u password=p dbname=d sslmode=disable connect_timeout=1"

func TestApp_GraphIsComplete(t *testing.T) {
	require.NoError(t, fx.ValidateApp(App()))
}

func lazyPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), unreachableDSN)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// testEnvironment stands in for the modules App adds over wiring: a pool that
// never connects, fixed configs and a silent logger.
func testEnvironment(t *testing.T, cfg web.Config, worldCfg worldapi.Config) fx.Option {
	t.Helper()
	pool := lazyPool(t)
	return fx.Options(
		fx.NopLogger,
		fx.Supply(cfg, worldCfg, postgresql.Specification{}, TimeZone("Europe/Berlin")),
		fx.Provide(
			func() *pgxpool.Pool { return pool },
			zap.NewNop,
			(*zap.Logger).Sugar,
		),
	)
}

func testWebConfig() web.Config {
	return web.Config{Addr: "127.0.0.1:0", MetricsAddr: "127.0.0.1:0", BaseURL: "http://localhost:8080"}
}

func TestWiring_Builds(t *testing.T) {
	// when
	app := fx.New(testEnvironment(t, testWebConfig(), worldapi.Config{}), wiring())

	// then
	require.NoError(t, app.Err())
}

func TestWiring_StartsAndStops(t *testing.T) {
	// given
	app := fx.New(testEnvironment(t, testWebConfig(), worldapi.Config{}), wiring())
	require.NoError(t, app.Err())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// when
	require.NoError(t, app.Start(ctx))

	// then
	assert.NoError(t, app.Stop(ctx))
}

// hooks records the lifecycle hooks a provider appends.
type hooks struct{ appended []fx.Hook }

func (h *hooks) Append(hook fx.Hook) { h.appended = append(h.appended, hook) }

func experienceParams(t *testing.T, lc fx.Lifecycle, cfg web.Config, worldCfg worldapi.Config) experienceJobParams {
	t.Helper()
	return experienceJobParams{
		Cfg:       cfg,
		WorldAPI:  worldCfg,
		API:       worldapi.NewHTTPWorldService(worldCfg.BaseURL),
		Pool:      lazyPool(t),
		Lifecycle: lc,
		Log:       zap.NewNop().Sugar(),
	}
}

func TestRunExperienceJob_DisabledAddsNoHook(t *testing.T) {
	// given
	lc := &hooks{}
	cfg := web.Config{ExperienceJobEnabled: false, ExperienceJobInterval: time.Minute}

	// when
	runExperienceJob(experienceParams(t, lc, cfg, worldapi.Config{BaseURL: "http://127.0.0.1:1"}))

	// then
	assert.Empty(t, lc.appended)
}

func TestRunExperienceJob_NoWorldAPIAddsNoHook(t *testing.T) {
	// given
	lc := &hooks{}
	cfg := web.Config{ExperienceJobEnabled: true, ExperienceJobInterval: time.Minute}

	// when
	runExperienceJob(experienceParams(t, lc, cfg, worldapi.Config{}))

	// then
	assert.Empty(t, lc.appended)
}

func TestRunExperienceJob_EnabledRunsUntilStop(t *testing.T) {
	// given
	lc := &hooks{}
	cfg := web.Config{ExperienceJobEnabled: true, ExperienceJobInterval: time.Minute}
	runExperienceJob(experienceParams(t, lc, cfg, worldapi.Config{BaseURL: "http://127.0.0.1:1"}))
	require.Len(t, lc.appended, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// when
	require.NoError(t, lc.appended[0].OnStart(ctx))
	err := lc.appended[0].OnStop(ctx)

	// then
	assert.NoError(t, err)
}

func TestServe_FailsTheStartOnABusyPort(t *testing.T) {
	// given
	var server *web.Server
	app := fx.New(testEnvironment(t, testWebConfig(), worldapi.Config{}), wiring(), fx.Populate(&server))
	require.NoError(t, app.Err())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, app.Start(ctx))
	defer func() { _ = app.Stop(ctx) }()
	busy := testWebConfig()
	busy.Addr = server.Addr()
	second := fx.New(testEnvironment(t, busy, worldapi.Config{}), wiring())

	// when
	err := second.Start(ctx)

	// then
	assert.ErrorContains(t, err, "web server listen")
}

func TestLoadTimeZone_RequiresTZ(t *testing.T) {
	// given
	t.Setenv("TZ", "")

	// when
	_, err := loadTimeZone()

	// then
	assert.ErrorContains(t, err, "TZ")
}

func TestLoadTimeZone_ReadsTZ(t *testing.T) {
	// given
	t.Setenv("TZ", "Europe/Warsaw")

	// when
	tz, err := loadTimeZone()

	// then
	require.NoError(t, err)
	assert.Equal(t, TimeZone("Europe/Warsaw"), tz)
}

func TestMetricsAddr_IsTheWebMetricsAddr(t *testing.T) {
	assert.Equal(t, fxmodule.MetricsAddr(":3005"), metricsAddr(web.Config{MetricsAddr: ":3005"}))
}

func TestHealthChecks_ReadyFailsOnAnUnreachableDatabase(t *testing.T) {
	// when
	checks := healthChecks(lazyPool(t))

	// then
	assert.Nil(t, checks.Live)
	assert.Error(t, checks.Ready())
}
