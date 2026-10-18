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

	"spot-assistant/internal/core/dto/queue"
	"spot-assistant/internal/ports"
)

var (
	t0      = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	ctx     = context.Background()
	errBoom = errors.New("boom")
)

var entryColumns = []string{"id", "guild_id", "spot_id", "user_id", "length_minutes", "created_at", "expires_at", "offered_at"}

func newMock(t *testing.T) (pgxmock.PgxPoolIface, *SpotQueueRepository) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock, NewSpotQueueRepository(mock)
}

func tsz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func TestUpsertEntry(t *testing.T) {
	// given
	mock, repo := newMock(t)
	expires := t0.Add(24 * time.Hour)
	mock.ExpectQuery("INSERT INTO spot_queue_entries").WithArgs("g1", int64(7), "u1", int32(120), tsz(expires)).
		WillReturnRows(pgxmock.NewRows(entryColumns).AddRow(int64(1), "g1", int64(7), "u1", int32(120), tsz(t0), tsz(expires), pgtype.Timestamptz{}))
	mock.ExpectQuery("INSERT INTO spot_queue_entries").WithArgs("g1", int64(7), "u1", int32(30), tsz(expires)).WillReturnError(errBoom)

	// when
	got, err := repo.UpsertEntry(ctx, queue.Entry{GuildID: "g1", SpotID: 7, SpotName: "Library", UserID: "u1", Length: 2 * time.Hour, ExpiresAt: expires})
	_, failErr := repo.UpsertEntry(ctx, queue.Entry{GuildID: "g1", SpotID: 7, UserID: "u1", Length: 30 * time.Minute, ExpiresAt: expires})

	// then
	require.NoError(t, err)
	assert.Equal(t, &queue.Entry{ID: 1, GuildID: "g1", SpotID: 7, SpotName: "Library", UserID: "u1", Length: 2 * time.Hour, CreatedAt: t0, ExpiresAt: expires}, got)
	assert.ErrorIs(t, failErr, errBoom)
}

func TestCountUserEntries(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectQuery("SELECT count").WithArgs("g1", "u1", int64(7), tsz(t0)).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))

	// when
	n, err := repo.CountUserEntries(ctx, queue.UserCount{GuildID: "g1", UserID: "u1", ExceptSpotID: 7, Now: t0})

	// then
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestGetEntry(t *testing.T) {
	// given
	mock, repo := newMock(t)
	columns := append(append([]string{}, entryColumns...), "spot_name")
	mock.ExpectQuery("WHERE q.id").WithArgs(int64(1)).
		WillReturnRows(pgxmock.NewRows(columns).AddRow(int64(1), "g1", int64(7), "u1", int32(60), tsz(t0), tsz(t0), tsz(t0), "Library"))
	mock.ExpectQuery("WHERE q.id").WithArgs(int64(2)).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("WHERE q.id").WithArgs(int64(3)).WillReturnError(errBoom)

	// when
	got, err := repo.GetEntry(ctx, 1)
	_, missingErr := repo.GetEntry(ctx, 2)
	_, failErr := repo.GetEntry(ctx, 3)

	// then
	require.NoError(t, err)
	offered := t0
	assert.Equal(t, &queue.Entry{ID: 1, GuildID: "g1", SpotID: 7, SpotName: "Library", UserID: "u1", Length: time.Hour, CreatedAt: t0, ExpiresAt: t0, OfferedAt: &offered}, got)
	assert.ErrorIs(t, missingErr, ports.ErrNotFound)
	assert.ErrorIs(t, failErr, errBoom)
}

func TestListUserEntries(t *testing.T) {
	// given
	mock, repo := newMock(t)
	columns := append(append([]string{}, entryColumns...), "spot_name")
	mock.ExpectQuery("WHERE q.guild_id").WithArgs("g1", "u1", tsz(t0)).
		WillReturnRows(pgxmock.NewRows(columns).AddRow(int64(1), "g1", int64(7), "u1", int32(60), tsz(t0), tsz(t0), pgtype.Timestamptz{}, "Library"))
	mock.ExpectQuery("WHERE q.guild_id").WithArgs("g1", "u2", tsz(t0)).WillReturnError(errBoom)

	// when
	got, err := repo.ListUserEntries(ctx, "g1", "u1", t0)
	_, failErr := repo.ListUserEntries(ctx, "g1", "u2", t0)

	// then
	require.NoError(t, err)
	assert.Equal(t, []queue.Entry{{ID: 1, GuildID: "g1", SpotID: 7, SpotName: "Library", UserID: "u1", Length: time.Hour, CreatedAt: t0, ExpiresAt: t0}}, got)
	assert.ErrorIs(t, failErr, errBoom)
}

func TestDeletes(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectExec("DELETE FROM spot_queue_entries").WithArgs(int64(1), "u1").WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectExec("DELETE FROM spot_queue_entries").WithArgs("g1", "u1", int64(7)).WillReturnResult(pgxmock.NewResult("DELETE", 0))
	mock.ExpectExec("DELETE FROM spot_queue_entries").WithArgs(tsz(t0)).WillReturnResult(pgxmock.NewResult("DELETE", 3))

	// when
	deleted, err1 := repo.DeleteEntry(ctx, 1, "u1")
	spotDeleted, err2 := repo.DeleteUserSpotEntry(ctx, "g1", "u1", 7)
	expired, err3 := repo.DeleteExpired(ctx, t0)

	// then
	require.NoError(t, errors.Join(err1, err2, err3))
	assert.True(t, deleted)
	assert.False(t, spotDeleted)
	assert.Equal(t, int64(3), expired)
}

func TestSelectDueEntries(t *testing.T) {
	// given
	mock, repo := newMock(t)
	columns := append(append([]string{}, entryColumns...), "spot_name")
	start := t0.Add(time.Minute)
	mock.ExpectQuery("spot_queue_free").WithArgs("g1", tsz(t0), tsz(start)).
		WillReturnRows(pgxmock.NewRows(columns).AddRow(int64(1), "g1", int64(7), "u1", int32(60), tsz(t0), tsz(t0), pgtype.Timestamptz{}, "Library"))
	mock.ExpectQuery("spot_queue_free").WithArgs("g2", tsz(t0), tsz(start)).WillReturnError(errBoom)

	// when
	got, err := repo.SelectDueEntries(ctx, queue.DueQuery{GuildID: "g1", Now: t0, StartAt: start})
	_, failErr := repo.SelectDueEntries(ctx, queue.DueQuery{GuildID: "g2", Now: t0, StartAt: start})

	// then
	require.NoError(t, err)
	assert.Equal(t, []queue.Entry{{ID: 1, GuildID: "g1", SpotID: 7, SpotName: "Library", UserID: "u1", Length: time.Hour, CreatedAt: t0, ExpiresAt: t0}}, got)
	assert.ErrorIs(t, failErr, errBoom)
}

func TestClaimAndResetOffers(t *testing.T) {
	// given
	mock, repo := newMock(t)
	start := t0.Add(time.Minute)
	mock.ExpectExec("SET offered_at = \\$1").WithArgs(tsz(t0), int64(1), tsz(start)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("NOT spot_queue_free").WithArgs("g1", tsz(start)).WillReturnResult(pgxmock.NewResult("UPDATE", 2))
	mock.ExpectExec("SET offered_at = NULL").WithArgs(int64(1)).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	// when
	claimed, claimErr := repo.ClaimOffer(ctx, queue.Claim{EntryID: 1, OfferedAt: t0, StartAt: start})
	resetTakenErr := repo.ResetTakenOffers(ctx, "g1", start)
	resetErr := repo.ResetOffer(ctx, 1)

	// then
	require.NoError(t, errors.Join(claimErr, resetTakenErr, resetErr))
	assert.True(t, claimed)
}
