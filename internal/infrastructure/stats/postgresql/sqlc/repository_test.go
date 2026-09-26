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
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	return mock, NewStatsRepository(mock, berlin)
}

func anyArgs(n int) []any {
	args := make([]any, n)
	for i := range args {
		args[i] = pgxmock.AnyArg()
	}
	return args
}

var totalsCols = []string{"reservations", "seconds", "exp_reservations", "exp_seconds", "exp"}

func TestNewStatsRepository_FallsBackToBerlinForAnUnnamedZone(t *testing.T) {
	assert.Equal(t, "Europe/Berlin", NewStatsRepository(nil, time.Local).tz)
	assert.Equal(t, "UTC", NewStatsRepository(nil, time.UTC).tz)
}

func TestStatsRepository_MapsTotalsRows(t *testing.T) {
	// given
	mock, repo := newMock(t)
	ctx := context.Background()
	f := stats.Filter{GuildID: "g", From: time.Now(), To: time.Now(), SpotID: 3, UserID: "u", CharacterKey: "k"}
	mock.ExpectQuery("name: SpotTotals").WithArgs("g", pgxmock.AnyArg(), pgxmock.AnyArg(), int64(3), "u", "k").
		WillReturnRows(pgxmock.NewRows(append([]string{"spot_id", "name", "archived"}, totalsCols...)).AddRow(int64(3), "Hero", true, int64(1), int64(3600), int64(1), int64(3600), int64(9)))
	mock.ExpectQuery("name: PlayerTotals").WithArgs(anyArgs(6)...).
		WillReturnRows(pgxmock.NewRows(append([]string{"user_id", "name"}, totalsCols...)).AddRow("u", "A/B", int64(2), int64(7200), int64(0), int64(0), int64(0)))
	mock.ExpectQuery("name: CharacterTotals").WithArgs(anyArgs(6)...).
		WillReturnRows(pgxmock.NewRows(append([]string{"character_key", "name"}, totalsCols...)).AddRow("a", "A", int64(1), int64(60), int64(1), int64(60), int64(-5)))
	mock.ExpectQuery("name: Daily").WithArgs(append([]any{"Europe/Berlin"}, anyArgs(6)...)...).
		WillReturnRows(pgxmock.NewRows(append([]string{"day"}, totalsCols...)).AddRow(pgtype.Date{Time: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), Valid: true}, int64(1), int64(60), int64(0), int64(0), int64(0)))

	// when
	spots, err1 := repo.SpotTotals(ctx, f)
	players, err2 := repo.PlayerTotals(ctx, f)
	characters, err3 := repo.CharacterTotals(ctx, f)
	days, err4 := repo.Daily(ctx, f)

	// then
	require.NoError(t, errors.Join(err1, err2, err3, err4))
	assert.Equal(t, []stats.SpotRow{{SpotID: 3, Name: "Hero", Archived: true, Totals: stats.Totals{Reservations: 1, Seconds: 3600, ExpReservations: 1, ExpSeconds: 3600, Exp: 9}}}, spots)
	assert.Equal(t, "A/B", players[0].Name)
	assert.Equal(t, int64(7200), players[0].Seconds)
	assert.Equal(t, int64(-5), characters[0].Exp)
	require.Len(t, days, 1)
	assert.Equal(t, "2026-09-01 00:00:00 +0200 CEST", days[0].Day.String())
}

func TestStatsRepository_PropagatesQueryErrors(t *testing.T) {
	// given
	mock, repo := newMock(t)
	ctx := context.Background()
	for _, q := range []struct {
		name string
		args int
	}{{"SpotTotals", 6}, {"PlayerTotals", 6}, {"CharacterTotals", 6}, {"Daily", 7}, {"ReservationDays", 4}, {"CharacterReservations", 5}} {
		mock.ExpectQuery("name: " + q.name + " :").WithArgs(anyArgs(q.args)...).WillReturnError(errBoom)
	}

	// when
	_, err1 := repo.SpotTotals(ctx, stats.Filter{})
	_, err2 := repo.PlayerTotals(ctx, stats.Filter{})
	_, err3 := repo.CharacterTotals(ctx, stats.Filter{})
	_, err4 := repo.Daily(ctx, stats.Filter{})
	_, err5 := repo.ReservationDays(ctx, "g", time.Now(), time.Now())
	_, err6 := repo.CharacterReservations(ctx, stats.Filter{}, 5)

	// then
	for _, err := range []error{err1, err2, err3, err4, err5, err6} {
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
