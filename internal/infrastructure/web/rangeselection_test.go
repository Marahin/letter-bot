package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rangeDeps() *Deps {
	return &Deps{Cfg: Config{BaseURL: "http://localhost:8080"}}
}

// runRange drives a request through the middleware and, inside the handler (where
// the selection is in context), captures it and persists the given days, so a
// single call exercises resolution and persistence together, as the real
// handlers do.
func runRange(d *Deps, r *http.Request, persist []time.Time) (RangeSelection, *httptest.ResponseRecorder) {
	var got RangeSelection
	rec := httptest.NewRecorder()
	d.WithRangeSelection(func(w http.ResponseWriter, req *http.Request) {
		got = RangeSelectionFrom(req.Context())
		d.PersistRangeSelection(w, req, "g1", persist)
	})(rec, r)
	return got, rec
}

func TestResolveRangeSelection_ExplicitQueryWinsAndPersists(t *testing.T) {
	d := rangeDeps()
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats?days=2026-06-18,2026-06-22", nil)

	sel, rec := runRange(d, r, []time.Time{time.Date(2026, 6, 18, 0, 0, 0, 0, time.Local)})

	assert.True(t, sel.Explicit)
	assert.Equal(t, "2026-06-18,2026-06-22", sel.Query.Get("days"))
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "2026-06-18", cookies[0].Value)
}

func TestResolveRangeSelection_FromToTreatedAsExplicit(t *testing.T) {
	d := rangeDeps()
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats?from=2026-06-10&to=2026-06-12", nil)

	sel, _ := runRange(d, r, nil)

	assert.True(t, sel.Explicit)
	assert.Equal(t, "2026-06-10", sel.Query.Get("from"))
}

func TestResolveRangeSelection_RestoresStoredCookieWithoutRewriting(t *testing.T) {
	d := rangeDeps()
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats", nil)
	r.SetPathValue("id", "g1")
	r.AddCookie(&http.Cookie{Name: guildCookieName(rangeCookieName, "g1"), Value: "2026-06-18,2026-06-22"})

	sel, rec := runRange(d, r, []time.Time{time.Date(2026, 6, 18, 0, 0, 0, 0, time.Local)})

	assert.True(t, sel.Explicit)
	assert.Equal(t, "2026-06-18,2026-06-22", sel.Query.Get("days"))
	// a restored selection is already stored, so it must not be rewritten
	assert.Empty(t, rec.Result().Cookies())
}

func TestResolveRangeSelection_DefaultIsImplicitAndDoesNotPin(t *testing.T) {
	d := rangeDeps()
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats", nil)

	sel, rec := runRange(d, r, nil)

	assert.False(t, sel.Explicit)
	assert.Empty(t, sel.Query)
	assert.Empty(t, rec.Result().Cookies())
}

// TestResolveRangeSelection_EmptyDaysClearsTheStoredSelection covers the picker's
// Clear: without this the stored cookie restored the very range the user cleared.
func TestResolveRangeSelection_EmptyDaysClearsTheStoredSelection(t *testing.T) {
	d := rangeDeps()
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats?days=", nil)
	r.SetPathValue("id", "g1")
	r.AddCookie(&http.Cookie{Name: guildCookieName(rangeCookieName, "g1"), Value: "2026-06-18,2026-06-22"})

	sel, rec := runRange(d, r, []time.Time{time.Date(2026, 6, 18, 0, 0, 0, 0, time.Local)})

	// the page falls back to its own default, and the stored selection is dropped
	assert.False(t, sel.Explicit)
	assert.Empty(t, sel.Query)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Empty(t, cookies[0].Value)
	assert.Equal(t, -1, cookies[0].MaxAge)
}

// TestResolveRangeSelection_ClearedNeedsAnEmptyFromTo covers both sides of the
// precedence between the two branches. The cleared branch is read first, so it
// must read the whole range, not the "days" value alone.
func TestResolveRangeSelection_ClearedNeedsAnEmptyFromTo(t *testing.T) {
	t.Run("every range value is empty, so the pin is dropped", func(t *testing.T) {
		d := rangeDeps()
		r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats?days=&from=&to=", nil)

		sel, rec := runRange(d, r, nil)

		assert.False(t, sel.Explicit)
		assert.Empty(t, sel.Query)
		require.Len(t, rec.Result().Cookies(), 1)
		assert.Empty(t, rec.Result().Cookies()[0].Value)
	})

	t.Run("a real from/to is a pinned range, not a reset", func(t *testing.T) {
		d := rangeDeps()
		r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats?days=&from=2026-01-01&to=2026-01-07", nil)

		sel, rec := runRange(d, r, []time.Time{time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)})

		assert.True(t, sel.Explicit)
		assert.Equal(t, "2026-01-01", sel.Query.Get("from"))
		assert.Equal(t, "2026-01-07", sel.Query.Get("to"))
		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "2026-01-01", cookies[0].Value)
	})
}

func TestRangeSelectionFrom_AbsentMiddlewareYieldsEmptySelection(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats", nil)
	sel := RangeSelectionFrom(r.Context())
	assert.False(t, sel.Explicit)
	assert.NotNil(t, sel.Query)
}

func TestEncodeDays(t *testing.T) {
	assert.Empty(t, EncodeDays(nil))
	assert.Equal(t, "2026-06-18,2026-06-22", EncodeDays([]time.Time{
		time.Date(2026, 6, 18, 0, 0, 0, 0, time.Local),
		time.Date(2026, 6, 22, 0, 0, 0, 0, time.Local),
	}))
}
