package fxmodule

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"

	"spot-assistant/internal/infrastructure/db/postgresql"
)

// pingTimeout bounds the first ping, so an unreachable database fails the start.
const pingTimeout = 30 * time.Second

// Database provides the pool. The migrations are applied by atlas, not here.
var Database = fx.Provide(connect)

func connect(lc fx.Lifecycle, cfg postgresql.Specification) (*pgxpool.Pool, error) {
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
