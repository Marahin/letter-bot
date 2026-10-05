package postgresql

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var nopLog = zap.NewNop().Sugar()

var migrationName = regexp.MustCompile(`^(\d{14})_[a-z0-9_]+\.sql$`)

func migrationFiles(t *testing.T) []string {
	t.Helper()
	names, err := fs.Glob(Migrations(), "*")
	require.NoError(t, err)
	require.NotEmpty(t, names)
	return names
}

func readMigration(t *testing.T, name string) string {
	t.Helper()
	body, err := fs.ReadFile(Migrations(), name)
	require.NoError(t, err)
	return string(body)
}

func TestMigrations_AreGooseFiles(t *testing.T) {
	for _, name := range migrationFiles(t) {
		t.Run(name, func(t *testing.T) {
			// given
			body := readMigration(t, name)

			// when
			var first string
			for line := range strings.Lines(body) {
				if strings.TrimSpace(line) != "" {
					first = strings.TrimSpace(line)
					break
				}
			}

			// then
			assert.Regexp(t, migrationName, name)
			assert.Equal(t, "-- +goose Up", first)
			if strings.Contains(strings.ToUpper(body), "CONCURRENTLY") {
				assert.Contains(t, body, "-- +goose NO TRANSACTION", "CONCURRENTLY cannot run in a transaction")
			}
		})
	}
}

func TestMigrations_VersionsAreUniqueAndSorted(t *testing.T) {
	// given
	var versions []int64
	for _, name := range migrationFiles(t) {
		m := migrationName.FindStringSubmatch(name)
		require.NotNil(t, m, name)
		v, err := strconv.ParseInt(m[1], 10, 64)
		require.NoError(t, err)
		versions = append(versions, v)
	}

	// when
	sorted := slices.Clone(versions)
	slices.Sort(sorted)

	// then
	assert.Equal(t, sorted, versions)
	assert.Len(t, slices.Compact(sorted), len(versions), "versions must be unique")
}

func TestMigrations_GooseLoadsThem(t *testing.T) {
	// given
	db, err := sql.Open("pgx", "postgres://u:p@127.0.0.1:1/d?sslmode=disable")
	require.NoError(t, err)
	defer db.Close()

	// when
	provider, err := newProvider(db)

	// then
	require.NoError(t, err)
	assert.Len(t, provider.ListSources(), len(migrationFiles(t)))
}

// A change to a released migration does not run again on a database that has it, so
// every change to this snapshot must be a new file.
func TestMigrations_Checksums(t *testing.T) {
	// given
	var sums strings.Builder

	// when
	for _, name := range migrationFiles(t) {
		sum := sha256.Sum256([]byte(readMigration(t, name)))
		fmt.Fprintf(&sums, "%s %s\n", name, hex.EncodeToString(sum[:]))
	}

	// then
	snaps.MatchSnapshot(t, sums.String())
}

func TestRun_FailsOnAnUnreachableDatabase(t *testing.T) {
	// given
	db, err := sql.Open("pgx", "postgres://u:p@127.0.0.1:1/d?sslmode=disable&connect_timeout=1")
	require.NoError(t, err)
	defer db.Close()

	// when
	_, err = run(t.Context(), db, migrationLockWait, nopLog)

	// then
	assert.ErrorContains(t, err, "connect")
}
