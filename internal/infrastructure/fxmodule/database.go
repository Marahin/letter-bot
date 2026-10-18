package fxmodule

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"spot-assistant/internal/infrastructure/db/postgresql"
)

// pingTimeout bounds the first ping, so an unreachable database fails the start.
const pingTimeout = 30 * time.Second

// Database runs the migrations, then provides the pool.
var Database = fx.Provide(migrate, connect)

// Migrated marks a database whose migrations ran. Take it to order a provider
// after the migrations.
type Migrated struct{}

func migrate(cfg postgresql.Specification, log *zap.SugaredLogger) (Migrated, error) {
	if _, err := postgresql.Migrate(context.Background(), cfg.DSN(), log); err != nil {
		return Migrated{}, fmt.Errorf("migrations failed: %w", err)
	}
	return Migrated{}, nil
}

func connect(lc fx.Lifecycle, cfg postgresql.Specification, _ Migrated) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("db config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db connect failed: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db connect failed: %w", err)
	}
	lc.Append(fx.StopHook(pool.Close))
	return pool, nil
}
