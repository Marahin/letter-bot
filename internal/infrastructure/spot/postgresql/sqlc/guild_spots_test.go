package sqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/ports"
)

func newGuildSpotRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "name", "created_at", "guild_id", "archived_at"})
}

func newSpotMock(t *testing.T) pgxmock.PgxPoolIface {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock
}

func TestSelectGuildSpots_MapsGuildAndArchivedAt(t *testing.T) {
	// given
	mock := newSpotMock(t)
	archivedAt := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM web_spot WHERE guild_id = \\$1::text").
		WithArgs("guild-1", true).
		WillReturnRows(newGuildSpotRows().
			AddRow(int64(1), "Dragon Lords", time.Now(), "guild-1", nil).
			AddRow(int64(2), "empty", time.Now(), "guild-1", archivedAt))
	repo := NewSpotRepository(mock)

	// when
	spots, err := repo.SelectGuildSpots(context.Background(), "guild-1", true)

	// then
	require.NoError(t, err)
	require.Len(t, spots, 2)
	assert.Equal(t, "guild-1", spots[0].GuildID)
	assert.False(t, spots[0].IsArchived())
	assert.True(t, spots[1].IsArchived())
	assert.True(t, archivedAt.Equal(*spots[1].ArchivedAt))
}

func TestSelectGuildSpots_Error(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("FROM web_spot").WithArgs("guild-1", false).WillReturnError(errors.New("boom"))
	repo := NewSpotRepository(mock)

	// when
	spots, err := repo.SelectGuildSpots(context.Background(), "guild-1", false)

	// then
	assert.Error(t, err)
	assert.Empty(t, spots)
}

func TestSelectGuildSpotByName_NotFound(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("lower\\(name\\) = lower\\(\\$2\\)").
		WithArgs("guild-1", "Nope").
		WillReturnError(pgx.ErrNoRows)
	repo := NewSpotRepository(mock)

	// when
	s, err := repo.SelectGuildSpotByName(context.Background(), "guild-1", "Nope")

	// then
	assert.Nil(t, s)
	assert.ErrorIs(t, err, ports.ErrNotFound)
}

func TestSelectGuildSpotByName_Found(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("lower\\(name\\) = lower\\(\\$2\\)").
		WithArgs("guild-1", "dragon lords").
		WillReturnRows(newGuildSpotRows().AddRow(int64(1), "Dragon Lords", time.Now(), "guild-1", nil))
	repo := NewSpotRepository(mock)

	// when
	s, err := repo.SelectGuildSpotByName(context.Background(), "guild-1", "dragon lords")

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), s.ID)
}

func TestSelectGuildSpotsLike(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("LIKE").
		WithArgs("guild-1", "dra").
		WillReturnRows(newGuildSpotRows().AddRow(int64(1), "Dragon Lords", time.Now(), "guild-1", nil))
	repo := NewSpotRepository(mock)

	// when
	spots, err := repo.SelectGuildSpotsLike(context.Background(), "guild-1", "dra")

	// then
	require.NoError(t, err)
	assert.Len(t, spots, 1)
}

func TestSelectGuildSpotsLike_Error(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("LIKE").WithArgs("guild-1", "dra").WillReturnError(errors.New("boom"))
	repo := NewSpotRepository(mock)

	// when
	spots, err := repo.SelectGuildSpotsLike(context.Background(), "guild-1", "dra")

	// then
	assert.Error(t, err)
	assert.Empty(t, spots)
}

func TestSelectGuildSpotByID(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("WHERE id = \\$1").
		WithArgs(int64(7), "guild-1").
		WillReturnRows(newGuildSpotRows().AddRow(int64(7), "Hero Cave", time.Now(), "guild-1", nil))
	mock.ExpectQuery("WHERE id = \\$1").
		WithArgs(int64(8), "guild-1").
		WillReturnError(pgx.ErrNoRows)
	repo := NewSpotRepository(mock)

	// when
	found, err := repo.SelectGuildSpotByID(context.Background(), "guild-1", 7)
	missing, missingErr := repo.SelectGuildSpotByID(context.Background(), "guild-1", 8)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Hero Cave", found.Name)
	assert.Nil(t, missing)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
}

func TestInsertSpot(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("INSERT INTO web_spot").
		WithArgs("Hero Cave", "guild-1").
		WillReturnRows(newGuildSpotRows().AddRow(int64(9), "Hero Cave", time.Now(), "guild-1", nil))
	repo := NewSpotRepository(mock)

	// when
	s, err := repo.InsertSpot(context.Background(), "guild-1", "Hero Cave")

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(9), s.ID)
	assert.Equal(t, "guild-1", s.GuildID)
}

func TestInsertSpot_Duplicate(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("INSERT INTO web_spot").
		WithArgs("Hero Cave", "guild-1").
		WillReturnError(&pgconn.PgError{Code: "23505"})
	repo := NewSpotRepository(mock)

	// when
	s, err := repo.InsertSpot(context.Background(), "guild-1", "Hero Cave")

	// then
	assert.Nil(t, s)
	assert.ErrorIs(t, err, ports.ErrDuplicate)
}

func TestSpotMutations_RowsAffected(t *testing.T) {
	cases := map[string]struct {
		sql  string
		args []any
		call func(repo *SpotRepository) error
	}{
		"rename": {
			sql:  "UPDATE web_spot SET name",
			args: []any{"New", int64(1), "guild-1"},
			call: func(repo *SpotRepository) error {
				return repo.RenameSpot(context.Background(), "guild-1", 1, "New")
			},
		},
		"archive": {
			sql:  "SET archived_at = now\\(\\)",
			args: []any{int64(1), "guild-1"},
			call: func(repo *SpotRepository) error { return repo.ArchiveSpot(context.Background(), "guild-1", 1) },
		},
		"restore": {
			sql:  "SET archived_at = NULL",
			args: []any{int64(1), "guild-1"},
			call: func(repo *SpotRepository) error { return repo.RestoreSpot(context.Background(), "guild-1", 1) },
		},
		"delete": {
			sql:  "DELETE FROM web_spot",
			args: []any{int64(1), "guild-1"},
			call: func(repo *SpotRepository) error { return repo.DeleteSpot(context.Background(), "guild-1", 1) },
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			mock := newSpotMock(t)
			mock.ExpectExec(tc.sql).WithArgs(tc.args...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
			mock.ExpectExec(tc.sql).WithArgs(tc.args...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
			repo := NewSpotRepository(mock)

			// when
			okErr := tc.call(repo)
			missingErr := tc.call(repo)

			// then
			assert.NoError(t, okErr)
			assert.ErrorIs(t, missingErr, ports.ErrNotFound)
		})
	}
}

func TestRenameSpot_Duplicate(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectExec("UPDATE web_spot SET name").
		WithArgs("Taken", int64(1), "guild-1").
		WillReturnError(&pgconn.PgError{Code: "23505"})
	repo := NewSpotRepository(mock)

	// when
	err := repo.RenameSpot(context.Background(), "guild-1", 1, "Taken")

	// then
	assert.ErrorIs(t, err, ports.ErrDuplicate)
}

func TestCountSpotReservations(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM web_reservation").
		WithArgs(int64(1), "guild-1").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(12)))
	repo := NewSpotRepository(mock)

	// when
	count, err := repo.CountSpotReservations(context.Background(), "guild-1", 1)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(12), count)
}

func TestSelectGuildSpotReservationCounts(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("FILTER \\(WHERE r.end_at >= now\\(\\)\\) AS upcoming").
		WithArgs("guild-1").
		WillReturnRows(pgxmock.NewRows([]string{"spot_id", "total", "upcoming"}).
			AddRow(int64(1), int64(12), int64(2)).
			AddRow(int64(4), int64(3), int64(0)))
	repo := NewSpotRepository(mock)

	// when
	counts, err := repo.SelectGuildSpotReservationCounts(context.Background(), "guild-1")

	// then
	require.NoError(t, err)
	assert.Equal(t, map[int64]spot.ReservationCounts{1: {Total: 12, Upcoming: 2}, 4: {Total: 3}}, counts)
}

func TestSelectGuildSpotReservationCounts_Error(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("FROM web_reservation").WithArgs("guild-1").WillReturnError(errors.New("boom"))
	repo := NewSpotRepository(mock)

	// when
	counts, err := repo.SelectGuildSpotReservationCounts(context.Background(), "guild-1")

	// then
	assert.Error(t, err)
	assert.Nil(t, counts)
}

func TestInsertSpotsIgnoreDuplicates(t *testing.T) {
	// given
	mock := newSpotMock(t)
	names := []string{"Dragon Lords", "Hero Cave"}
	mock.ExpectExec("ON CONFLICT \\(guild_id, lower\\(name\\)\\) WHERE archived_at IS NULL DO NOTHING").
		WithArgs("guild-1", names).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	repo := NewSpotRepository(mock)

	// when
	inserted, err := repo.InsertSpotsIgnoreDuplicates(context.Background(), "guild-1", names)

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(1), inserted)
}

func TestInsertSpotsIgnoreDuplicates_NoNamesSkipsQuery(t *testing.T) {
	// given
	mock := newSpotMock(t)
	repo := NewSpotRepository(mock)

	// when
	inserted, err := repo.InsertSpotsIgnoreDuplicates(context.Background(), "guild-1", nil)

	// then
	require.NoError(t, err)
	assert.Zero(t, inserted)
}
