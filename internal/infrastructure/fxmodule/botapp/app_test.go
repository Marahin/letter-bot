package botapp

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/servusdei2018/shards/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/bot"
	"spot-assistant/internal/infrastructure/db/postgresql"
	"spot-assistant/internal/infrastructure/fxmodule"
	"spot-assistant/internal/infrastructure/worldapi"
)

func TestApp_GraphIsComplete(t *testing.T) {
	require.NoError(t, fx.ValidateApp(App()))
}

// testEnvironment stands in for the modules App adds over wiring: a pool that
// never connects, fixed configs, a silent logger and a shard manager that never
// dials Discord.
func testEnvironment(t *testing.T) fx.Option {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), "host=127.0.0.1 port=1 user=u password=p dbname=d sslmode=disable")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	session, err := discordgo.New("Bot x")
	require.NoError(t, err)
	return fx.Options(
		fx.NopLogger,
		fx.Supply(bot.Config{Token: "x", MetricsAddr: "127.0.0.1:0"}, worldapi.Config{}, postgresql.Specification{}),
		fx.Provide(
			func() *pgxpool.Pool { return pool },
			zap.NewNop,
			(*zap.Logger).Sugar,
		),
		fx.Replace(&shards.Manager{Gateway: session}),
	)
}

func TestWiring_Builds(t *testing.T) {
	// when
	app := fx.New(testEnvironment(t), wiring())

	// then
	require.NoError(t, app.Err())
}

type fakeBot struct {
	startErr  error
	started   bool
	shutdowns int
}

func (b *fakeBot) Start() error {
	b.started = true
	return b.startErr
}

func (b *fakeBot) Shutdown() error {
	b.shutdowns++
	return errors.New("already closed")
}

func TestBotHooks_StartFailureStopsTheListener(t *testing.T) {
	// given
	b := &fakeBot{startErr: errors.New("gateway refused")}
	stopped := false
	hook := botHooks(b, func(ctx context.Context) {
		<-ctx.Done()
		stopped = true
	}, zap.NewNop().Sugar())

	// when
	err := hook.OnStart(context.Background())

	// then
	require.ErrorContains(t, err, "bot start failed")
	assert.True(t, stopped, "a failed start waits for the listener to stop")
}

func TestBotHooks_StopEndsTheListenerBeforeTheShutdown(t *testing.T) {
	// given
	b := &fakeBot{}
	shutdownsSeenByListener := -1
	hook := botHooks(b, func(ctx context.Context) {
		<-ctx.Done()
		shutdownsSeenByListener = b.shutdowns
	}, zap.NewNop().Sugar())
	require.NoError(t, hook.OnStart(context.Background()))

	// when
	err := hook.OnStop(context.Background())

	// then
	assert.NoError(t, err, "a failed shutdown is logged, not returned")
	assert.True(t, b.started)
	assert.Equal(t, 1, b.shutdowns)
	assert.Equal(t, 0, shutdownsSeenByListener, "the listener ends before the bot shuts down")
}

func TestBotHooks_StopShutsDownWhenTheListenerDoesNotEnd(t *testing.T) {
	// given
	b := &fakeBot{}
	release := make(chan struct{})
	defer close(release)
	hook := botHooks(b, func(context.Context) { <-release }, zap.NewNop().Sugar())
	require.NoError(t, hook.OnStart(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// when
	err := hook.OnStop(ctx)

	// then
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, 1, b.shutdowns)
}

func TestMetricsAddr_IsTheBotMetricsAddr(t *testing.T) {
	assert.Equal(t, fxmodule.MetricsAddr(":2112"), metricsAddr(bot.Config{MetricsAddr: ":2112"}))
}

func TestHealthChecks_LiveFailsBeforeTheStart(t *testing.T) {
	// given
	var checks fxmodule.HealthChecks
	app := fx.New(testEnvironment(t), wiring(), fx.Populate(&checks))
	require.NoError(t, app.Err())

	// when
	err := checks.Live()

	// then
	assert.ErrorContains(t, err, "bot not running")
}
