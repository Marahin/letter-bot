package statshttp

import (
	"context"
	"html"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/web/webtest"
	"spot-assistant/internal/ports"
)

func visitor(t *testing.T) (http.Handler, webtest.Mocks) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	return webtest.Handler(d, Register), m
}

func worldRowsFixture() []stats.WorldRow {
	return []stats.WorldRow{
		{Key: "celesta", Name: "Celesta", Guilds: 2, Totals: withLoot(tot(10, 20*3600, 2, 4*3600, 8_000_000), 1, 3600, 250_000)},
		{Key: "antica", Name: "Antica", Guilds: 1, Totals: tot(1, 3600, 0, 0, 0)},
	}
}

func TestHandleWorlds_AnonymousSeesTheCards(t *testing.T) {
	// given
	h, m := visitor(t)
	m.Worlds.EXPECT().Worlds(mock.Anything, mock.Anything).Return(worldRowsFixture(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/worlds", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"<title>Worlds - TibiaLoot.com</title>", `href="/worlds/Celesta"`, `href="/worlds/Antica"`,
		"2 servers", "1 server", "2,000,000", "250,000", "No data", "/world-set", `href="/login"`, "data-range-picker",
	} {
		assert.Contains(t, body, want)
	}
	assert.Regexp(t, `href="/worlds"\s+aria-current="page"`, body)
	assert.NotContains(t, body, "<aside")
	assert.NotContains(t, body, "days with data", "the picker marks no data days here")
}

func TestHandleWorlds_SignedInKeepsTheSidebar(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Worlds.EXPECT().Worlds(mock.Anything, mock.Anything).Return(nil, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/worlds", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<aside")
	assert.Regexp(t, `href="/worlds"\s+aria-current="page"`, body)
	assert.Contains(t, body, "No world yet.")
	assert.Contains(t, body, `href="/tools/loot-calculator"`)
}

func TestHandleWorlds_Error(t *testing.T) {
	// given
	h, m := visitor(t)
	m.Worlds.EXPECT().Worlds(mock.Anything, mock.Anything).Return(nil, errBoom)

	// when
	rec := webtest.Serve(h, webtest.Get("/worlds", nil))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleWorlds_RangeHasItsOwnCookie(t *testing.T) {
	// given
	h, m := visitor(t)
	var got stats.Range
	m.Worlds.EXPECT().Worlds(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, rng stats.Range) ([]stats.WorldRow, error) {
		got = rng
		return nil, nil
	})

	// when
	rec := webtest.Serve(h, webtest.Get("/worlds?days=2026-09-01,2026-09-03", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, got.Days, 3)
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	names := map[string]string{}
	for _, c := range res.Cookies() {
		names[c.Name] = c.Value
	}
	assert.Equal(t, "2026-09-01,2026-09-03", names["letter_range_worlds"])
	for name := range names {
		assert.False(t, strings.HasPrefix(name, "letter_range_") && name != "letter_range_worlds", name)
	}
}

func TestHandleWorld(t *testing.T) {
	t.Run("renders the boards and the servers", func(t *testing.T) {
		// given
		h, m := visitor(t)
		m.Worlds.EXPECT().World(mock.Anything, "Celesta", mock.Anything).RunAndReturn(func(_ context.Context, _ string, rng stats.Range) (*stats.WorldDetail, error) {
			return &stats.WorldDetail{
				Key: "celesta", Name: "Celesta", Range: rng, Totals: withLoot(tot(5, 5*3600, 1, 3600, 100), 1, 3600, 10_000),
				Guilds:                []stats.WorldGuildRow{{GuildID: "g1", Name: "Celesta Community", Icon: "abc", Totals: tot(5, 5*3600, 0, 0, 0)}},
				SpawnsByProfitPerHour: []stats.SpawnRow{{Key: "hero cave", Name: "Hero Cave", Totals: withLoot(tot(3, 3*3600, 0, 0, 0), 1, 3*3600, 900_000)}},
				CharactersByProfit:    []stats.WorldCharacterRow{{GuildID: "g1", CharacterRow: stats.CharacterRow{Key: "quiet nyx", Name: "Quiet Nyx", Totals: withLoot(tot(1, 3600, 0, 0, 0), 1, 3600, 5_000)}}},
			}, nil
		})

		// when
		rec := webtest.Serve(h, webtest.Get("/worlds/Celesta", nil))

		// then
		require.Equal(t, http.StatusOK, rec.Code)
		body := rec.Body.String()
		for _, want := range []string{
			"<title>Celesta - Worlds - TibiaLoot.com</title>", "1 Discord server books respawns with Letter on this world.",
			`href="/servers/g1/stats"`, "https://cdn.discordapp.com/icons/g1/abc.png?size=64", "Discord servers on Celesta",
			`href="/spawns/hero%20cave"`, "300,000 gp/h", `href="/spawns?sort=profit_h&amp;world=celesta"`,
			`href="/servers/g1/characters/Quiet%20Nyx"`, "5,000 gp", `href="/worlds"`,
		} {
			assert.Contains(t, body, want)
		}
	})
	t.Run("unknown world", func(t *testing.T) {
		// given
		h, m := visitor(t)
		m.Worlds.EXPECT().World(mock.Anything, "Nowhere", mock.Anything).Return(nil, ports.ErrNotFound)

		// when
		rec := webtest.Serve(h, webtest.Get("/worlds/Nowhere", nil))

		// then
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
	t.Run("error", func(t *testing.T) {
		// given
		h, m := visitor(t)
		m.Worlds.EXPECT().World(mock.Anything, "Boom", mock.Anything).Return(nil, errBoom)

		// when
		rec := webtest.Serve(h, webtest.Get("/worlds/Boom", nil))

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func spawnsFixture() stats.Page[stats.SpawnRow] {
	return stats.Page[stats.SpawnRow]{Rows: []stats.SpawnRow{
		{Key: "hero cave", Name: "Hero Cave", Worlds: 2, Totals: withLoot(tot(4, 4*3600, 1, 3600, 2_000_000), 1, 1800, -50_000)},
		{Key: "soul war/bony sea", Name: "Soul War/Bony Sea", Worlds: 1, Totals: tot(1, 3600, 0, 0, 0)},
	}, Total: 2}
}

func TestHandleSpawns_PassesTheQueryAndKeepsTheFilter(t *testing.T) {
	// given
	h, m := visitor(t)
	m.Worlds.EXPECT().Spawns(mock.Anything, mock.Anything, "celesta", stats.Sort{Key: stats.SortProfitPerHour, Asc: true}, maxTableRows).Return(spawnsFixture(), nil)
	m.Worlds.EXPECT().PublicWorlds(mock.Anything).Return([]stats.World{{Key: "antica", Name: "Antica"}, {Key: "celesta", Name: "Celesta"}}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/spawns?world=Celesta&sort=profit_h&dir=asc", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"<title>Spawns on Celesta - TibiaLoot.com</title>", `<option value="celesta" selected>Celesta</option>`,
		`href="/spawns/hero%20cave"`, `href="/spawns/soul%20war%2Fbony%20sea"`, "-100,000", "text-rose-400",
		`aria-sort="ascending"><a href="/spawns?dir=desc&amp;sort=profit_h&amp;world=celesta"`,
		`href="/spawns?dir=asc&amp;format=csv&amp;sort=profit_h&amp;world=celesta"`,
		`name="world" value="celesta"`,
	} {
		assert.Contains(t, body, want)
	}
	assert.Regexp(t, `href="/spawns"\s+aria-current="page"`, body)
}

func TestHandleSpawns_DefaultsAndAWorldListFailure(t *testing.T) {
	// given
	h, m := visitor(t)
	m.Worlds.EXPECT().Spawns(mock.Anything, mock.Anything, "", stats.Sort{Key: stats.SortHours}, maxTableRows).Return(stats.Page[stats.SpawnRow]{}, nil)
	m.Worlds.EXPECT().PublicWorlds(mock.Anything).Return(nil, errBoom)

	// when
	rec := webtest.Serve(h, webtest.Get("/spawns?sort=exp", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "No respawn was booked in this range.")
	assert.NotContains(t, body, `id="spawns-world"`)
}

func TestHandleSpawns_CSV(t *testing.T) {
	// given
	h, m := visitor(t)
	m.Worlds.EXPECT().Spawns(mock.Anything, mock.Anything, "celesta", stats.Sort{Key: stats.SortHours}, maxCSVRows).Return(spawnsFixture(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/spawns?format=csv&world=celesta&days=2026-09-01,2026-09-02", nil))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `spawns-2026-09-01_2026-09-02-celesta.csv`)
	assert.Equal(t, "Respawn,Worlds,Reservations,Hours,Exp/h,Profit/h\n"+
		"Hero Cave,2,4,4.00,2000000,-100000\n"+
		"Soul War/Bony Sea,1,1,1.00,,\n", rec.Body.String())
}

func TestHandleSpawns_Error(t *testing.T) {
	// given
	h, m := visitor(t)
	m.Worlds.EXPECT().Spawns(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(stats.Page[stats.SpawnRow]{}, errBoom)

	// when
	rec := webtest.Serve(h, webtest.Get("/spawns", nil))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSpawn(t *testing.T) {
	t.Run("renders per world and per server", func(t *testing.T) {
		// given
		h, m := visitor(t)
		m.Worlds.EXPECT().Spawn(mock.Anything, "soul war/bony sea", mock.Anything).RunAndReturn(func(_ context.Context, _ string, rng stats.Range) (*stats.SpawnDetail, error) {
			return &stats.SpawnDetail{
				Key: "soul war/bony sea", Name: "Soul War/Bony Sea", Range: rng, Totals: withLoot(tot(3, 3*3600, 0, 0, 0), 1, 3600, 70_000),
				Worlds: []stats.WorldRow{{Key: "celesta", Name: "Celesta", Guilds: 1, Totals: tot(3, 3*3600, 0, 0, 0)}},
				Spots:  []stats.SpawnSpotRow{{WorldKey: "celesta", World: "Celesta", GuildID: "g1", GuildName: "Celesta Community", SpotID: 7, SpotName: "Bony Sea", Totals: tot(3, 3*3600, 0, 0, 0)}},
			}, nil
		})

		// when
		rec := webtest.Serve(h, webtest.Get("/spawns/soul%20war%2Fbony%20sea", nil))

		// then
		require.Equal(t, http.StatusOK, rec.Code)
		body := html.UnescapeString(rec.Body.String())
		for _, want := range []string{
			"<title>Soul War/Bony Sea - Spawns - TibiaLoot.com</title>", "Booked on 1 world in this range.",
			"By world", `href="/worlds/Celesta"`, "By server", `href="/servers/g1/stats/spots/7"`, "Celesta · Bony Sea",
			"70,000 gp", `href="/spawns"`,
		} {
			assert.Contains(t, body, want)
		}
	})
	t.Run("unknown spawn", func(t *testing.T) {
		// given
		h, m := visitor(t)
		m.Worlds.EXPECT().Spawn(mock.Anything, "nope", mock.Anything).Return(nil, ports.ErrNotFound)

		// when
		rec := webtest.Serve(h, webtest.Get("/spawns/nope", nil))

		// then
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
	t.Run("error", func(t *testing.T) {
		// given
		h, m := visitor(t)
		m.Worlds.EXPECT().Spawn(mock.Anything, "boom", mock.Anything).Return(nil, errBoom)

		// when
		rec := webtest.Serve(h, webtest.Get("/spawns/boom", nil))

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestPublicPages_InPolish(t *testing.T) {
	// given
	h, m := visitor(t)
	m.Worlds.EXPECT().Worlds(mock.Anything, mock.Anything).Return(worldRowsFixture(), nil)
	m.Worlds.EXPECT().Spawns(mock.Anything, mock.Anything, "", mock.Anything, mock.Anything).Return(spawnsFixture(), nil)
	m.Worlds.EXPECT().PublicWorlds(mock.Anything).Return([]stats.World{{Key: "celesta", Name: "Celesta"}}, nil)

	// when
	worlds := webtest.Serve(h, webtest.Get("/worlds?lang=pl", nil))
	spawns := webtest.Serve(h, webtest.Get("/spawns?lang=pl", nil))

	// then
	require.Equal(t, http.StatusOK, worlds.Code)
	require.Equal(t, http.StatusOK, spawns.Code)
	assert.Contains(t, worlds.Body.String(), "2 serwery")
	assert.Contains(t, worlds.Body.String(), "Otwórz statystyki świata")
	assert.Contains(t, spawns.Body.String(), "Wszystkie światy")
	assert.Contains(t, spawns.Body.String(), "Zysk/h")
}
