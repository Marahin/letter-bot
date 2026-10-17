package statshttp

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/web/webtest"
)

func TestHandleOverview_ProfitTilesAndBoards(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	totals := tot(12, 30*3600, 4, 10*3600, 5_000_000)
	totals.LootReservations, totals.LootSeconds, totals.Profit = 3, 2*3600, 1_800_000
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).RunAndReturn(func(_ context.Context, _ string, rng stats.Range) (*stats.Overview, error) {
		return &stats.Overview{
			Range:  rng,
			Totals: totals,
			Daily:  days(rng, tot(1, 3600, 0, 0, 0)),
			Leaderboards: stats.Leaderboards{
				SpotsByProfitPerHour: []stats.SpotRow{{SpotID: 4, Name: "Hero Cave", Totals: withLoot(tot(4, 4*3600, 0, 0, 0), 2, 4*3600, 2_000_000)}},
				CharactersByProfit:   []stats.CharacterRow{{Key: "storm quiet", Name: "Storm Quiet", Totals: withLoot(tot(1, 3600, 0, 0, 0), 1, 3600, -20_000)}},
			},
		}, nil
	})

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"Profit per hour", "1,800,000 gp", "900,000 gp", "Sessions for 3 of 12 reservations", "Divided by session time, not booked time",
		"Best profit per hour", "Respawns with at least 3 h of sessions", "500,000 gp/h", `href="/servers/g1/stats/spots?sort=profit_h"`,
		"Most profit", "-20,000 gp", `href="/servers/g1/stats/characters?sort=profit"`,
	} {
		assert.Contains(t, body, want)
	}
}

func TestHandleOverview_WithoutLootShowsNoDataNotZero(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).RunAndReturn(func(_ context.Context, _ string, rng stats.Range) (*stats.Overview, error) {
		return &stats.Overview{Range: rng, Totals: tot(3, 3*3600, 1, 3600, 10), Daily: days(rng, tot(0, 0, 0, 0, 0))}, nil
	})

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.NotContains(t, body, ">0 gp<")
	assert.Contains(t, body, "No reservation in this range has a loot session.")
	assert.Contains(t, body, "Profit comes from Party Hunt sessions pasted in the Loot Calculator")
}

func TestHandleSpots_SortByProfitPerHour(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Spots(mock.Anything, guildID, mock.Anything, stats.Sort{Key: stats.SortProfitPerHour}, maxTableRows).Return(spotRowsFixture(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots?sort=profit_h", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Regexp(t, `aria-sort="descending"><a href="/servers/g1/stats/spots\?dir=asc&amp;sort=profit_h"`, body)
	assert.Contains(t, body, "-90,000")
}

func TestHandleSpot_LootTiles(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	totals := withLoot(tot(2, 7200, 0, 0, 0), 1, 3600, 300_000)
	totals.Loot, totals.Supplies = 900_000, 600_000
	m.Stats.EXPECT().Spot(mock.Anything, guildID, int64(4), mock.Anything).RunAndReturn(func(_ context.Context, _ string, _ int64, rng stats.Range) (*stats.SpotDetail, error) {
		return &stats.SpotDetail{Spot: &spot.Spot{ID: 4, Name: "Hero Cave"}, Range: rng, Totals: totals, Daily: days(rng, tot(0, 0, 0, 0, 0))}, nil
	})

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats/spots/4", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "900,000 gp")
	assert.Contains(t, body, "600,000 gp")
	assert.Contains(t, body, "300,000 gp")
	assert.Contains(t, body, "Supplies")
}

func TestHandleCharacter_ExplainsTheShare(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Characters.EXPECT().Profile(mock.Anything, guildID, "Quiet Nyx", mock.Anything).Return(&stats.CharacterProfile{Name: "Quiet Nyx", Source: stats.ProfileUnavailable}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/characters/Quiet%20Nyx", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "equal share of each party session")
}

func TestHandleOverview_EmptyRangeSaysHowDataArrives(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, true)
	m.Stats.EXPECT().Overview(mock.Anything, guildID, mock.Anything).Return(&stats.Overview{}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/stats?lang=pl", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Rezerwuj respy z Letterem")
	assert.Contains(t, body, `href="/tools/loot-calculator"`)
	assert.True(t, strings.Contains(body, "Zysk na godzinę"))
}
