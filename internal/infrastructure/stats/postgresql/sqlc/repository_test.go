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
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/ports"
)

var errBoom = errors.New("boom")

func newMock(t *testing.T) (pgxmock.PgxPoolIface, *StatsRepository) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, mock.ExpectationsWereMet())
		mock.Close()
	})
	repo, err := NewStatsRepository(mock, "Europe/Berlin")
	require.NoError(t, err)
	return mock, repo
}

func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

var totalsCols = []string{"reservations", "seconds", "exp_reservations", "exp_seconds", "exp"}

func TestNewStatsRepository_NeedsANamedZone(t *testing.T) {
	// when
	utc, err := NewStatsRepository(nil, "UTC")
	_, errEmpty := NewStatsRepository(nil, "")
	_, errBogus := NewStatsRepository(nil, "Mars/Olympus")

	// then
	require.NoError(t, err)
	assert.Equal(t, "UTC", utc.tz)
	assert.Error(t, errEmpty)
	assert.Error(t, errBogus)
}

func TestStatsRepository_MapsTotalsRows(t *testing.T) {
	// given
	mock, repo := newMock(t)
	ctx := context.Background()
	f := stats.Filter{GuildID: "g", From: time.Now(), To: time.Now(), SpotID: 3, UserID: "u", CharacterKey: "k"}
	q := stats.Query{Filter: f, Sort: stats.Sort{Key: stats.SortExp, Asc: true}, Limit: 10}
	withTotal := func(cols ...string) []string { return append(append(cols, totalsCols...), "total_rows") }
	mock.ExpectQuery("name: SpotTotals").WithArgs("exp", true, int32(10), "g", pgxmock.AnyArg(), pgxmock.AnyArg(), int64(3), "u", "k").
		WillReturnRows(pgxmock.NewRows(withTotal("spot_id", "name", "archived")).AddRow(int64(3), "Hero", true, int64(1), int64(3600), int64(1), int64(3600), int64(9), int64(12)))
	mock.ExpectQuery("name: PlayerTotals").WithArgs("g", "exp", true, int32(10), pgxmock.AnyArg(), pgxmock.AnyArg(), int64(3), "u", "k").
		WillReturnRows(pgxmock.NewRows(append([]string{"user_id", "name"}, append(totalsCols, "total_rows")...)).AddRow("u", "A/B", int64(2), int64(7200), int64(0), int64(0), int64(0), int64(5)))
	mock.ExpectQuery("name: CharacterTotals").WithArgs("exp", true, int32(10), "g", pgxmock.AnyArg(), pgxmock.AnyArg(), int64(3), "u", "k").
		WillReturnRows(pgxmock.NewRows(append([]string{"character_key", "name"}, append(totalsCols, "total_rows")...)).AddRow("a", "A", int64(1), int64(60), int64(1), int64(60), int64(-5), int64(7)))
	mock.ExpectQuery("name: Daily").WithArgs(append([]any{"Europe/Berlin"}, anyArgs(6)...)...).
		WillReturnRows(pgxmock.NewRows(append([]string{"day"}, totalsCols...)).AddRow(pgtype.Date{Time: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Valid: true}, int64(1), int64(60), int64(0), int64(0), int64(0)))

	// when
	spots, err1 := repo.SpotTotals(ctx, q)
	players, err2 := repo.PlayerTotals(ctx, q)
	characters, err3 := repo.CharacterTotals(ctx, q)
	days, err4 := repo.Daily(ctx, f)

	// then
	require.NoError(t, errors.Join(err1, err2, err3, err4))
	assert.Equal(t, stats.Page[stats.SpotRow]{
		Rows:  []stats.SpotRow{{SpotID: 3, Name: "Hero", Archived: true, Totals: stats.Totals{Reservations: 1, Seconds: 3600, ExpReservations: 1, ExpSeconds: 3600, Exp: 9}}},
		Total: 12,
	}, spots)
	assert.Equal(t, "A/B", players.Rows[0].Name)
	assert.Equal(t, int64(7200), players.Rows[0].Seconds)
	assert.Equal(t, 5, players.Total)
	assert.Equal(t, int64(-5), characters.Rows[0].Exp)
	assert.Equal(t, 7, characters.Total)
	require.Len(t, days, 1)
	assert.Equal(t, "2026-09-01 00:00:00 +0200 CEST", days[0].Day.String())
}

func TestStatsRepository_CharacterLeaderboards(t *testing.T) {
	// given
	mock, repo := newMock(t)
	cols := append([]string{"total_rows", "board", "character_key", "name"}, totalsCols...)
	i8 := func(v int64) pgtype.Int8 { return pgtype.Int8{Int64: v, Valid: true} }
	txt := func(v string) pgtype.Text { return pgtype.Text{String: v, Valid: true} }
	mock.ExpectQuery("name: CharacterLeaderboards").WithArgs("g", pgxmock.AnyArg(), pgxmock.AnyArg(), int64(0), "", "", int32(10), int64(10800)).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(int64(4), txt("exp"), txt("a"), "A", i8(2), i8(7200), i8(1), i8(3600), i8(90)).
			AddRow(int64(4), txt("exp"), txt("b"), "B", i8(1), i8(3600), i8(1), i8(3600), i8(80)).
			AddRow(int64(4), txt("exp_h"), txt("b"), "B", i8(1), i8(3600), i8(1), i8(3600), i8(80)))
	mock.ExpectQuery("name: CharacterLeaderboards").WithArgs(anyArgs(8)...).
		WillReturnRows(pgxmock.NewRows(cols).AddRow(int64(3), pgtype.Text{}, pgtype.Text{}, "", pgtype.Int8{}, pgtype.Int8{}, pgtype.Int8{}, pgtype.Int8{}, pgtype.Int8{}))

	// when
	boards, err := repo.CharacterLeaderboards(context.Background(), stats.Filter{GuildID: "g"}, 10, 10800)
	empty, err2 := repo.CharacterLeaderboards(context.Background(), stats.Filter{GuildID: "g"}, 10, 10800)

	// then
	require.NoError(t, errors.Join(err, err2))
	assert.Equal(t, 4, boards.Characters)
	require.Len(t, boards.ByExp, 2)
	assert.Equal(t, stats.CharacterRow{Key: "a", Name: "A", Totals: stats.Totals{Reservations: 2, Seconds: 7200, ExpReservations: 1, ExpSeconds: 3600, Exp: 90}}, boards.ByExp[0])
	require.Len(t, boards.ByExpPerHour, 1)
	assert.Equal(t, "B", boards.ByExpPerHour[0].Name)
	assert.Equal(t, stats.CharacterBoards{Characters: 3}, empty)
}

func TestStatsRepository_PropagatesQueryErrors(t *testing.T) {
	// given
	mock, repo := newMock(t)
	ctx := context.Background()
	for _, q := range []struct {
		name string
		args int
	}{{"SpotTotals", 9}, {"PlayerTotals", 9}, {"CharacterTotals", 9}, {"CharacterLeaderboards", 8}, {"Daily", 7}, {"ReservationDays", 4}, {"CharacterReservations", 5}} {
		mock.ExpectQuery("name: " + q.name + " :").WithArgs(anyArgs(q.args)...).WillReturnError(errBoom)
	}

	// when
	_, err1 := repo.SpotTotals(ctx, stats.Query{})
	_, err2 := repo.PlayerTotals(ctx, stats.Query{})
	_, err3 := repo.CharacterTotals(ctx, stats.Query{})
	_, err7 := repo.CharacterLeaderboards(ctx, stats.Filter{}, 10, 0)
	_, err4 := repo.Daily(ctx, stats.Filter{})
	_, err5 := repo.ReservationDays(ctx, "g", time.Now(), time.Now())
	_, err6 := repo.CharacterReservations(ctx, stats.Filter{}, 5)

	// then
	for _, err := range []error{err1, err2, err3, err7, err4, err5, err6} {
		assert.ErrorIs(t, err, errBoom)
	}
}

func TestStatsRepository_ReservationDaysAndLatestPlayerName(t *testing.T) {
	// given
	mock, repo := newMock(t)
	ctx := context.Background()
	mock.ExpectQuery("name: ReservationDays").WithArgs("Europe/Berlin", "g", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"day"}).AddRow(pgtype.Date{Time: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), Valid: true}))
	mock.ExpectQuery("name: LatestPlayerName").WithArgs("g", "u").WillReturnRows(pgxmock.NewRows([]string{"name"}).AddRow("Quiet Nyx"))
	mock.ExpectQuery("name: LatestPlayerName").WithArgs("g", "x").WillReturnError(pgx.ErrNoRows)

	// when
	days, err := repo.ReservationDays(ctx, "g", time.Now(), time.Now())
	name, err2 := repo.LatestPlayerName(ctx, "g", "u")
	_, err3 := repo.LatestPlayerName(ctx, "g", "x")

	// then
	require.NoError(t, err)
	require.NoError(t, err2)
	assert.Equal(t, 2, days[0].Day())
	assert.Equal(t, "Quiet Nyx", name)
	assert.ErrorIs(t, err3, ports.ErrNotFound)
}

func TestStatsRepository_CharacterReservations(t *testing.T) {
	// given
	mock, repo := newMock(t)
	start := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	cols := []string{"id", "spot_id", "spot_name", "author", "start_at", "end_at", "status", "gain"}
	mock.ExpectQuery("name: CharacterReservations").WithArgs("k", "g", pgxmock.AnyArg(), pgxmock.AnyArg(), int32(5)).
		WillReturnRows(pgxmock.NewRows(cols).
			AddRow(int64(1), int64(2), "Hero", "K/L", pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: start.Add(time.Hour), Valid: true}, pgtype.Text{String: "ok", Valid: true}, pgtype.Int8{Int64: 42, Valid: true}).
			AddRow(int64(3), int64(2), "Hero", "K", pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Text{}, pgtype.Int8{}))

	// when
	rows, err := repo.CharacterReservations(context.Background(), stats.Filter{GuildID: "g", CharacterKey: "k"}, 5)

	// then
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, experience.StatusOK, rows[0].Status)
	require.NotNil(t, rows[0].Gain)
	assert.Equal(t, int64(42), *rows[0].Gain)
	assert.Equal(t, start.Add(time.Hour), rows[0].EndAt)
	assert.Equal(t, experience.Status(""), rows[1].Status)
	assert.Nil(t, rows[1].Gain)
}
