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

func TestApp_GraphIsComplete(t *testing.T) {
	require.NoError(t, fx.ValidateApp(App()))
}

type fakeShutdowner struct{ called bool }

func (f *fakeShutdowner) Shutdown(...fx.ShutdownOption) error {
	f.called = true
	return nil
}

func TestRun_FailedSeedFailsTheStart(t *testing.T) {
	// given a pool nothing listens behind
	pool, err := pgxpool.New(context.Background(), "host=127.0.0.1 port=1 user=u password=p dbname=d sslmode=disable connect_timeout=1")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
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
	err = lc.Start(context.Background())

	// then
	require.Error(t, err)
	assert.False(t, shutdowner.called)
}
