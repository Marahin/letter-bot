package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetLanguageCookie_SiteWideAndLongLived(t *testing.T) {
	// given a server on a plain-HTTP base URL
	d := &Deps{Cfg: Config{BaseURL: "http://localhost:8080"}}
	rec := httptest.NewRecorder()

	// when a language is stored
	d.SetLanguageCookie(rec, "ru")

	// then it is one year-long cookie for every server: unlike the per-server session
	// cookie letter_range_<id>, it must survive closing the window and signing out.
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	assert.Equal(t, "letter_lang", c.Name)
	assert.Equal(t, "ru", c.Value)
	assert.Equal(t, "/", c.Path)
	assert.Equal(t, 31536000, c.MaxAge)
	assert.True(t, c.HttpOnly) // only the server reads it
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.False(t, c.Secure) // plain HTTP base URL
}

func TestSetLanguageCookie_SecureOnHTTPS(t *testing.T) {
	d := &Deps{Cfg: Config{BaseURL: "https://letter.example"}}
	rec := httptest.NewRecorder()
	d.SetLanguageCookie(rec, "pl")
	require.Len(t, rec.Result().Cookies(), 1)
	assert.True(t, rec.Result().Cookies()[0].Secure)
}

func TestSetLanguageCookie_BlankClears(t *testing.T) {
	d := &Deps{Cfg: Config{BaseURL: "http://localhost:8080"}}
	rec := httptest.NewRecorder()
	d.SetLanguageCookie(rec, "")
	require.Len(t, rec.Result().Cookies(), 1)
	assert.Negative(t, rec.Result().Cookies()[0].MaxAge) // deletion
}

func TestLanguageCookie_RoundTrip(t *testing.T) {
	d := &Deps{Cfg: Config{BaseURL: "http://localhost:8080"}}

	// missing cookie reads as empty
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	assert.Empty(t, d.LanguageCookie(r))

	// a present cookie (whitespace-trimmed) reads back its value
	r.AddCookie(&http.Cookie{Name: "letter_lang", Value: " uk "})
	assert.Equal(t, "uk", d.LanguageCookie(r))
}
