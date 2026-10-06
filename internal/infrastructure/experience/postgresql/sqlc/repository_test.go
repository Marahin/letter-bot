package sqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/ports"
)

var (
	t0      = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

func newMock(t *testing.T) pgxmock.PgxPoolIface {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock
}

func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func snapshotRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "world", "character_key", "character_name", "level", "experience", "vocation", "observed_at", "last_seen_at"})
}

func tsz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func TestExperienceRepository_ListTrackedWorlds(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("FROM guilds_world").WillReturnRows(pgxmock.NewRows([]string{"world"}).AddRow("Antica").AddRow("Celesta"))
	repo := NewExperienceRepository(mock)

	// when
	worlds, err := repo.ListTrackedWorlds(context.Background())

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"Antica", "Celesta"}, worlds)
}

func TestExperienceRepository_ListTrackedCharacterKeys(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("string_to_array").
		WithArgs("Celesta", tsz(t0)).
		WillReturnRows(pgxmock.NewRows([]string{"character_key"}).AddRow("quiet nyx"))
	repo := NewExperienceRepository(mock)

	// when
	keys, err := repo.ListTrackedCharacterKeys(context.Background(), "Celesta", t0)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"quiet nyx"}, keys)
}

func TestExperienceRepository_LatestSnapshots(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("DISTINCT ON").WithArgs("Celesta", []string{"quiet nyx", "missing"}).
		WillReturnRows(snapshotRows().AddRow(int64(3), "Celesta", "quiet nyx", "Quiet Nyx", int32(500), int64(1000), "Elite Knight", tsz(t0), tsz(t0.Add(time.Hour))))
	mock.ExpectQuery("DISTINCT ON").WithArgs("Celesta", []string{"x"}).WillReturnError(errBoom)
	repo := NewExperienceRepository(mock)

	// when
	got, err := repo.LatestSnapshots(context.Background(), "Celesta", []string{"quiet nyx", "missing"})
	empty, emptyErr := repo.LatestSnapshots(context.Background(), "Celesta", nil)
	_, failedErr := repo.LatestSnapshots(context.Background(), "Celesta", []string{"x"})

	// then
	require.NoError(t, err)
	assert.Equal(t, map[string]experience.Snapshot{"quiet nyx": {
		ID: 3, World: "Celesta", CharacterKey: "quiet nyx", CharacterName: "Quiet Nyx", Level: 500, Experience: 1000,
		Vocation: "Elite Knight", ObservedAt: t0, LastSeenAt: t0.Add(time.Hour),
	}}, got)
	assert.NoError(t, emptyErr)
	assert.Empty(t, empty)
	assert.ErrorIs(t, failedErr, errBoom)
}

func TestExperienceRepository_SaveRun(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO highscore_snapshots").
		WithArgs("Celesta", []string{"quiet nyx"}, []string{"Quiet Nyx"}, []int32{500}, []int64{1000}, []string{"Elite Knight"}, tsz(t0)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("UPDATE highscore_snapshots").WithArgs(tsz(t0), []int64{4, 5}).WillReturnResult(pgxmock.NewResult("UPDATE", 2))
	mock.ExpectExec("INSERT INTO highscore_runs").WithArgs("Celesta", tsz(t0), tsz(t0.Add(time.Minute)), int32(20), int32(1000)).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	repo := NewExperienceRepository(mock)

	// when
	err := repo.SaveRun(context.Background(), experience.RunResult{
		Run:      experience.Run{World: "Celesta", ObservedAt: t0, FetchedAt: t0.Add(time.Minute), Pages: 20, Rows: 1000},
		Inserted: []experience.Snapshot{{CharacterKey: "quiet nyx", CharacterName: "Quiet Nyx", Level: 500, Experience: 1000, Vocation: "Elite Knight"}},
		SeenIDs:  []int64{4, 5},
	})

	// then
	assert.NoError(t, err)
}

func TestExperienceRepository_SaveRunWithOnlyTheRun(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO highscore_runs").WithArgs(anyArgs(5)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectCommit()
	repo := NewExperienceRepository(mock)

	// when
	err := repo.SaveRun(context.Background(), experience.RunResult{Run: experience.Run{World: "Celesta"}})

	// then
	assert.NoError(t, err)
}

func TestExperienceRepository_SaveRunRollsBack(t *testing.T) {
	tests := []struct {
		name  string
		setup func(mock pgxmock.PgxPoolIface)
	}{
		{name: "begin", setup: func(mock pgxmock.PgxPoolIface) { mock.ExpectBegin().WillReturnError(errBoom) }},
		{name: "insert snapshots", setup: func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO highscore_snapshots").WithArgs(anyArgs(7)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		}},
		{name: "touch", setup: func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO highscore_snapshots").WithArgs(anyArgs(7)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectExec("UPDATE highscore_snapshots").WithArgs(anyArgs(2)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		}},
		{name: "run", setup: func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO highscore_snapshots").WithArgs(anyArgs(7)...).WillReturnResult(pgxmock.NewResult("INSERT", 1))
			mock.ExpectExec("UPDATE highscore_snapshots").WithArgs(anyArgs(2)...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			mock.ExpectExec("INSERT INTO highscore_runs").WithArgs(anyArgs(5)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			mock := newMock(t)
			tt.setup(mock)
			repo := NewExperienceRepository(mock)

			// when
			err := repo.SaveRun(context.Background(), experience.RunResult{
				Run:      experience.Run{World: "Celesta"},
				Inserted: []experience.Snapshot{{CharacterKey: "a"}},
				SeenIDs:  []int64{1},
			})

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestExperienceRepository_FirstRunObservedAt(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("min\\(observed_at\\)").WithArgs("Celesta").WillReturnRows(pgxmock.NewRows([]string{"observed_at"}).AddRow(tsz(t0)))
	mock.ExpectQuery("min\\(observed_at\\)").WithArgs("Antica").WillReturnRows(pgxmock.NewRows([]string{"observed_at"}).AddRow(pgtype.Timestamptz{}))
	mock.ExpectQuery("min\\(observed_at\\)").WithArgs("Broken").WillReturnError(errBoom)
	repo := NewExperienceRepository(mock)

	// when
	got, err := repo.FirstRunObservedAt(context.Background(), "Celesta")
	_, noneErr := repo.FirstRunObservedAt(context.Background(), "Antica")
	_, failedErr := repo.FirstRunObservedAt(context.Background(), "Broken")

	// then
	require.NoError(t, err)
	assert.Equal(t, t0, got)
	assert.ErrorIs(t, noneErr, ports.ErrNotFound)
	assert.ErrorIs(t, failedErr, errBoom)
}

func TestExperienceRepository_FirstRunObservedAtOrAfter(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("FROM highscore_runs").WithArgs("Celesta", tsz(t0)).WillReturnRows(pgxmock.NewRows([]string{"observed_at"}).AddRow(tsz(t0.Add(time.Minute))))
	mock.ExpectQuery("FROM highscore_runs").WithArgs("Celesta", tsz(t0.Add(time.Hour))).WillReturnError(pgx.ErrNoRows)
	repo := NewExperienceRepository(mock)

	// when
	got, err := repo.FirstRunObservedAtOrAfter(context.Background(), "Celesta", t0)
	_, noneErr := repo.FirstRunObservedAtOrAfter(context.Background(), "Celesta", t0.Add(time.Hour))

	// then
	require.NoError(t, err)
	assert.Equal(t, t0.Add(time.Minute), got)
	assert.ErrorIs(t, noneErr, ports.ErrNotFound)
}

func TestExperienceRepository_SnapshotLookups(t *testing.T) {
	// given
	mock := newMock(t)
	row := func() *pgxmock.Rows {
		return snapshotRows().AddRow(int64(1), "Celesta", "a", "A", int32(1), int64(10), "", tsz(t0), tsz(t0))
	}
	mock.ExpectQuery("observed_at <= ").WithArgs("Celesta", "a", tsz(t0)).WillReturnRows(row())
	mock.ExpectQuery("observed_at <= ").WithArgs("Celesta", "b", tsz(t0)).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("observed_at > ").WithArgs("Celesta", "a", tsz(t0), tsz(t0.Add(time.Hour))).WillReturnRows(row())
	mock.ExpectQuery("observed_at > ").WithArgs("Celesta", "b", tsz(t0), tsz(t0.Add(time.Hour))).WillReturnError(pgx.ErrNoRows)
	repo := NewExperienceRepository(mock)

	// when
	before, beforeErr := repo.SnapshotAtOrBefore(context.Background(), "Celesta", "a", t0)
	_, beforeNone := repo.SnapshotAtOrBefore(context.Background(), "Celesta", "b", t0)
	after, afterErr := repo.FirstSnapshotAfter(context.Background(), "Celesta", "a", t0, t0.Add(time.Hour))
	_, afterNone := repo.FirstSnapshotAfter(context.Background(), "Celesta", "b", t0, t0.Add(time.Hour))

	// then
	require.NoError(t, beforeErr)
	require.NoError(t, afterErr)
	assert.Equal(t, int64(10), before.Experience)
	assert.Equal(t, int64(10), after.Experience)
	assert.ErrorIs(t, beforeNone, ports.ErrNotFound)
	assert.ErrorIs(t, afterNone, ports.ErrNotFound)
}

func TestExperienceRepository_PendingReservations(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("NOT EXISTS").WithArgs("Celesta", tsz(t0), tsz(t0.Add(time.Hour)), tsz(t0.Add(2*time.Hour))).
		WillReturnRows(pgxmock.NewRows([]string{"id", "author", "start_at", "end_at"}).AddRow(int64(7), "A/B", tsz(t0), tsz(t0.Add(time.Hour))))
	mock.ExpectQuery("NOT EXISTS").WithArgs(anyArgs(4)...).WillReturnError(errBoom)
	repo := NewExperienceRepository(mock)

	// when
	got, err := repo.PendingReservations(context.Background(), "Celesta", t0, t0.Add(time.Hour), t0.Add(2*time.Hour))
	_, failedErr := repo.PendingReservations(context.Background(), "Celesta", t0, t0, t0)

	// then
	require.NoError(t, err)
	assert.Equal(t, []experience.PendingReservation{{ID: 7, Author: "A/B", StartAt: t0, EndAt: t0.Add(time.Hour)}}, got)
	assert.ErrorIs(t, failedErr, errBoom)
}

func TestExperienceRepository_InsertReservationExperience(t *testing.T) {
	// given
	mock := newMock(t)
	start, end, gain := int64(100), int64(150), int64(50)
	mock.ExpectExec("INSERT INTO reservation_experience").
		WithArgs([]int64{7, 7}, []string{"a", "b"}, []string{"A", "B"}, []int64{100, 0}, []bool{true, false},
			[]int64{150, 0}, []bool{true, false}, []int64{50, 0}, []string{"ok", "no_data"}).
		WillReturnResult(pgxmock.NewResult("INSERT", 2))
	repo := NewExperienceRepository(mock)

	// when
	err := repo.InsertReservationExperience(context.Background(), []experience.ReservationExperience{
		{ReservationID: 7, CharacterKey: "a", CharacterName: "A", StartExperience: &start, EndExperience: &end, Gain: &gain, Status: experience.StatusOK},
		{ReservationID: 7, CharacterKey: "b", CharacterName: "B", Status: experience.StatusNoData},
	})
	emptyErr := repo.InsertReservationExperience(context.Background(), nil)

	// then
	assert.NoError(t, err)
	assert.NoError(t, emptyErr)
}

func TestExperienceRepository_SnapshotHistory(t *testing.T) {
	// given
	mock := newMock(t)
	mock.ExpectQuery("observed_at >= ").WithArgs("Celesta", "a", tsz(t0), tsz(t0.Add(time.Hour))).
		WillReturnRows(snapshotRows().
			AddRow(int64(1), "Celesta", "a", "A", int32(1), int64(10), "", tsz(t0), tsz(t0)).
			AddRow(int64(2), "Celesta", "a", "A", int32(2), int64(20), "", tsz(t0.Add(time.Minute)), tsz(t0.Add(time.Minute))))
	mock.ExpectQuery("observed_at >= ").WithArgs(anyArgs(4)...).WillReturnError(errBoom)
	repo := NewExperienceRepository(mock)

	// when
	got, err := repo.SnapshotHistory(context.Background(), "Celesta", "a", t0, t0.Add(time.Hour))
	_, failedErr := repo.SnapshotHistory(context.Background(), "Celesta", "a", t0, t0)

	// then
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, int64(20), got[1].Experience)
	assert.ErrorIs(t, failedErr, errBoom)
}
