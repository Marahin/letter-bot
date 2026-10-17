package sqlc

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/loot"
	"spot-assistant/internal/ports"
)

var (
	t0      = time.Date(2026, 9, 25, 18, 0, 0, 0, time.UTC)
	errBoom = errors.New("boom")
)

func newMock(t *testing.T) (pgxmock.PgxPoolIface, *LootRepository) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	return mock, NewLootRepository(mock)
}

func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

func tsz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func session() loot.Session {
	return loot.Session{
		Hash:        []byte{1, 2, 3},
		SubmittedBy: "111",
		StartedAt:   t0,
		EndedAt:   t0.Add(time.Hour),
		Duration:  time.Hour,
		PartySize: 2,
		Members: []loot.SessionMember{
			{Member: loot.Member{Name: "Marahin", Loot: 1, Supplies: 2, Balance: 3, Damage: 4, Healing: 5}, Key: "marahin", Position: 0},
			{Member: loot.Member{Name: "Iscarlott", Leader: true, Loot: 6, Supplies: 7, Balance: 8, Damage: 9, Healing: 10}, Key: "iscarlott", Position: 1},
		},
		Loot: 7, Supplies: 9, Balance: 11, Damage: 13, Healing: 15,
	}
}

func attribution() loot.Attribution {
	return loot.Attribution{SessionID: 42, ReservationID: 9, OverlapSeconds: 60}
}

func TestLootRepository_SaveSession_New(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("name: InsertSession").
		WithArgs([]byte{1, 2, 3}, "111", tsz(t0), tsz(t0.Add(time.Hour)), int32(3600), int32(2), int64(7), int64(9), int64(11), int64(13), int64(15)).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(42)))
	mock.ExpectExec("name: InsertMembers").
		WithArgs(int64(42), []string{"marahin", "iscarlott"}, []string{"Marahin", "Iscarlott"}, []int32{0, 1}, []bool{false, true},
			[]int64{1, 6}, []int64{2, 7}, []int64{3, 8}, []int64{4, 9}, []int64{5, 10}).
		WillReturnResult(pgxmock.NewResult("INSERT", 2))
	mock.ExpectCommit()

	// when
	id, err := repo.SaveSession(context.Background(), session())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
}

func TestLootRepository_SaveSession_StoredBeforeReturnsItsID(t *testing.T) {
	// given
	mock, repo := newMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("name: InsertSession").WithArgs(anyArgs(11)...).WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("name: SessionIDByHash").WithArgs([]byte{1, 2, 3}).WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectCommit()

	// when
	id, err := repo.SaveSession(context.Background(), session())

	// then
	require.NoError(t, err)
	assert.Equal(t, int64(7), id)
}

func TestLootRepository_SaveSession_RollsBack(t *testing.T) {
	tests := map[string]func(mock pgxmock.PgxPoolIface){
		"begin": func(mock pgxmock.PgxPoolIface) { mock.ExpectBegin().WillReturnError(errBoom) },
		"insert": func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectQuery("name: InsertSession").WithArgs(anyArgs(11)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		},
		"lookup": func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectQuery("name: InsertSession").WithArgs(anyArgs(11)...).WillReturnError(pgx.ErrNoRows)
			mock.ExpectQuery("name: SessionIDByHash").WithArgs(anyArgs(1)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		},
		"members": func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectQuery("name: InsertSession").WithArgs(anyArgs(11)...).WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int64(1)))
			mock.ExpectExec("name: InsertMembers").WithArgs(anyArgs(10)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		},
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			mock, repo := newMock(t)
			setup(mock)

			// when
			_, err := repo.SaveSession(context.Background(), session())

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestLootRepository_MatchCandidates(t *testing.T) {
	// given
	mock, repo := newMock(t)
	cols := []string{"id", "guild_id", "guild_name", "spot_id", "spot_name", "author", "author_discord_id", "start_at", "end_at"}
	mock.ExpectQuery("name: MatchCandidates").
		WithArgs(tsz(t0.Add(-3*time.Hour)), tsz(t0.Add(time.Hour)), tsz(t0), []string{"marahin"}).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(int64(9), "g", "Guild", int64(7), "Hero Cave", "Marahin/Iscarlott", "111", tsz(t0), tsz(t0.Add(2*time.Hour))))
	mock.ExpectQuery("name: MatchCandidates").WithArgs(anyArgs(4)...).WillReturnError(pgx.ErrNoRows)

	// when
	got, err := repo.MatchCandidates(context.Background(), t0, t0.Add(time.Hour), []string{"marahin"}, 3*time.Hour)
	_, errFail := repo.MatchCandidates(context.Background(), t0, t0.Add(time.Hour), []string{"marahin"}, 3*time.Hour)

	// then
	require.NoError(t, err)
	assert.Equal(t, []loot.Candidate{{
		ReservationID: 9, GuildID: "g", GuildName: "Guild", SpotID: 7, SpotName: "Hero Cave",
		Author: "Marahin/Iscarlott", AuthorDiscordID: "111", StartAt: t0, EndAt: t0.Add(2 * time.Hour),
	}}, got)
	assert.ErrorIs(t, errFail, ports.ErrNotFound, "mapped by postgresql.MapError")
}

func TestLootRepository_Pick(t *testing.T) {
	t.Run("counted", func(t *testing.T) {
		// given
		mock, repo := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("name: DeleteOtherPicks").WithArgs(int64(42), int64(9)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
		mock.ExpectQuery("name: PickSession").WithArgs(int64(9), int64(42), int32(3600), true).WillReturnRows(pgxmock.NewRows([]string{"session_id"}).AddRow(int64(42)))
		mock.ExpectCommit()

		// when
		ok, err := repo.Pick(context.Background(), loot.Attribution{SessionID: 42, ReservationID: 9, OverlapSeconds: 3600, ByAuthor: true})

		// then
		require.NoError(t, err)
		assert.True(t, ok)
	})
	t.Run("the reservation keeps its session and the delete rolls back", func(t *testing.T) {
		// given
		mock, repo := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec("name: DeleteOtherPicks").WithArgs(anyArgs(2)...).WillReturnResult(pgxmock.NewResult("DELETE", 1))
		mock.ExpectQuery("name: PickSession").WithArgs(anyArgs(4)...).WillReturnError(pgx.ErrNoRows)
		mock.ExpectRollback()

		// when
		ok, err := repo.Pick(context.Background(), attribution())

		// then
		require.NoError(t, err)
		assert.False(t, ok)
	})
	for name, setup := range map[string]func(mock pgxmock.PgxPoolIface){
		"begin": func(mock pgxmock.PgxPoolIface) { mock.ExpectBegin().WillReturnError(errBoom) },
		"delete": func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectExec("name: DeleteOtherPicks").WithArgs(anyArgs(2)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		},
		"pick": func(mock pgxmock.PgxPoolIface) {
			mock.ExpectBegin()
			mock.ExpectExec("name: DeleteOtherPicks").WithArgs(anyArgs(2)...).WillReturnResult(pgxmock.NewResult("DELETE", 0))
			mock.ExpectQuery("name: PickSession").WithArgs(anyArgs(4)...).WillReturnError(errBoom)
			mock.ExpectRollback()
		},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			mock, repo := newMock(t)
			setup(mock)

			// when
			_, err := repo.Pick(context.Background(), attribution())

			// then
			assert.ErrorIs(t, err, errBoom)
		})
	}
}

func TestLootRepository_SaveSession_RefusesFiguresOutOfTheInt32Range(t *testing.T) {
	tests := map[string]func(*loot.Session){
		"duration":   func(s *loot.Session) { s.Duration = (math.MaxInt32 + 1) * time.Second },
		"party size": func(s *loot.Session) { s.PartySize = math.MaxInt32 + 1 },
		"position":   func(s *loot.Session) { s.Members[1].Position = math.MinInt32 - 1 },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			// given
			_, repo := newMock(t)
			s := session()
			edit(&s)

			// when
			_, err := repo.SaveSession(context.Background(), s)

			// then
			assert.ErrorIs(t, err, errOutOfRange)
		})
	}
}

func TestLootRepository_Pick_RefusesAnOverlapOutOfTheInt32Range(t *testing.T) {
	// given
	_, repo := newMock(t)
	a := attribution()
	a.OverlapSeconds = math.MaxInt32 + 1

	// when
	_, err := repo.Pick(context.Background(), a)

	// then
	assert.ErrorIs(t, err, errOutOfRange)
}
