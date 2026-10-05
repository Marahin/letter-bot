package postgresql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"fmt"
	"io/fs"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"
	"go.uber.org/zap"
)

//go:embed migrations/*.sql
var embedded embed.FS

// migrationLockKey serialises the migration runs of the bot and the web binaries.
// The experience job uses 7419001.
const migrationLockKey int64 = 7419000

const (
	connectTimeout    = 30 * time.Second
	migrationLockWait = 10 * time.Minute
	unlockTimeout     = 5 * time.Second
)

// MigrateResult lists the versions that a Migrate call wrote to goose_db_version.
type MigrateResult struct {
	// Adopted are versions that atlas applied, recorded without a run.
	Adopted []int64
	// Applied are versions that this call ran.
	Applied []int64
}

func migrations() (fs.FS, error) {
	return fs.Sub(embedded, "migrations")
}

// Migrate applies the pending migrations. More than one process can call it at the same
// time: an advisory lock lets one run while the others wait, and they then find nothing
// pending.
func Migrate(ctx context.Context, dsn string, log *zap.SugaredLogger) (MigrateResult, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("open db: %w", err)
	}
	defer db.Close()
	return run(ctx, db, migrationLockWait, log)
}

func run(ctx context.Context, db *sql.DB, lockWait time.Duration, log *zap.SugaredLogger) (MigrateResult, error) {
	provider, err := newProvider(db)
	if err != nil {
		return MigrateResult{}, err
	}

	connectCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	conn, err := db.Conn(connectCtx)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("connect: %w", err)
	}
	if err := conn.PingContext(connectCtx); err != nil {
		discard(conn)
		return MigrateResult{}, fmt.Errorf("connect: %w", err)
	}

	if err := lock(ctx, conn, lockWait, log); err != nil {
		discard(conn)
		return MigrateResult{}, err
	}
	defer unlock(conn, log)

	local := make([]int64, 0, len(provider.ListSources()))
	for _, s := range provider.ListSources() {
		local = append(local, s.Version)
	}
	adopted, err := adopt(ctx, db, local, log)
	if err != nil {
		return MigrateResult{}, err
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return MigrateResult{}, fmt.Errorf("run migrations: %w", err)
	}
	applied := make([]int64, 0, len(results))
	for _, r := range results {
		applied = append(applied, r.Source.Version)
		log.Infow("migration applied", "version", r.Source.Version, "file", r.Source.Path, "duration", r.Duration)
	}
	if len(results) == 0 {
		version, err := provider.GetDBVersion(ctx)
		if err != nil {
			return MigrateResult{}, fmt.Errorf("read the database version: %w", err)
		}
		log.Infow("database schema is up to date", "version", version)
	}
	return MigrateResult{Adopted: adopted, Applied: applied}, nil
}

func newProvider(db *sql.DB) (*goose.Provider, error) {
	fsys, err := migrations()
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return provider, nil
}

func lock(ctx context.Context, conn *sql.Conn, wait time.Duration, log *zap.SugaredLogger) error {
	var ok bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&ok); err != nil {
		return fmt.Errorf("take the migration lock: %w", err)
	}
	if ok {
		return nil
	}
	log.Infow("waiting for the migration lock", "key", migrationLockKey, "timeout", wait)
	waitCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	if _, err := conn.ExecContext(waitCtx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("wait for the migration lock: %w", err)
	}
	return nil
}

func unlock(conn *sql.Conn, log *zap.SugaredLogger) {
	ctx, cancel := context.WithTimeout(context.Background(), unlockTimeout)
	defer cancel()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey); err != nil {
		log.Warnw("migration unlock failed, closing the connection", "error", err)
		discard(conn)
		return
	}
	_ = conn.Close()
}

// discard closes the session instead of returning it to the pool, so a lock that it
// may hold goes with it.
func discard(conn *sql.Conn) {
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = conn.Close()
}
