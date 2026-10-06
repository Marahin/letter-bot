package sqlc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
		WithArgs("guild-1", `dra\_\%`, int32(ports.SpotsLikeLimit)).
		WillReturnRows(newGuildSpotRows().AddRow(int64(1), "Dragon Lords", time.Now(), "guild-1", nil))
	repo := NewSpotRepository(mock)

	// when
	spots, err := repo.SelectGuildSpotsLike(context.Background(), "guild-1", "dra_%")

	// then
	require.NoError(t, err)
	assert.Len(t, spots, 1)
}

func TestSelectGuildSpotsLike_Error(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("LIKE").WithArgs("guild-1", "dra", int32(ports.SpotsLikeLimit)).WillReturnError(errors.New("boom"))
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

func TestSelectSpotReservationCounts(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("WHERE s.id = \\$1").
		WithArgs(int64(1), "guild-1").
		WillReturnRows(pgxmock.NewRows([]string{"total", "upcoming"}).AddRow(int64(12), int64(2)))
	repo := NewSpotRepository(mock)

	// when
	counts, err := repo.SelectSpotReservationCounts(context.Background(), "guild-1", 1)

	// then
	require.NoError(t, err)
	assert.Equal(t, spot.ReservationCounts{Total: 12, Upcoming: 2}, counts)
}

func TestSelectSpotReservationCounts_Error(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("WHERE s.id = \\$1").WithArgs(int64(1), "guild-1").WillReturnError(errors.New("boom"))
	repo := NewSpotRepository(mock)

	// when
	_, err := repo.SelectSpotReservationCounts(context.Background(), "guild-1", 1)

	// then
	assert.Error(t, err)
}

func TestSelectGuildSpotList(t *testing.T) {
	// given
	mock := newSpotMock(t)
	archivedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("CROSS JOIN LATERAL").
		WithArgs("guild-1", true, `50\%`).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "created_at", "guild_id", "archived_at", "total", "upcoming"}).
			AddRow(int64(4), "50% Cave", pgtype.Timestamptz{Time: archivedAt, Valid: true}, pgtype.Text{String: "guild-1", Valid: true}, pgtype.Timestamptz{Time: archivedAt, Valid: true}, int64(3), int64(1)))
	repo := NewSpotRepository(mock)

	// when
	list, err := repo.SelectGuildSpotList(context.Background(), "guild-1", spot.ListFilter{Archived: true, Query: "50%"})

	// then
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "50% Cave", list[0].Name)
	assert.True(t, list[0].IsArchived())
	assert.Equal(t, spot.ReservationCounts{Total: 3, Upcoming: 1}, list[0].Reservations)
}

func TestSelectGuildSpotList_Error(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("CROSS JOIN LATERAL").WithArgs("guild-1", false, "").WillReturnError(errors.New("boom"))
	repo := NewSpotRepository(mock)

	// when
	list, err := repo.SelectGuildSpotList(context.Background(), "guild-1", spot.ListFilter{})

	// then
	assert.Error(t, err)
	assert.Nil(t, list)
}

func TestCountGuildSpots(t *testing.T) {
	// given
	mock := newSpotMock(t)
	mock.ExpectQuery("AS archived").
		WithArgs("guild-1").
		WillReturnRows(pgxmock.NewRows([]string{"active", "archived"}).AddRow(int64(7), int64(2)))
	mock.ExpectQuery("AS archived").WithArgs("guild-1").WillReturnError(errors.New("boom"))
	repo := NewSpotRepository(mock)

	// when
	active, archived, err := repo.CountGuildSpots(context.Background(), "guild-1")
	_, _, failErr := repo.CountGuildSpots(context.Background(), "guild-1")

	// then
	require.NoError(t, err)
	assert.Equal(t, 7, active)
	assert.Equal(t, 2, archived)
	assert.Error(t, failErr)
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

func TestSelectTopGuildSpots_MapsTheRanking(t *testing.T) {
	// given
	mock := newSpotMock(t)
	since := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM web_reservation r").
		WithArgs("guild-1", pgtype.Timestamptz{Time: since, Valid: true}, pgtype.Text{String: "u1", Valid: true}, int32(25)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "created_at", "guild_id", "archived_at", "bookings", "last_start_at"}).
			AddRow(int64(1), "Library -1", time.Now(), "guild-1", nil, int64(7), last))
	repo := NewSpotRepository(mock)

	// when
	ranked, err := repo.SelectTopGuildSpots(context.Background(), "guild-1", "u1", since, 25)

	// then
	require.NoError(t, err)
	require.Len(t, ranked, 1)
	assert.Equal(t, "Library -1", ranked[0].Name)
	assert.Equal(t, int64(7), ranked[0].Bookings)
	assert.True(t, last.Equal(ranked[0].LastStartAt))
}

func TestSelectTopGuildSpots_AnyAuthorAndErrors(t *testing.T) {
	// given
	mock := newSpotMock(t)
	since := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM web_reservation r").
		WithArgs("guild-1", pgtype.Timestamptz{Time: since, Valid: true}, pgtype.Text{}, int32(25)).
		WillReturnError(errors.New("boom"))
	repo := NewSpotRepository(mock)

	// when
	_, err := repo.SelectTopGuildSpots(context.Background(), "guild-1", "", since, 25)

	// then
	assert.EqualError(t, err, "boom")
}
