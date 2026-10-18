package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetRangeSelectionCookie_ScopedSessionCookie(t *testing.T) {
	// given a server on a plain-HTTP base URL
	d := &Deps{Cfg: Config{BaseURL: "http://localhost:8080"}}
	rec := httptest.NewRecorder()

	// when a selection is stored for one guild
	d.SetRangeSelectionCookie(rec, "g1", "2026-06-18,2026-06-22")

	// then it is a session cookie (no Max-Age) scoped to that server's pages, so
	// Stats and Attendance under /servers/g1 share it but other guilds don't.
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "letter_range_g1", c.Name)
	assert.Equal(t, "2026-06-18,2026-06-22", c.Value)
	assert.Equal(t, "/", c.Path)
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.False(t, c.Secure)    // plain HTTP base URL
	assert.Equal(t, 0, c.MaxAge) // session cookie: clears when the window closes
	assert.True(t, c.Expires.IsZero())
}

func TestSetRangeSelectionCookie_SecureOnHTTPS(t *testing.T) {
	d := &Deps{Cfg: Config{BaseURL: "https://letter.example"}}
	rec := httptest.NewRecorder()
	d.SetRangeSelectionCookie(rec, "g1", "2026-06-18")
	require.Len(t, rec.Result().Cookies(), 1)
	assert.True(t, rec.Result().Cookies()[0].Secure)
}

func TestSetRangeSelectionCookie_BlankClears(t *testing.T) {
	d := &Deps{Cfg: Config{BaseURL: "http://localhost:8080"}}
	rec := httptest.NewRecorder()
	d.SetRangeSelectionCookie(rec, "g1", "")
	require.Len(t, rec.Result().Cookies(), 1)
	assert.Negative(t, rec.Result().Cookies()[0].MaxAge) // deletion
}

func TestRangeSelectionCookie_RoundTrip(t *testing.T) {
	d := &Deps{Cfg: Config{BaseURL: "http://localhost:8080"}}

	// missing cookie reads as empty
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats", nil)
	assert.Empty(t, d.RangeSelectionCookie(r))

	// a present cookie (whitespace-trimmed) reads back its value
	r.SetPathValue("id", "g1")
	r.AddCookie(&http.Cookie{Name: "letter_range_g1", Value: " 2026-06-18,2026-06-22 "})
	assert.Equal(t, "2026-06-18,2026-06-22", d.RangeSelectionCookie(r))
}
