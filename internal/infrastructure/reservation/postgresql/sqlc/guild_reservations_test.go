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

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/ports"
)

func newReservationMock(t *testing.T) pgxmock.PgxPoolIface {
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

func newReservationThenSpotRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{
		// web_reservation
		"id", "author", "created_at", "start_at", "end_at", "spot_id", "guild_id", "author_discord_id",
		// web_spot
		"id", "name", "created_at", "guild_id", "archived_at",
	})
}

func TestSearchReservationsWithSpot_EmptyFilterSearchesAll(t *testing.T) {
	// given
	mock := newReservationMock(t)
	now := time.Now()
	mock.ExpectQuery("FROM web_reservation INNER JOIN web_spot").
		WithArgs("guild-1", pgtype.Int8{}, pgtype.Text{}, pgtype.Text{}, pgtype.Timestamptz{}, pgtype.Timestamptz{}, "all", int32(10), int32(20)).
		WillReturnRows(newReservationThenSpotRows().
			AddRow(int64(5), "Nyx", now, now, now.Add(time.Hour), int64(1), "guild-1", "111",
				int64(1), "Dragon Lords", now, "guild-1", nil))
	repo := NewReservationRepository(mock)

	// when
	res, err := repo.SearchReservationsWithSpot(context.Background(), reservation.SearchFilter{
		GuildID: "guild-1",
		Offset:  10,
		Limit:   20,
	})

	// then
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, int64(5), res[0].Reservation.ID)
	assert.Equal(t, "Dragon Lords", res[0].Spot.Name)
}

func TestSearchReservationsWithSpot_MapsEveryFilter(t *testing.T) {
	// given
	mock := newReservationMock(t)
	spotID := int64(3)
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM web_reservation INNER JOIN web_spot").
		WithArgs("guild-1",
			pgtype.Int8{Int64: 3, Valid: true},
			pgtype.Text{String: `nyx\_1`, Valid: true},
			pgtype.Text{String: "111", Valid: true},
			mocks.NewPgTimestamptzTime(from),
			mocks.NewPgTimestamptzTime(to),
			"past", int32(0), int32(50)).
		WillReturnRows(newReservationThenSpotRows())
	repo := NewReservationRepository(mock)

	// when
	res, err := repo.SearchReservationsWithSpot(context.Background(), reservation.SearchFilter{
		GuildID:         "guild-1",
		SpotID:          &spotID,
		Author:          "nyx_1",
		AuthorDiscordID: "111",
		From:            &from,
		To:              &to,
		Scope:           reservation.ScopePast,
		Limit:           50,
	})

	// then
	require.NoError(t, err)
	assert.Empty(t, res)
}

func TestSearchReservationsWithSpot_Error(t *testing.T) {
	// given
	mock := newReservationMock(t)
	mock.ExpectQuery("FROM web_reservation INNER JOIN web_spot").WithArgs(anyArgs(9)...).WillReturnError(errors.New("boom"))
	repo := NewReservationRepository(mock)

	// when
	res, err := repo.SearchReservationsWithSpot(context.Background(), reservation.SearchFilter{GuildID: "guild-1"})

	// then
	assert.Error(t, err)
	assert.Empty(t, res)
}

func TestCountReservations(t *testing.T) {
	// given
	mock := newReservationMock(t)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM web_reservation").
		WithArgs("guild-1", pgtype.Int8{}, pgtype.Text{}, pgtype.Text{}, pgtype.Timestamptz{}, pgtype.Timestamptz{}, "upcoming").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(42)))
	repo := NewReservationRepository(mock)

	// when
	count, err := repo.CountReservations(context.Background(), reservation.SearchFilter{
		GuildID: "guild-1",
		Scope:   reservation.ScopeUpcoming,
		Limit:   50,
	})

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(42), count)
}

func TestSelectGuildReservationWithSpot(t *testing.T) {
	// given
	mock := newReservationMock(t)
	now := time.Now()
	mock.ExpectQuery("WHERE web_reservation.id = \\$1").
		WithArgs(int64(5), "guild-1").
		WillReturnRows(newReservationThenSpotRows().
			AddRow(int64(5), "Nyx", now, now, now.Add(time.Hour), int64(1), "guild-1", "111",
				int64(1), "Dragon Lords", now, "guild-1", nil))
	mock.ExpectQuery("WHERE web_reservation.id = \\$1").
		WithArgs(int64(6), "guild-1").
		WillReturnError(pgx.ErrNoRows)
	repo := NewReservationRepository(mock)

	// when
	found, err := repo.SelectGuildReservationWithSpot(context.Background(), "guild-1", 5)
	missing, missingErr := repo.SelectGuildReservationWithSpot(context.Background(), "guild-1", 6)

	// then
	require.NoError(t, err)
	assert.Equal(t, "Nyx", found.Author)
	assert.Equal(t, int64(1), found.Spot.ID)
	assert.Nil(t, missing)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
}

func TestUpdateReservation(t *testing.T) {
	// given
	mock := newReservationMock(t)
	start := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	r := reservation.Reservation{ID: 5, SpotID: 2, StartAt: start, EndAt: end, Author: "Nyx/Storm", AuthorDiscordID: "111"}
	args := []any{int64(2), mocks.NewPgTimestamptzTime(start), mocks.NewPgTimestamptzTime(end), "Nyx/Storm", "111", int64(5), "guild-1"}
	mock.ExpectExec("UPDATE web_reservation").WithArgs(args...).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("UPDATE web_reservation").WithArgs(args...).WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	mock.ExpectExec("UPDATE web_reservation").WithArgs(args...).WillReturnError(&pgconn.PgError{Code: "23P01"})
	repo := NewReservationRepository(mock)

	// when
	okErr := repo.UpdateReservation(context.Background(), "guild-1", r)
	missingErr := repo.UpdateReservation(context.Background(), "guild-1", r)
	conflictErr := repo.UpdateReservation(context.Background(), "guild-1", r)

	// then
	assert.NoError(t, okErr)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
	assert.ErrorIs(t, conflictErr, ports.ErrConflict)
}

func TestDeleteGuildReservation(t *testing.T) {
	// given
	mock := newReservationMock(t)
	mock.ExpectExec("DELETE FROM web_reservation").WithArgs(int64(5), "guild-1").WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec("DELETE FROM web_reservation").WithArgs(int64(6), "guild-1").WillReturnResult(pgxmock.NewResult("DELETE", 0))
	repo := NewReservationRepository(mock)

	// when
	okErr := repo.DeleteGuildReservation(context.Background(), "guild-1", 5)
	missingErr := repo.DeleteGuildReservation(context.Background(), "guild-1", 6)

	// then
	assert.NoError(t, okErr)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
}

func TestSelectOverlappingReservationsBySpotID(t *testing.T) {
	// given
	mock := newReservationMock(t)
	start := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	mock.ExpectQuery("AND id <> \\$3::bigint").
		WithArgs(int64(2), "guild-1", int64(5), mocks.NewPgTimestamptzTime(start), mocks.NewPgTimestamptzTime(end)).
		WillReturnRows(newReservationRows().
			AddRow(int64(7), "Other", start, start.Add(-time.Hour), start.Add(time.Hour), int64(2), "guild-1", "222"))
	repo := NewReservationRepository(mock)

	// when
	res, err := repo.SelectOverlappingReservationsBySpotID(context.Background(), "guild-1", 2, start, end, 5)

	// then
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, int64(7), res[0].ID)
	assert.Equal(t, "222", res[0].AuthorDiscordID)
}

func TestSelectOverlappingReservationsBySpotID_Error(t *testing.T) {
	// given
	mock := newReservationMock(t)
	mock.ExpectQuery("AND id <> \\$3::bigint").WithArgs(anyArgs(5)...).WillReturnError(errors.New("boom"))
	repo := NewReservationRepository(mock)

	// when
	res, err := repo.SelectOverlappingReservationsBySpotID(context.Background(), "guild-1", 2, time.Now(), time.Now(), 0)

	// then
	assert.Error(t, err)
	assert.Empty(t, res)
}

func TestSelectKnownAuthors(t *testing.T) {
	// given
	mock := newReservationMock(t)
	mock.ExpectQuery("WITH matching AS").
		WithArgs("guild-1", `n\%yx`).
		WillReturnRows(pgxmock.NewRows([]string{"author_discord_id", "author"}).
			AddRow("111", "Quiet Nyx").
			AddRow("222", "Nyxie"))
	repo := NewReservationRepository(mock)

	// when
	authors, err := repo.SelectKnownAuthors(context.Background(), "guild-1", "n%yx")

	// then
	require.NoError(t, err)
	assert.Equal(t, []*reservation.KnownAuthor{
		{AuthorDiscordID: "111", Author: "Quiet Nyx"},
		{AuthorDiscordID: "222", Author: "Nyxie"},
	}, authors)
}

func TestSelectKnownAuthors_Error(t *testing.T) {
	// given
	mock := newReservationMock(t)
	mock.ExpectQuery("WITH matching AS").WithArgs(anyArgs(2)...).WillReturnError(errors.New("boom"))
	repo := NewReservationRepository(mock)

	// when
	authors, err := repo.SelectKnownAuthors(context.Background(), "guild-1", "nyx")

	// then
	assert.Error(t, err)
	assert.Empty(t, authors)
}

func TestSelectOverlappingReservations_FiltersBySpotID(t *testing.T) {
	// given
	mock := newReservationMock(t)
	start := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	mock.ExpectQuery("AND web_reservation.spot_id = \\$3").
		WithArgs(start, end, int64(2), "guild-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "author", "author_discord_id", "start_at", "end_at", "guild_id"}).
			AddRow(int64(7), "Other", "222", start, end, "guild-1"))
	repo := NewReservationRepository(mock)

	// when
	res, err := repo.SelectOverlappingReservations(context.Background(), 2, start, end, "guild-1")

	// then
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, int64(7), res[0].ID)
	assert.Equal(t, start, res[0].StartAt)
}
