package scheduler

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// PgAdvisoryLock is a session-level Postgres advisory lock held on a dedicated pooled connection.
type PgAdvisoryLock struct {
	pool *pgxpool.Pool
	key  int64
	log  *zap.SugaredLogger
}

func NewPgAdvisoryLock(pool *pgxpool.Pool, key int64, log *zap.SugaredLogger) *PgAdvisoryLock {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &PgAdvisoryLock{pool: pool, key: key, log: log}
}

func (l *PgAdvisoryLock) TryLock(ctx context.Context) (func(), bool, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	var ok bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", l.key).Scan(&ok); err != nil {
		conn.Release()
		return nil, false, err
	}
	if !ok {
		conn.Release()
		return nil, false, nil
	}

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", l.key); err != nil {
			// The lock belongs to the session: close it rather than return a locked connection to the pool.
			l.log.Warnw("advisory unlock failed, closing the connection", "error", err)
			_ = conn.Hijack().Close(ctx)
			return
		}
		conn.Release()
	}, true, nil
}
