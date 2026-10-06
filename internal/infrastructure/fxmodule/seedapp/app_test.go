//go:build devauth

package seedapp

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
)

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
// never connects and a silent logger.
func testEnvironment(t *testing.T) fx.Option {
	t.Helper()
	pool := lazyPool(t)
	return fx.Options(
		fx.NopLogger,
		fx.Provide(
			func() *pgxpool.Pool { return pool },
			zap.NewNop,
			(*zap.Logger).Sugar,
		),
	)
}

func TestWiring_Builds(t *testing.T) {
	// when
	app := fx.New(testEnvironment(t), wiring())

	// then
	require.NoError(t, app.Err())
}

type fakeShutdowner struct{ called bool }

func (f *fakeShutdowner) Shutdown(...fx.ShutdownOption) error {
	f.called = true
	return nil
}

func TestRun_FailedSeedFailsTheStart(t *testing.T) {
	// given a pool nothing listens behind
	pool := lazyPool(t)
	lc := fxtest.NewLifecycle(t)
	shutdowner := &fakeShutdowner{}
	run(runParams{
		Lifecycle:  lc,
		Shutdowner: shutdowner,
		Pool:       pool,
		Configs:    guildsqlc.NewGuildConfigRepository(pool),
		Roles:      guildsqlc.NewGuildRoleRepository(pool),
		Log:        zap.NewNop().Sugar(),
	})

	// when
	err := lc.Start(context.Background())

	// then
	require.Error(t, err)
	assert.False(t, shutdowner.called)
}
