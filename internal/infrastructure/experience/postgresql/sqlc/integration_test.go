package sqlc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/world"
	coreexperience "spot-assistant/internal/core/experience"
	"spot-assistant/internal/infrastructure/experience/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/scheduler"
	"spot-assistant/internal/infrastructure/worldapi"
)

// fakeTibiaData serves /highscores/{world}/experience/all/{page} from a mutable board, padded with
// untracked characters to two pages.
type fakeTibiaData struct {
	mu      sync.Mutex
	scraped time.Time
	board   []world.HighscoreEntry
	fail    bool
}

func (f *fakeTibiaData) set(scraped time.Time, board ...world.HighscoreEntry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := 0; i < fillers; i++ {
		board = append(board, hs(fmt.Sprintf("Filler %d", i), 1000))
	}
	f.scraped, f.board = scraped, board
}

const fillers = 55

func (f *fakeTibiaData) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	page, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/v4/highscores/Itworld/experience/all/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	perPage := coreexperience.PageSize
	total := (len(f.board) + perPage - 1) / perPage
	from, to := (page-1)*perPage, min(page*perPage, len(f.board))
	list := []map[string]any{}
	for _, e := range f.board[from:to] {
		list = append(list, map[string]any{"name": e.Name, "level": e.Level, "value": e.Value, "vocation": e.Vocation, "world": "Itworld"})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"highscores": map[string]any{
			"world": "Itworld", "category": "experience", "highscore_age": 5, "highscore_list": list,
			"highscore_page": map[string]any{"current_page": page, "total_pages": total, "total_records": len(f.board)},
		},
		"information": map[string]any{"timestamp": f.scraped},
	})
}

func hs(name string, exp int64) world.HighscoreEntry {
	return world.HighscoreEntry{Name: name, Level: int(exp / 100), Value: exp, Vocation: "Master Sorcerer"}
}

// TestExperienceJob_EndToEnd runs the job against a real database. Set LETTER_TEST_DATABASE_URL to run it, e.g.
// postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable with all migrations applied.
func TestExperienceJob_EndToEnd(t *testing.T) {
	dsn := os.Getenv("LETTER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LETTER_TEST_DATABASE_URL not set")
	}

	// given
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}
	cleanup := func() {
		exec(`DELETE FROM web_reservation WHERE guild_id LIKE 'it-exp-%'`)
		exec(`DELETE FROM web_spot WHERE guild_id LIKE 'it-exp-%'`)
		exec(`DELETE FROM guilds_world WHERE guild_id LIKE 'it-exp-%'`)
		exec(`DELETE FROM guilds WHERE guild_id LIKE 'it-exp-%'`)
		exec(`DELETE FROM highscore_snapshots WHERE world IN ('Itworld', 'Itworld-np')`)
		exec(`DELETE FROM highscore_runs WHERE world IN ('Itworld', 'Itworld-np')`)
	}
	cleanup()
	t.Cleanup(cleanup)

	// In the past: the adapter uses a scrape time only when it is not after the real clock.
	base := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Minute)
	start, end := base.Add(10*time.Minute), base.Add(70*time.Minute)
	exec(`INSERT INTO guilds (guild_id, name, premium) VALUES ('it-exp-p', 'P', true), ('it-exp-np', 'NP', false), ('it-exp-np2', 'NP2', false)`)
	exec(`INSERT INTO guilds_world (guild_id, world_name) VALUES ('it-exp-p', 'Itworld'), ('it-exp-np', 'Itworld'), ('it-exp-np2', 'Itworld-np')`)
	exec(`INSERT INTO web_spot (name, created_at, guild_id) VALUES ('It Spot', now(), 'it-exp-p'), ('It Spot', now(), 'it-exp-np')`)
	var mainID, nobodyID, otherID int64
	insert := func(guild, author string, s, e time.Time) int64 {
		var id int64
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO web_reservation (author, created_at, start_at, end_at, spot_id, guild_id, author_discord_id)
			VALUES ($1, now(), $2, $3, (SELECT id FROM web_spot WHERE guild_id = $4), $4, 'd1') RETURNING id`, author, s, e, guild).Scan(&id))
		return id
	}
	mainID = insert("it-exp-p", "Quiet Nyx/Storm Quiet/ dark quiet", start, end)
	nobodyID = insert("it-exp-p", "Nobody Tracked", start.Add(time.Minute), end.Add(time.Minute))
	otherID = insert("it-exp-np", "Other Guy", start, end)

	fake := &fakeTibiaData{}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	repo := sqlc.NewExperienceRepository(pool)
	job := coreexperience.New(worldapi.NewHTTPWorldService(server.URL+"/v4"), repo, 15*time.Minute, nil)
	cycle := func(now time.Time) {
		t.Helper()
		require.NoError(t, job.Collect(ctx, "Itworld", now))
		require.NoError(t, job.Attribute(ctx, "Itworld", now))
	}

	// when
	worlds, err := repo.ListTrackedWorlds(ctx)
	require.NoError(t, err)
	fake.set(base, hs("Quiet Nyx", 100_000), hs("Untracked", 90_000), hs("Storm Quiet", 80_000), hs("Other Guy", 70_000), hs("Dark Quiet", 60_000))
	cycle(base.Add(time.Minute))
	fake.set(base.Add(30*time.Minute), hs("Quiet Nyx", 150_000), hs("Untracked", 95_000), hs("Storm Quiet", 80_000), hs("Other Guy", 75_000), hs("Dark Quiet", 60_000))
	cycle(base.Add(31 * time.Minute))
	fake.set(base.Add(60*time.Minute), hs("Quiet Nyx", 150_000), hs("Untracked", 95_000), hs("Storm Quiet", 80_000), hs("Other Guy", 75_000), hs("Dark Quiet", 60_000))
	cycle(base.Add(61 * time.Minute))
	fake.mu.Lock()
	fake.fail = true
	fake.mu.Unlock()
	failedErr := job.Collect(ctx, "Itworld", base.Add(70*time.Minute))
	fake.mu.Lock()
	fake.fail = false
	fake.mu.Unlock()
	// Dark Quiet dropped out of the top 1000. The cut-off run misses it, so attribution waits for the next run.
	fake.set(base.Add(80*time.Minute), hs("Quiet Nyx", 230_000), hs("Untracked", 99_000), hs("Storm Quiet", 90_000), hs("Other Guy", 80_000))
	cycle(base.Add(81 * time.Minute))
	var waiting int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM reservation_experience WHERE reservation_id = $1`, mainID).Scan(&waiting))
	fake.set(base.Add(95*time.Minute), hs("Quiet Nyx", 230_000), hs("Untracked", 99_000), hs("Storm Quiet", 90_000), hs("Other Guy", 80_000))
	cycle(base.Add(96 * time.Minute))
	cycle(base.Add(97 * time.Minute))

	// then
	assert.Contains(t, worlds, "Itworld")
	assert.NotContains(t, worlds, "Itworld-np")
	assert.Error(t, failedErr)
	assert.Zero(t, waiting)

	type run struct {
		observed time.Time
		pages    int
		rows     int
	}
	var runs []run
	rows, err := pool.Query(ctx, `SELECT observed_at, pages, rows FROM highscore_runs WHERE world = 'Itworld' ORDER BY observed_at`)
	require.NoError(t, err)
	for rows.Next() {
		var r run
		require.NoError(t, rows.Scan(&r.observed, &r.pages, &r.rows))
		runs = append(runs, r)
	}
	require.NoError(t, rows.Err())
	require.Len(t, runs, 6, "the failed collect must not write a run")
	assert.True(t, runs[0].observed.Equal(base.Add(-5*time.Minute)), "observed_at = scrape time - highscore_age minutes")
	assert.Equal(t, 2, runs[0].pages)
	assert.Equal(t, 5+fillers, runs[0].rows)
	assert.Equal(t, 4+fillers, runs[3].rows)

	type snapshot struct {
		key      string
		exp      int64
		observed time.Time
		lastSeen time.Time
	}
	var snaps []snapshot
	rows, err = pool.Query(ctx, `SELECT character_key, experience, observed_at, last_seen_at FROM highscore_snapshots WHERE world = 'Itworld' ORDER BY character_key, observed_at`)
	require.NoError(t, err)
	for rows.Next() {
		var s snapshot
		require.NoError(t, rows.Scan(&s.key, &s.exp, &s.observed, &s.lastSeen))
		snaps = append(snaps, s)
	}
	require.NoError(t, rows.Err())
	got := map[string][]int64{}
	for _, s := range snaps {
		got[s.key] = append(got[s.key], s.exp)
	}
	assert.Equal(t, map[string][]int64{
		"quiet nyx":   {100_000, 150_000, 230_000},
		"storm quiet": {80_000, 90_000},
		"dark quiet":  {60_000},
	}, got, "only tracked characters, only when the value changed")
	for _, s := range snaps {
		if s.key == "dark quiet" {
			assert.True(t, s.lastSeen.Equal(base.Add(55*time.Minute)), "unchanged runs move last_seen_at")
		}
	}

	type gain struct {
		key    string
		start  *int64
		end    *int64
		gain   *int64
		status string
	}
	var gains []gain
	rows, err = pool.Query(ctx, `SELECT reservation_id, character_key, start_experience, end_experience, gain, status
		FROM reservation_experience WHERE reservation_id = ANY($1) ORDER BY reservation_id, character_key`, []int64{mainID, nobodyID, otherID})
	require.NoError(t, err)
	byReservation := map[int64][]gain{}
	for rows.Next() {
		var id int64
		var g gain
		require.NoError(t, rows.Scan(&id, &g.key, &g.start, &g.end, &g.gain, &g.status))
		byReservation[id] = append(byReservation[id], g)
		gains = append(gains, g)
	}
	require.NoError(t, rows.Err())
	i := func(v int64) *int64 { return &v }
	assert.Equal(t, []gain{
		{key: "dark quiet", start: i(60_000), status: "no_data"},
		{key: "quiet nyx", start: i(100_000), end: i(230_000), gain: i(130_000), status: "ok"},
		{key: "storm quiet", start: i(80_000), end: i(90_000), gain: i(10_000), status: "ok"},
	}, byReservation[mainID])
	assert.Equal(t, []gain{{key: "nobody tracked", status: "no_data"}}, byReservation[nobodyID])
	assert.Empty(t, byReservation[otherID], "non-premium guilds are not attributed")
	assert.Len(t, gains, 4, "the last cycle must not attribute twice")
}

func TestPgAdvisoryLock(t *testing.T) {
	dsn := os.Getenv("LETTER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("LETTER_TEST_DATABASE_URL not set")
	}

	// given
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	first := scheduler.NewPgAdvisoryLock(pool, 7419999, nil)
	second := scheduler.NewPgAdvisoryLock(pool, 7419999, nil)

	// when
	unlock, ok, err := first.TryLock(ctx)
	require.NoError(t, err)
	_, heldElsewhere, heldErr := second.TryLock(ctx)
	unlock()
	unlockAgain, okAfter, afterErr := second.TryLock(ctx)
	require.NoError(t, afterErr)
	unlockAgain()

	// then
	assert.True(t, ok)
	assert.NoError(t, heldErr)
	assert.False(t, heldElsewhere)
	assert.True(t, okAfter)
}
