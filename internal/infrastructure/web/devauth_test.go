//go:build devauth

package web

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/infrastructure/devauth"
)

const devManagerID = "700000000000000001"

func newDevAuthFixture(t *testing.T, devAuth bool, baseURL string) *authFixture {
	t.Helper()
	f := newAuthFixture(t)
	f.srv.cfg.DevAuth = devAuth
	f.srv.cfg.BaseURL = baseURL
	f.h = f.srv.Handler()
	return f
}

func TestDevLogin_ListsTheMockUsers(t *testing.T) {
	// given
	f := newDevAuthFixture(t, true, "http://localhost:8080")

	// when
	rec := f.do(htmlGet("/dev/login?to=/dashboard"), nil)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	for _, u := range devauth.Users {
		assert.Contains(t, rec.Body.String(), `href="/dev/login/`+u.ID+`?to=%2Fdashboard"`)
	}
	assert.Contains(t, rec.Body.String(), "Development mock authentication.")
}

func TestDevLogin_DisabledWithoutTheFlagOrOnARealOrigin(t *testing.T) {
	for name, f := range map[string]*authFixture{
		"flag off":        newDevAuthFixture(t, false, "http://localhost:8080"),
		"production":      newDevAuthFixture(t, true, "https://tibialoot.com"),
		"look-alike host": newDevAuthFixture(t, true, "http://localhost.attacker.test"),
		"unparsable":      newDevAuthFixture(t, true, "http://[::1"),
	} {
		t.Run(name, func(t *testing.T) {
			// when
			list := f.do(htmlGet("/dev/login"), nil)
			login := f.do(htmlGet("/dev/login/"+devManagerID), nil)

			// then
			assert.Equal(t, http.StatusNotFound, list.Code)
			assert.Equal(t, http.StatusNotFound, login.Code)
		})
	}
}

func TestDevLogin_SignsInAndLands(t *testing.T) {
	for name, tc := range map[string]struct {
		target   string
		location string
	}{
		"dashboard by default": {"/dev/login/" + devManagerID, "/dashboard"},
		"back to the page":     {"/dev/login/" + devManagerID + "?to=%2Fservers%2Fg1%2Freservations", "/servers/g1/reservations"},
		"never off-site":       {"/dev/login/" + devManagerID + "?to=https%3A%2F%2Fevil.test", "/dashboard"},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			f := newDevAuthFixture(t, true, "http://127.0.0.1:8080")
			f.auth.EXPECT().Complete(mock.Anything, devauth.DevCodePrefix+devManagerID).Return(&webuser.User{DiscordUserID: devManagerID}, nil)

			// when
			rec := f.do(htmlGet(tc.target), nil)

			// then
			require.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, tc.location, rec.Header().Get("Location"))
			assert.NotNil(t, sessionCookie(rec))
		})
	}
}

func TestDevLogin_UnknownUserAndFailure(t *testing.T) {
	// given
	f := newDevAuthFixture(t, true, "http://localhost:8080")
	f.auth.EXPECT().Complete(mock.Anything, devauth.DevCodePrefix+devManagerID).Return(nil, assert.AnError)

	// when
	unknown := f.do(htmlGet("/dev/login/1"), nil)
	failed := f.do(htmlGet("/dev/login/"+devManagerID), nil)

	// then
	assert.Equal(t, http.StatusNotFound, unknown.Code)
	assert.Equal(t, http.StatusInternalServerError, failed.Code)
}
