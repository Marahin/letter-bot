package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
	"go.uber.org/zap"
)

// atlasTable is the history table that bin/migrate (atlas, --revisions-schema
// atlas_schema_revisions) wrote before goose.
const atlasTable = "atlas_schema_revisions.atlas_schema_revisions"

// The bits of atlas_schema_revisions.type.
const (
	atlasBaseline = 1 << 0
	atlasExecute  = 1 << 1
	atlasResolved = 1 << 2
)

var atlasVersion = regexp.MustCompile(`^\d{14}$`)

type atlasRevision struct {
	Version string
	Type    int
	Applied int
	Total   int
	Error   string
}

// adoptionVersions returns the local versions that the atlas history marks as applied.
// A baseline revision stands for its own version and every version before it.
func adoptionVersions(revs []atlasRevision, local []int64) ([]int64, error) {
	var maxDone int64
	for _, r := range revs {
		if !atlasVersion.MatchString(r.Version) {
			continue // atlas keeps internal rows in the same table
		}
		done := r.Type&atlasBaseline != 0 || r.Type&atlasResolved != 0 ||
			(r.Type&atlasExecute != 0 && r.Error == "" && r.Applied == r.Total)
		if !done {
			return nil, fmt.Errorf("atlas revision %s is partially applied (%d/%d statements, error %q); fix it by hand before starting",
				r.Version, r.Applied, r.Total, r.Error)
		}
		v, err := strconv.ParseInt(r.Version, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("atlas revision %s: %w", r.Version, err)
		}
		maxDone = max(maxDone, v)
	}
	if maxDone == 0 {
		return nil, nil
	}
	if !slices.Contains(local, maxDone) {
		return nil, fmt.Errorf("atlas revision %d is not a migration of this binary; the database is newer or has an unknown history", maxDone)
	}
	var adopted []int64
	for _, v := range local {
		if v <= maxDone {
			adopted = append(adopted, v)
		}
	}
	return adopted, nil
}

// adopt writes a finished atlas history to goose_db_version once, so goose does not
// run those migrations again. The caller holds the migration lock.
func adopt(ctx context.Context, db *sql.DB, local []int64, log *zap.SugaredLogger) (adopted []int64, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("adopt atlas history: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	gooseTable, err := exists(ctx, tx, goose.DefaultTablename)
	if err != nil {
		return nil, err
	}
	if gooseTable {
		var tracked bool
		q := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE version_id > 0)", goose.DefaultTablename)
		if err := tx.QueryRowContext(ctx, q).Scan(&tracked); err != nil {
			return nil, fmt.Errorf("read goose history: %w", err)
		}
		if tracked {
			return nil, tx.Rollback()
		}
	}

	atlas, err := exists(ctx, tx, atlasTable)
	if err != nil {
		return nil, err
	}
	if atlas {
		revs, err := atlasRevisions(ctx, tx)
		if err != nil {
			return nil, err
		}
		if adopted, err = adoptionVersions(revs, local); err != nil {
			return nil, err
		}
	}
	if len(adopted) == 0 {
		untracked, err := exists(ctx, tx, "public.web_reservation")
		if err != nil {
			return nil, err
		}
		if untracked {
			return nil, errors.New("database has tables but no migration history (atlas or goose); baseline it by hand")
		}
		return nil, tx.Rollback()
	}
	if err := recordVersions(ctx, tx, gooseTable, adopted); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("adopt atlas history: %w", err)
	}
	log.Infow("adopted the atlas migration history", "up_to", adopted[len(adopted)-1], "count", len(adopted))
	return adopted, nil
}

func exists(ctx context.Context, tx *sql.Tx, table string) (bool, error) {
	var found bool
	if err := tx.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&found); err != nil {
		return false, fmt.Errorf("look up table %s: %w", table, err)
	}
	return found, nil
}

func atlasRevisions(ctx context.Context, tx *sql.Tx) ([]atlasRevision, error) {
	rows, err := tx.QueryContext(ctx, "SELECT version, type, applied, total, coalesce(error, '') FROM "+atlasTable+" ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("read atlas history: %w", err)
	}
	defer rows.Close()
	var revs []atlasRevision
	for rows.Next() {
		var r atlasRevision
		if err := rows.Scan(&r.Version, &r.Type, &r.Applied, &r.Total, &r.Error); err != nil {
			return nil, fmt.Errorf("read atlas history: %w", err)
		}
		revs = append(revs, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read atlas history: %w", err)
	}
	return revs, nil
}

func recordVersions(ctx context.Context, tx *sql.Tx, tableExists bool, versions []int64) error {
	store, err := database.NewStore(database.DialectPostgres, goose.DefaultTablename)
	if err != nil {
		return err
	}
	if !tableExists {
		if err := store.CreateVersionTable(ctx, tx); err != nil {
			return fmt.Errorf("create goose history: %w", err)
		}
	}
	var hasZero bool
	q := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE version_id = 0)", goose.DefaultTablename)
	if err := tx.QueryRowContext(ctx, q).Scan(&hasZero); err != nil {
		return fmt.Errorf("read goose history: %w", err)
	}
	if !hasZero {
		versions = append([]int64{0}, versions...)
	}
	for _, v := range versions {
		if err := store.Insert(ctx, tx, database.InsertRequest{Version: v}); err != nil {
			return fmt.Errorf("record version %d: %w", v, err)
		}
	}
	return nil
}
