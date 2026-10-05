package statshttp

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/character"
	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/core/permission"
	corestats "spot-assistant/internal/core/stats"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/infrastructure/web/webtest"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

var errBoom = errors.New("boom")

func signedIn(t *testing.T, premium bool) (http.Handler, webtest.Mocks, *http.Cookie) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(&access.GuildAccess{
		Config: guildconfig.Config{GuildID: guildID, Name: "Celesta Community", Premium: premium},
		Caps:   permission.Capabilities{View: true},
	}, nil)
	m.Stats.EXPECT().DataDays(mock.Anything, guildID, mock.Anything).Return([]time.Time{time.Now()}, nil).Maybe()
	return h, m, cookie
}

func tot(res, secs, expRes, expSecs, exp int64) stats.Totals {
	return stats.Totals{Reservations: res, Seconds: secs, ExpReservations: expRes, ExpSeconds: expSecs, Exp: exp}
}

func days(rng stats.Range, t stats.Totals) []stats.Day {
	out := make([]stats.Day, len(rng.Days))
	for i, d := range rng.Days {
		out[i] = stats.Day{Day: d, Totals: t}
	}
	return out
}

func TestHandleOverview_RendersTilesChartsAndLeaderboards(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).RunAndReturn(func(_ context.Context, _ string, rng stats.Range) (*stats.Overview, error) {
		return &stats.Overview{
			Range:    rng,
			Totals:   tot(12, 30*3600, 4, 10*3600, 5_000_000),
			Spots:    2,
			Players:  3,
			Daily:    days(rng, tot(1, 3600, 1, 3600, 100)),
			TopSpots: []stats.SpotRow{{SpotID: 4, Name: "Hero Cave", Totals: tot(8, 20*3600, 0, 0, 0)}},
			Leaderboards: stats.Leaderboards{
				PlayersByHours:  []stats.PlayerRow{{UserID: "u9", Name: "Quiet Nyx/Storm", Totals: tot(5, 12*3600, 0, 0, 0)}},
				CharactersByExp: []stats.CharacterRow{{Key: "quiet nyx", Name: "Quiet Nyx", Totals: tot(2, 7200, 2, 7200, 4_000_000)}},
			},
		}, nil
	})

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "5,000,000")
	assert.Contains(t, body, "500,000", "experience per hour over the 10 h with data")
	assert.Contains(t, body, "Data for 4 of 12 reservations")
	assert.Contains(t, body, "Hero Cave")
	assert.Contains(t, body, `href="/servers/g1/stats/players/u9"`)
	assert.Contains(t, body, `href="/servers/g1/characters/Quiet%20Nyx"`)
	assert.Contains(t, body, "data-range-picker")
	assert.Contains(t, body, "data-chart-tip")
	assert.Contains(t, body, `href="/servers/g1/stats/spots/4"`, "the busiest respawns link their page")
	// The default range ends today, which is not over yet.
	assert.Contains(t, body, "data-chart-partial")
	assert.Contains(t, body, "(today so far)")
	assert.Contains(t, body, "A dashed line ends on today")
	// The exp/h leaderboard is empty: it must read "no data", not list zeros.
	assert.Contains(t, body, "No reservation in this range has experience data")
}

func TestHandleOverview_WithoutExperienceDataNeverShowsZero(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).RunAndReturn(func(_ context.Context, _ string, rng stats.Range) (*stats.Overview, error) {
		return &stats.Overview{Range: rng, Totals: tot(3, 3*3600, 0, 0, 0), Daily: days(rng, tot(0, 0, 0, 0, 0))}, nil
	})

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats?lang=pl", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.GreaterOrEqual(t, strings.Count(body, "Brak danych"), 3)
	assert.Contains(t, body, "top 1000")
}

func TestHandleOverview_EmptyRange(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).Return(&stats.Overview{}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "No reservations in this range.")
}

func TestHandleOverview_PinnedRangeIsResolvedAndPersisted(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	var got stats.Range
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).RunAndReturn(func(_ context.Context, _ string, rng stats.Range) (*stats.Overview, error) {
		got = rng
		return &stats.Overview{Range: rng}, nil
	})

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats?days=2026-09-05,2026-09-01,2026-09-03", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, got.Days, 5)
	assert.Equal(t, "2026-09-01", got.From.Format(time.DateOnly))
	var persisted string
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	for _, c := range res.Cookies() {
		if c.Name == "letter_range_"+guildID {
			persisted = c.Value
		}
	}
	assert.Contains(t, persisted, "2026-09-01")
	assert.Contains(t, persisted, "2026-09-05")
	assert.NotContains(t, persisted, "2026-09-03", "only the ends of the span are stored")
	assert.Contains(t, rec.Body.String(), "01 Sep – 05 Sep 2026")
}

func TestHandleOverview_NonPremiumAndErrors(t *testing.T) {
	t.Run("non-premium", func(t *testing.T) {
		// given
		h, _, cookie := signedIn(t, false)

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

		// then
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
	t.Run("service error", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).Return(nil, errBoom)

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleOverview_DataDaysFailureStillRenders(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(&access.GuildAccess{
		Config: guildconfig.Config{GuildID: guildID, Premium: true}, Caps: permission.Capabilities{View: true},
	}, nil)
	m.Stats.EXPECT().DataDays(mock.Anything, guildID, mock.Anything).Return(nil, errBoom)
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).Return(&stats.Overview{}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
}

func spotRowsFixture() stats.Page[stats.SpotRow] {
	return stats.Page[stats.SpotRow]{Rows: []stats.SpotRow{
		{SpotID: 4, Name: "Hero Cave", Totals: tot(3, 3*3600, 1, 3600, 1_200_000)},
		{SpotID: 5, Name: "Old Spot", Archived: true, Totals: tot(1, 1800, 0, 0, 0)},
		{SpotID: 6, Name: "=HYPERLINK(\"x\")", Totals: tot(1, 3600, 0, 0, 0)},
	}, Total: 3}
}

func TestHandleSpots_SortsOnTheServer(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Spots(mock.Anything, guildID, mock.Anything, stats.Sort{Key: stats.SortExp}, maxTableRows).Return(spotRowsFixture(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots?sort=exp", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `aria-sort="descending"`)
	assert.Contains(t, body, `href="/servers/g1/stats/spots?dir=asc&amp;sort=exp"`)
	assert.Contains(t, body, `href="/servers/g1/stats/spots/4"`)
	assert.Contains(t, body, "1,200,000")
	assert.Contains(t, body, "Archived")
	assert.Contains(t, body, "No data")
	assert.Contains(t, body, `name="sort" value="exp"`)
	assert.Contains(t, body, "format=csv")
}

func TestHandleSpots_CSVExportsUpToTheCapWithEmptyNoDataCells(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Spots(mock.Anything, guildID, mock.Anything, mock.Anything, maxCSVRows).Return(spotRowsFixture(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots?format=csv&days=2026-09-01,2026-09-02", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "stats-spots-2026-09-01_2026-09-02.csv")
	assert.Equal(t, "Respawn,Reservations,Hours,Experience,Exp/h\nHero Cave,3,3.00,1200000,1200000\nOld Spot,1,0.50,,\n\"'=HYPERLINK(\"\"x\"\")\",1,1.00,,\n", rec.Body.String())
}

func TestHandlePlayersAndCharacters_ListsAndTruncates(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Players(mock.Anything, guildID, mock.Anything, stats.Sort{Key: stats.SortName, Asc: true}, maxTableRows).
		Return(stats.Page[stats.PlayerRow]{Rows: []stats.PlayerRow{{UserID: "u2", Name: "Storm Quiet"}}, Total: 1}, nil)
	many := make([]stats.CharacterRow, maxTableRows)
	for i := range many {
		many[i] = stats.CharacterRow{Key: "c", Name: "Char"}
	}
	m.Stats.EXPECT().Characters(mock.Anything, guildID, mock.Anything, mock.Anything, maxTableRows).Return(stats.Page[stats.CharacterRow]{Rows: many, Total: 501}, nil)

	// when
	players := webtest.Serve(h, webtest.Get("/servers/g1/stats/players?sort=name", cookie))
	characters := webtest.Serve(h, webtest.Get("/servers/g1/stats/characters", cookie))

	// then
	require.Equal(t, http.StatusOK, players.Code)
	assert.Contains(t, players.Body.String(), `href="/servers/g1/stats/players/u2"`)
	require.Equal(t, http.StatusOK, characters.Code)
	assert.Contains(t, characters.Body.String(), "Shows 500 of 501 rows")
}

func TestHandleTable_EmptyAndError(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Characters(mock.Anything, guildID, mock.Anything, mock.Anything, mock.Anything).Return(stats.Page[stats.CharacterRow]{}, nil)
	m.Stats.EXPECT().Players(mock.Anything, guildID, mock.Anything, mock.Anything, mock.Anything).Return(stats.Page[stats.PlayerRow]{}, errBoom)

	// when
	empty := webtest.Serve(h, webtest.Get("/servers/g1/stats/characters", cookie))
	failed := webtest.Serve(h, webtest.Get("/servers/g1/stats/players", cookie))

	// then
	assert.Equal(t, http.StatusOK, empty.Code)
	assert.NotContains(t, empty.Body.String(), "format=csv")
	assert.Equal(t, http.StatusInternalServerError, failed.Code)
}

func TestHandleSpot(t *testing.T) {
	t.Run("renders", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		archived := time.Now()
		m.Stats.EXPECT().Spot(mock.Anything, guildID, int64(4), mock.Anything).RunAndReturn(func(_ context.Context, _ string, _ int64, rng stats.Range) (*stats.SpotDetail, error) {
			players := stats.Page[stats.PlayerRow]{Rows: make([]stats.PlayerRow, 25), Total: 27}
			for i := range players.Rows {
				players.Rows[i] = stats.PlayerRow{UserID: "u", Name: "P"}
			}
			return &stats.SpotDetail{
				Spot: &spot.Spot{ID: 4, Name: "Hero Cave", ArchivedAt: &archived}, Range: rng,
				Totals: tot(2, 7200, 0, 0, 0), Daily: days(rng, tot(0, 0, 0, 0, 0)), Players: players,
			}, nil
		})

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots/4", cookie))

		// then
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "Hero Cave")
		assert.Contains(t, body, `href="/servers/g1/spots/4"`)
		assert.Contains(t, body, "Shows 25 of 27 rows")
	})
	t.Run("foreign spot", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Stats.EXPECT().Spot(mock.Anything, guildID, int64(9), mock.Anything).Return(nil, ports.ErrNotFound)

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots/9", cookie))

		// then
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
	t.Run("bad id and error", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Stats.EXPECT().Spot(mock.Anything, guildID, int64(5), mock.Anything).Return(nil, errBoom)

		// when
		bad := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots/abc", cookie))
		failed := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots/5", cookie))

		// then
		assert.Equal(t, http.StatusBadRequest, bad.Code)
		assert.Equal(t, http.StatusInternalServerError, failed.Code)
	})
}

func TestHandlePlayer(t *testing.T) {
	t.Run("renders", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Stats.EXPECT().Player(mock.Anything, guildID, "u2", mock.Anything).RunAndReturn(func(_ context.Context, _, _ string, rng stats.Range) (*stats.PlayerDetail, error) {
			return &stats.PlayerDetail{
				UserID: "u2", Name: "Quiet Nyx/Storm Quiet", Range: rng, Totals: tot(1, 3600, 1, 3600, 10),
				Daily:      days(rng, tot(1, 3600, 1, 3600, 10)),
				Spots:      stats.Page[stats.SpotRow]{Rows: []stats.SpotRow{{SpotID: 4, Name: "Hero Cave", Totals: tot(1, 3600, 1, 3600, 10)}}, Total: 1},
				Characters: stats.Page[stats.CharacterRow]{Rows: []stats.CharacterRow{{Key: "storm quiet", Name: "Storm Quiet"}}, Total: 1},
			}, nil
		})

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/players/u2", cookie))

		// then
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "Quiet Nyx/Storm Quiet")
		assert.Contains(t, body, `href="/servers/g1/characters/Storm%20Quiet"`)
	})
	t.Run("unknown and error", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Stats.EXPECT().Player(mock.Anything, guildID, "x", mock.Anything).Return(nil, ports.ErrNotFound)
		m.Stats.EXPECT().Player(mock.Anything, guildID, "y", mock.Anything).Return(nil, errBoom)

		// when
		unknown := webtest.Serve(h, webtest.Get("/servers/g1/stats/players/x", cookie))
		failed := webtest.Serve(h, webtest.Get("/servers/g1/stats/players/y", cookie))

		// then
		assert.Equal(t, http.StatusNotFound, unknown.Code)
		assert.Equal(t, http.StatusInternalServerError, failed.Code)
	})
}

func TestHandleCharacter(t *testing.T) {
	t.Run("renders the profile, history and reservations", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		login := time.Now().Add(-2 * time.Hour)
		gain := int64(1_500_000)
		m.Characters.EXPECT().Profile(mock.Anything, guildID, "Quiet Nyx", mock.Anything).RunAndReturn(func(_ context.Context, _, _ string, rng stats.Range) (*stats.CharacterProfile, error) {
			return &stats.CharacterProfile{
				Key: "quiet nyx", Name: "Quiet Nyx", World: "Celesta", Source: stats.ProfileFound, Range: rng,
				Character: &character.Character{Name: "Quiet Nyx", Level: 612, Vocation: "Elite Knight", World: "Celesta", GuildName: "Refugees", GuildRank: "Veteran", LastLogin: &login, AccountStatus: "Premium Account"},
				History:   []stats.HistoryPoint{{Day: rng.Days[0], Level: 611, Experience: 3_700_000_000}, {Day: rng.Days[1], Level: 612, Experience: 3_750_000_000}},
				Totals:    tot(2, 7200, 1, 3600, gain),
				Daily:     days(rng, tot(0, 0, 0, 0, 0)),
				Spots:     stats.Page[stats.SpotRow]{Rows: []stats.SpotRow{{SpotID: 4, Name: "Hero Cave", Totals: tot(2, 7200, 1, 3600, gain)}}, Total: 1},
				Recent: []stats.CharacterReservation{
					{ID: 1, SpotID: 4, SpotName: "Hero Cave", Author: "Quiet Nyx/Storm Quiet", StartAt: login, EndAt: login.Add(time.Hour), Status: experience.StatusOK, Gain: &gain},
					{ID: 2, SpotID: 4, SpotName: "Hero Cave", Author: "Quiet Nyx", StartAt: login, EndAt: login.Add(time.Hour), Status: experience.StatusNoData},
					{ID: 3, SpotID: 4, SpotName: "Hero Cave", Author: "Quiet Nyx", StartAt: login, EndAt: login.Add(time.Hour)},
				},
			}, nil
		})

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/characters/Quiet%20Nyx", cookie))

		// then
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "Elite Knight")
		assert.Contains(t, body, "Veteran of Refugees")
		assert.Contains(t, body, "2 hours ago")
		assert.Regexp(t, `>3\.\d\dB<`, body)
		assert.Contains(t, body, "Level 612 · 3,750,000,000 exp")
		assert.Contains(t, body, `href="/servers/g1/characters/Storm%20Quiet"`)
		assert.Contains(t, body, "1,500,000")
		assert.Contains(t, body, "Pending")
		assert.Contains(t, body, "No data")
	})
	t.Run("TibiaData down, no world, no reservations", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Characters.EXPECT().Profile(mock.Anything, guildID, "Ghost", mock.Anything).Return(&stats.CharacterProfile{Name: "Ghost", Source: stats.ProfileUnavailable}, nil)

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/characters/Ghost", cookie))

		// then
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		assert.Contains(t, body, "TibiaData does not answer now")
		assert.Contains(t, body, "This server has no Tibia world yet")
		assert.Contains(t, body, "This character has no reservations in this range.")
	})
	t.Run("deleted character with an empty history", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Characters.EXPECT().Profile(mock.Anything, guildID, "Old", mock.Anything).Return(&stats.CharacterProfile{Name: "Old", World: "Celesta", Source: stats.ProfileNotFound}, nil)

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/characters/Old", cookie))

		// then
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "TibiaData does not know a character with this name")
		assert.Contains(t, rec.Body.String(), "The highscores of Celesta have no value")
	})
	t.Run("unknown and error", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, true)
		m.Characters.EXPECT().Profile(mock.Anything, guildID, "Nobody", mock.Anything).Return(nil, ports.ErrNotFound)
		m.Characters.EXPECT().Profile(mock.Anything, guildID, "Boom", mock.Anything).Return(nil, errBoom)

		// when
		unknown := webtest.Serve(h, webtest.Get("/servers/g1/characters/Nobody", cookie))
		failed := webtest.Serve(h, webtest.Get("/servers/g1/characters/Boom", cookie))

		// then
		assert.Equal(t, http.StatusNotFound, unknown.Code)
		assert.Equal(t, http.StatusInternalServerError, failed.Code)
	})
}

func TestParseDays(t *testing.T) {
	cases := map[string]int{
		"days=2026-09-01,bogus,2026-09-03": 2,
		"from=2026-09-01&to=2026-09-10":    2,
		"to=2026-09-10":                    1,
		"days=nope":                        0,
		"":                                 0,
	}
	for query, want := range cases {
		// when
		got := parseDays(mustQuery(t, query))

		// then
		assert.Len(t, got, want, query)
	}
}

func TestRangeLabel(t *testing.T) {
	// given
	ctx := i18n.WithLocale(context.Background(), i18n.Normalize("en"))
	d := func(y int, m time.Month, dd int) time.Time { return time.Date(y, m, dd, 0, 0, 0, 0, time.Local) }

	// then
	assert.Equal(t, "", rangeLabel(ctx, stats.Range{}))
	assert.Equal(t, "01 Sep 2026", rangeLabel(ctx, corestats.ResolveRange([]time.Time{d(2026, 9, 1)}, d(2026, 9, 1))))
	assert.Equal(t, "30 Dec 2025 – 02 Jan 2026", rangeLabel(ctx, corestats.ResolveRange([]time.Time{d(2025, 12, 30), d(2026, 1, 2)}, d(2026, 1, 2))))
}

func TestAxisFormat_KeepsNeighbouringTicksDistinct(t *testing.T) {
	// given
	en := i18n.WithLocale(context.Background(), i18n.Normalize("en"))
	pl := i18n.WithLocale(context.Background(), i18n.Normalize("pl"))

	// then
	assert.Equal(t, "3.75B", axisFormat(en, []float64{3.5e9, 3.8e9})(3.75e9))
	assert.Equal(t, "1,2 mln", axisFormat(pl, []float64{0, 2e6})(1.2e6))
	assert.Equal(t, "15K", axisFormat(en, []float64{0, 40_000})(15_000))
	assert.Equal(t, "0.5", axisFormat(en, []float64{0, 1})(0.5))
	assert.Equal(t, "660", axisFormat(en, []float64{0, 661})(660))
	assert.Equal(t, "-1.0M", axisFormat(en, []float64{-2e6, 0})(-1e6))
}

func mustQuery(t *testing.T, raw string) url.Values {
	t.Helper()
	q, err := url.ParseQuery(raw)
	require.NoError(t, err)
	return q
}

func TestParseSort(t *testing.T) {
	cases := []struct {
		key, dir string
		want     stats.Sort
	}{
		{"", "", stats.Sort{Key: stats.SortHours}},
		{"bogus", "asc", stats.Sort{Key: stats.SortHours, Asc: true}},
		{"name", "", stats.Sort{Key: stats.SortName, Asc: true}},
		{"name", "desc", stats.Sort{Key: stats.SortName}},
		{"exp_h", "", stats.Sort{Key: stats.SortExpPerHour}},
		{"reservations", "sideways", stats.Sort{Key: stats.SortReservations}},
	}
	for _, c := range cases {
		// when
		got := parseSort(c.key, c.dir)

		// then
		assert.Equal(t, c.want, got, "%s/%s", c.key, c.dir)
	}
}

func TestCSVText_DefusesFormulas(t *testing.T) {
	for in, want := range map[string]string{
		"=1+1": "'=1+1", "+1": "'+1", "-1": "'-1", "@SUM(A1)": "'@SUM(A1)", "\tx": "'\tx", "\rx": "'\rx",
		"Quiet Nyx": "Quiet Nyx", "": "", "a=b": "a=b",
	} {
		assert.Equal(t, want, csvText(in), in)
	}
}

func publicGuild(premium bool) access.GuildAccess {
	return access.GuildAccess{Config: guildconfig.Config{GuildID: guildID, Name: "Celesta Community", BotPresent: true, Premium: premium}}
}

// anonymous serves the Stats routes to a signed-out visitor of g1.
func anonymous(t *testing.T, premium bool) (http.Handler, *web.Deps, webtest.Mocks) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	webtest.PublicGuild(m, publicGuild(premium))
	m.Stats.EXPECT().DataDays(mock.Anything, guildID, mock.Anything).Return(nil, nil).Maybe()
	return h, d, m
}

func emptyOverview(m webtest.Mocks) {
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).Return(&stats.Overview{}, nil)
}

func TestPublicStats_AnonymousSeesThePublicBar(t *testing.T) {
	// given
	h, _, m := anonymous(t, true)
	emptyOverview(m)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "data-public-stats")
	assert.Contains(t, body, "Public stats")
	assert.Contains(t, body, `href="/servers/g1/stats/players"`)
	assert.Regexp(t, `<a href="/servers/g1/stats" aria-current="page"`, body)
	assert.Contains(t, body, `data-login-cta`)
	assert.Contains(t, body, `href="/login?to=%2Fservers%2Fg1%2Fstats"`)
	assert.Contains(t, body, "A member of Celesta Community? Sign in")
	assert.NotContains(t, body, "<aside")
	assert.Contains(t, body, "data-topbar-menu")
}

func TestPublicStats_LockedServerShowsTheLock(t *testing.T) {
	// given
	h, _, _ := anonymous(t, false)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", nil))

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "Unlock the feature with Premium. Join the Discord and get on board!")
	assert.Contains(t, rec.Body.String(), "Back to home")
}

func TestPublicStats_UnknownServerIs404(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	m.Access.EXPECT().Public(mock.Anything, "nope").Return(nil, ports.ErrNotFound)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/nope/stats", nil))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPublicStats_PublicLookupErrorIs500(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	m.Access.EXPECT().Public(mock.Anything, guildID).Return(nil, errBoom)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", nil))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestPublicStats_SignedInVisitorWhoIsNotAMember(t *testing.T) {
	for name, accessErr := range map[string]error{
		"not a member":    ports.ErrNotFound,
		"discord is down": ports.ErrUpstreamUnavailable,
		"token refused":   ports.ErrUnauthorized,
	} {
		t.Run(name, func(t *testing.T) {
			// given a user whose own server is g2
			h, d, m := anonymous(t, true)
			d2 := access.GuildAccess{Config: guildconfig.Config{GuildID: "g2", Name: "Home Guild", Premium: true}, Caps: permission.Capabilities{View: true}}
			cookie := webtest.SignIn(t, d, m, "u1", d2)
			m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(nil, accessErr)
			emptyOverview(m)

			// when
			rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			body := rec.Body.String()
			assert.Contains(t, body, "data-public-stats")
			assert.NotContains(t, body, "data-login-cta", "a signed-in visitor needs no sign-in")
			assert.Contains(t, body, "<aside")
			assert.Contains(t, body, `href="/servers/g2/reservations"`)
			assert.NotContains(t, body, `href="/servers/g1/reservations"`)
		})
	}
}

func TestPublicStats_MemberGetsTheSidebarNotThePublicBar(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	emptyOverview(m)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, "data-public-stats")
	assert.Contains(t, body, `href="/servers/g1/reservations"`)
}

func TestPublicStats_StaleSessionGoesOnSignedOut(t *testing.T) {
	// given a session whose user row is gone
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Auth.ExpectedCalls = nil
	m.Auth.EXPECT().User(mock.Anything, "u1").Return(nil, ports.ErrNotFound)
	webtest.PublicGuild(m, publicGuild(true))
	m.Stats.EXPECT().DataDays(mock.Anything, guildID, mock.Anything).Return(nil, nil).Maybe()
	emptyOverview(m)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "data-login-cta")
}

func TestPublicStats_RespawnAndPlayerPages(t *testing.T) {
	// given
	h, _, m := anonymous(t, true)
	m.Stats.EXPECT().Spot(mock.Anything, guildID, int64(4), mock.Anything).RunAndReturn(func(_ context.Context, _ string, _ int64, rng stats.Range) (*stats.SpotDetail, error) {
		return &stats.SpotDetail{Spot: &spot.Spot{ID: 4, Name: "Hero Cave"}, Range: rng, Totals: tot(1, 3600, 0, 0, 0), Daily: days(rng, tot(0, 0, 0, 0, 0))}, nil
	})
	m.Stats.EXPECT().Player(mock.Anything, guildID, "u2", mock.Anything).RunAndReturn(func(_ context.Context, _, _ string, rng stats.Range) (*stats.PlayerDetail, error) {
		return &stats.PlayerDetail{UserID: "u2", Name: "Quiet Nyx", Range: rng}, nil
	})

	// when
	spotRec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots/4", nil))
	playerRec := webtest.Serve(h, webtest.Get("/servers/g1/stats/players/u2", nil))

	// then
	require.Equal(t, http.StatusOK, spotRec.Code)
	assert.Contains(t, spotRec.Body.String(), "data-public-stats")
	assert.Regexp(t, `<a href="/servers/g1/stats/spots" aria-current="page"`, spotRec.Body.String())
	assert.NotContains(t, spotRec.Body.String(), `href="/servers/g1/spots/4"`, "the reservations need sign-in")
	assert.Contains(t, spotRec.Body.String(), `href="/login?to=%2Fservers%2Fg1%2Fstats%2Fspots%2F4"`)
	require.Equal(t, http.StatusOK, playerRec.Code)
	assert.Contains(t, playerRec.Body.String(), "Quiet Nyx")
	assert.Contains(t, playerRec.Body.String(), "data-public-stats")
}

func TestPublicStats_CSVExport(t *testing.T) {
	// given
	h, _, m := anonymous(t, true)
	m.Stats.EXPECT().Spots(mock.Anything, guildID, mock.Anything, mock.Anything, maxCSVRows).Return(spotRowsFixture(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots?format=csv", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), "Hero Cave")
}

func TestPublicStats_InPolish(t *testing.T) {
	// given
	h, _, m := anonymous(t, true)
	emptyOverview(m)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats?lang=pl", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Statystyki publiczne")
	assert.Contains(t, body, "Należysz do Celesta Community? Zaloguj się")
	assert.Contains(t, body, "Zaloguj się przez Discorda")
}
