package postgresql

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scratchDatabase creates an empty database for one test and drops it at the end. Set
// LETTER_TEST_DATABASE_URL to run, e.g.
// postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable.
func scratchDatabase(t *testing.T) string {
	t.Helper()
	base := os.Getenv("LETTER_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("LETTER_TEST_DATABASE_URL not set")
	}
	admin, err := sql.Open("pgx", base)
	require.NoError(t, err)
	name := fmt.Sprintf("letter_it_mig_%d", rand.Uint64())
	_, err = admin.ExecContext(t.Context(), "CREATE DATABASE "+name)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = admin.ExecContext(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", name)
		_, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name)
		assert.NoError(t, err)
		_ = admin.Close()
	})

	u, err := url.Parse(base)
	require.NoError(t, err)
	u.Path = "/" + name
	return u.String()
}

func openDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func gooseVersions(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), "SELECT version_id FROM goose_db_version WHERE version_id > 0 ORDER BY id")
	require.NoError(t, err)
	defer rows.Close()
	var versions []int64
	for rows.Next() {
		var v int64
		require.NoError(t, rows.Scan(&v))
		versions = append(versions, v)
	}
	require.NoError(t, rows.Err())
	return versions
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var found bool
	require.NoError(t, db.QueryRowContext(t.Context(), "SELECT to_regclass($1) IS NOT NULL", table).Scan(&found))
	return found
}

func localVersions(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	provider, err := newProvider(db)
	require.NoError(t, err)
	var versions []int64
	for _, s := range provider.ListSources() {
		versions = append(versions, s.Version)
	}
	return versions
}

func TestMigrate_FreshDatabase(t *testing.T) {
	// given
	dsn := scratchDatabase(t)
	db := openDB(t, dsn)

	// when
	res, err := Migrate(t.Context(), dsn, nopLog)

	// then
	require.NoError(t, err)
	assert.Empty(t, res.Adopted)
	assert.Equal(t, localVersions(t, db), res.Applied)
	assert.Equal(t, res.Applied, gooseVersions(t, db))
	var column bool
	require.NoError(t, db.QueryRowContext(t.Context(), `SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'web_users' AND column_name = 'default_guild_id')`).Scan(&column))
	assert.True(t, column)
}

func TestMigrate_SecondRunIsANoop(t *testing.T) {
	// given
	dsn := scratchDatabase(t)
	_, err := Migrate(t.Context(), dsn, nopLog)
	require.NoError(t, err)

	// when
	res, err := Migrate(t.Context(), dsn, nopLog)

	// then
	require.NoError(t, err)
	assert.Empty(t, res.Adopted)
	assert.Empty(t, res.Applied)
}

func TestMigrate_ConcurrentRunsApplyOnce(t *testing.T) {
	// given
	dsn := scratchDatabase(t)
	db := openDB(t, dsn)
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]MigrateResult, 2)
	errs := make([]error, 2)
	for i := range 2 {
		wg.Go(func() {
			<-start
			results[i], errs[i] = Migrate(context.Background(), dsn, nopLog)
		})
	}

	// when
	close(start)
	wg.Wait()

	// then
	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	local := localVersions(t, db)
	assert.Len(t, local, len(results[0].Applied)+len(results[1].Applied))
	assert.True(t, len(results[0].Applied) == 0 || len(results[1].Applied) == 0, "one run applies everything")
	assert.Equal(t, local, gooseVersions(t, db), "each version is recorded once")
}

// atlasDDL is the layout of the atlas revisions table (ariga.io/atlas, ent schema).
const atlasDDL = `CREATE SCHEMA atlas_schema_revisions;
CREATE TABLE atlas_schema_revisions.atlas_schema_revisions (
	version character varying NOT NULL,
	description character varying NOT NULL,
	type bigint NOT NULL DEFAULT 2,
	applied bigint NOT NULL DEFAULT 0,
	total bigint NOT NULL DEFAULT 0,
	executed_at timestamptz NOT NULL,
	execution_time bigint NOT NULL,
	error text NULL,
	error_stmt text NULL,
	hash character varying NOT NULL,
	partial_hashes jsonb NULL,
	operator_version character varying NOT NULL,
	PRIMARY KEY (version)
)`

func atlasHistory(t *testing.T, db *sql.DB, upTo int64, rows string) {
	t.Helper()
	ctx := t.Context()
	if upTo > 0 {
		provider, err := newProvider(db)
		require.NoError(t, err)
		_, err = provider.UpTo(ctx, upTo)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, "DROP TABLE "+goose.DefaultTablename)
		require.NoError(t, err)
	}
	_, err := db.ExecContext(ctx, atlasDDL)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO atlas_schema_revisions.atlas_schema_revisions
		(version, description, type, applied, total, executed_at, execution_time, error, hash, operator_version) VALUES `+rows)
	require.NoError(t, err)
}

func TestMigrate_AdoptsAtlasBaseline(t *testing.T) {
	// given
	dsn := scratchDatabase(t)
	db := openDB(t, dsn)
	atlasHistory(t, db, 20251211123500, `
		('.atlas_cloud_identifiers', '', 0, 0, 0, now(), 0, NULL, '', 'Atlas CLI v0.21.1'),
		('20240429143026', 'actual_schema', 1, 0, 0, now(), 0, NULL, '', 'Atlas CLI v0.21.1'),
		('20250611195746', 'add_world_setting', 2, 1, 1, now(), 10, NULL, 'h', 'Atlas CLI v0.21.1'),
		('20251211123500', 'add_performance_indexes', 2, 5, 5, now(), 10, '', 'h', 'Atlas CLI v0.21.1')`)

	// when
	res, err := Migrate(t.Context(), dsn, nopLog)

	// then
	require.NoError(t, err)
	assert.Equal(t, []int64{20240429143025, 20240429143026, 20250611195746, 20251211123500}, res.Adopted)
	assert.Len(t, res.Applied, len(localVersions(t, db))-4)
	assert.Equal(t, localVersions(t, db), gooseVersions(t, db))
	assert.True(t, tableExists(t, db, "atlas_schema_revisions.atlas_schema_revisions"), "the atlas history stays")

	// when
	again, err := Migrate(t.Context(), dsn, nopLog)

	// then
	require.NoError(t, err)
	assert.Empty(t, again.Adopted)
	assert.Empty(t, again.Applied)
}

func TestMigrate_RefusesPartialAtlasRevision(t *testing.T) {
	// given
	dsn := scratchDatabase(t)
	db := openDB(t, dsn)
	atlasHistory(t, db, 20250611195746, `
		('20240429143026', 'actual_schema', 1, 0, 0, now(), 0, NULL, '', 'v'),
		('20250611195746', 'add_world_setting', 2, 1, 1, now(), 10, NULL, 'h', 'v'),
		('20251211123500', 'add_performance_indexes', 2, 2, 5, now(), 10, 'boom', 'h', 'v')`)

	// when
	res, err := Migrate(t.Context(), dsn, nopLog)

	// then
	require.ErrorContains(t, err, "atlas revision 20251211123500 is partially applied (2/5 statements")
	assert.Empty(t, res.Applied)
	assert.False(t, tableExists(t, db, goose.DefaultTablename))
}

func TestMigrate_RefusesUntrackedSchema(t *testing.T) {
	// given
	dsn := scratchDatabase(t)
	db := openDB(t, dsn)
	provider, err := newProvider(db)
	require.NoError(t, err)
	_, err = provider.UpTo(t.Context(), 20240429143026)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), "DROP TABLE "+goose.DefaultTablename)
	require.NoError(t, err)

	// when
	_, err = Migrate(t.Context(), dsn, nopLog)

	// then
	require.ErrorContains(t, err, "database has tables but no migration history")
	assert.False(t, tableExists(t, db, goose.DefaultTablename))
}

func TestRun_LockWaitTimesOut(t *testing.T) {
	// given
	dsn := scratchDatabase(t)
	holder := openDB(t, dsn)
	conn, err := holder.Conn(t.Context())
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.ExecContext(t.Context(), "SELECT pg_advisory_lock($1)", migrationLockKey)
	require.NoError(t, err)
	db := openDB(t, dsn)

	// when
	began := time.Now()
	_, err = run(t.Context(), db, 200*time.Millisecond, nopLog)

	// then
	require.ErrorContains(t, err, "wait for the migration lock")
	assert.Less(t, time.Since(began), 10*time.Second)
	assert.False(t, tableExists(t, db, goose.DefaultTablename), "nothing ran without the lock")
}
