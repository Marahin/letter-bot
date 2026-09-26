// Package webtest builds a web.Deps for handler tests: an in-memory session store,
// mockery mocks for the core services, and helpers to sign a user in. It is not a
// _test package so every feature package can import it.
package webtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/infrastructure/web"
)

// BaseURL is the origin of the test Deps; same-origin POSTs send it as Origin.
const BaseURL = "http://localhost:8080"

// Mocks are the service mocks inside the Deps NewDeps returns.
type Mocks struct {
	Auth         *mocks.MockAuthService
	Access       *mocks.MockGuildAccessService
	Premium      *mocks.MockPremiumService
	Settings     *mocks.MockGuildSettingsService
	Spots        *mocks.MockSpotService
	Reservations *mocks.MockReservationService
}

// NewDeps returns Deps over an scs memstore and fresh mocks.
func NewDeps(t *testing.T) (*web.Deps, Mocks) {
	t.Helper()
	cfg := web.Config{BaseURL: BaseURL, Discord: web.DiscordConfig{ClientID: "4242"}}
	m := Mocks{
		Auth:         mocks.NewMockAuthService(t),
		Access:       mocks.NewMockGuildAccessService(t),
		Premium:      mocks.NewMockPremiumService(t),
		Settings:     mocks.NewMockGuildSettingsService(t),
		Spots:        mocks.NewMockSpotService(t),
		Reservations: mocks.NewMockReservationService(t),
	}
	d := &web.Deps{
		Cfg:          cfg,
		Log:          zap.NewNop().Sugar(),
		Sessions:     web.NewSessionManager(cfg, nil),
		Auth:         m.Auth,
		Access:       m.Access,
		Premium:      m.Premium,
		Settings:     m.Settings,
		Spots:        m.Spots,
		Reservations: m.Reservations,
	}
	return d, m
}

// Handler mounts register on a fresh router behind the session middleware and
// CSRF check, as the real server does.
func Handler(d *web.Deps, register func(*web.Router, *web.Deps)) http.Handler {
	mux := http.NewServeMux()
	router := web.NewRouter(mux)
	d.Routes = router
	register(router, d)
	return d.WithLocale(d.CSRFMiddleware(d.Sessions.LoadAndSave(mux)))
}

// SignIn stores a session for userID and returns its cookie. The user is not a
// site admin, and the sidebar lists the given servers.
func SignIn(t *testing.T, d *web.Deps, m Mocks, userID string, servers ...access.GuildAccess) *http.Cookie {
	t.Helper()
	return signIn(t, d, m, userID, false, servers)
}

// SignInSiteAdmin is SignIn for a site admin.
func SignInSiteAdmin(t *testing.T, d *web.Deps, m Mocks, userID string, servers ...access.GuildAccess) *http.Cookie {
	t.Helper()
	return signIn(t, d, m, userID, true, servers)
}

func signIn(t *testing.T, d *web.Deps, m Mocks, userID string, siteAdmin bool, servers []access.GuildAccess) *http.Cookie {
	t.Helper()
	m.Auth.EXPECT().User(mock.Anything, userID).Return(&webuser.User{DiscordUserID: userID, Username: "user-" + userID}, nil).Maybe()
	m.Access.EXPECT().IsSiteAdmin(userID).Return(siteAdmin).Maybe()
	m.Access.EXPECT().AccessibleGuilds(mock.Anything, userID).Return(servers, nil).Maybe()

	ctx, err := d.Sessions.Load(context.Background(), "")
	require.NoError(t, err)
	d.Sessions.Put(ctx, web.SessionUserKey, userID)
	token, _, err := d.Sessions.Commit(ctx)
	require.NoError(t, err)
	return &http.Cookie{Name: d.Sessions.Cookie.Name, Value: token}
}

// Get builds a full-page browser GET, signed in when cookie is not nil.
func Get(target string, cookie *http.Cookie) *http.Request {
	r := httptest.NewRequest(http.MethodGet, target, nil)
	r.Header.Set("Accept", "text/html")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	return r
}

// Post builds a same-origin form POST, signed in when cookie is not nil.
func Post(target string, form url.Values, cookie *http.Cookie) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "text/html")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	return r
}

// Serve runs one request and returns the recorder.
func Serve(h http.Handler, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}
