package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/ports"
)

type authFixture struct {
	srv     *Server
	h       http.Handler
	auth    *mocks.MockAuthService
	access  *mocks.MockGuildAccessService
	premium *mocks.MockPremiumService
}

func newAuthFixture(t *testing.T) *authFixture {
	t.Helper()
	cfg := Config{BaseURL: "http://localhost:8080", Discord: DiscordConfig{ClientID: "4242"}}
	f := &authFixture{
		auth:    mocks.NewMockAuthService(t),
		access:  mocks.NewMockGuildAccessService(t),
		premium: mocks.NewMockPremiumService(t),
	}
	f.srv = newServer(cfg, zap.NewNop().Sugar(), NewSessionManager(cfg, nil)).
		WithServices(Services{Auth: f.auth, Access: f.access, Premium: f.premium})
	f.srv.Mount(func(r *Router, d *Deps) {
		ok := func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("feature ok")) }
		r.Get("/servers/{id}/manage-only", d.RequireAuth(d.RequireManage(ok)))
		r.Get("/servers/{id}/reserve-only", d.RequireAuth(d.RequireReserve(ok)))
		r.Get("/servers/{id}/admin-only", d.RequireAuth(d.RequireAdmin(ok)))
		r.Get("/servers/{id}/premium-only", d.RequireAuth(d.RequireView(d.RequirePremium(ok))))
		r.Get("/servers/{id}/reservations", d.RequireAuth(d.RequireView(ok)))
		r.Get("/site-admin-only", d.RequireAuth(d.RequireSiteAdmin(ok)))
	})
	f.h = f.srv.Handler()
	return f
}

// signIn stores a session for userID and returns its cookie.
func (f *authFixture) signIn(t *testing.T, userID string, servers ...access.GuildAccess) *http.Cookie {
	t.Helper()
	f.auth.EXPECT().User(mock.Anything, userID).Return(&webuser.User{DiscordUserID: userID, Username: "nyx", GlobalName: "Quiet Nyx"}, nil).Maybe()
	f.access.EXPECT().IsSiteAdmin(userID).Return(userID == "site-admin").Maybe()
	f.access.EXPECT().AccessibleGuilds(mock.Anything, userID).Return(servers, nil).Maybe()
	ctx, err := f.srv.sessions.Load(context.Background(), "")
	require.NoError(t, err)
	f.srv.sessions.Put(ctx, SessionUserKey, userID)
	token, _, err := f.srv.sessions.Commit(ctx)
	require.NoError(t, err)
	return &http.Cookie{Name: sessionCookieName, Value: token}
}

func (f *authFixture) do(r *http.Request, cookie *http.Cookie) *httptest.ResponseRecorder {
	if cookie != nil {
		r.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, r)
	return rec
}

func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

func sameOriginPost(target string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, target, strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	return r
}

func guild(id, name string, premium bool, caps permission.Capabilities) access.GuildAccess {
	return access.GuildAccess{Config: guildconfig.Config{GuildID: id, Name: name, BotPresent: true, Premium: premium}, Caps: caps}
}

var (
	viewerCaps  = permission.Capabilities{View: true}
	reserveCaps = permission.Capabilities{View: true, Reserve: true}
	adminCaps   = permission.Capabilities{Admin: true, Manage: true, View: true, Reserve: true, Overbook: true}
)

// login runs GET /login and returns the state sent to Discord and the session cookie.
func (f *authFixture) login(t *testing.T, target string) (string, *http.Cookie) {
	t.Helper()
	var state string
	f.auth.EXPECT().LoginURL(mock.Anything).RunAndReturn(func(s string) string {
		state = s
		return "https://discord.test/authorize?state=" + s
	}).Once()
	rec := f.do(htmlGet(target), nil)
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "https://discord.test/authorize?state="+state, rec.Header().Get("Location"))
	require.Len(t, state, 32)
	cookie := sessionCookie(rec)
	require.NotNil(t, cookie)
	return state, cookie
}

func TestLogin_RedirectsToDiscordWithFreshState(t *testing.T) {
	// given
	f := newAuthFixture(t)

	// when
	first, _ := f.login(t, "/login")
	second, _ := f.login(t, "/login")

	// then
	assert.NotEqual(t, first, second)
}

func TestLogin_SignedInGoesStraightToTheDestination(t *testing.T) {
	// given
	f := newAuthFixture(t)
	cookie := f.signIn(t, "u1")

	// when
	rec := f.do(htmlGet("/login?to=/evil.example"), cookie)

	// then
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

func TestCallback_CompletesSignInAndRenewsTheSession(t *testing.T) {
	// given
	f := newAuthFixture(t)
	state, cookie := f.login(t, "/login?to="+url.QueryEscape("/servers/g1/reservations"))
	f.auth.EXPECT().Complete(mock.Anything, "code-1").Return(&webuser.User{DiscordUserID: "u1"}, nil)

	// when
	rec := f.do(htmlGet("/auth/callback?state="+state+"&code=code-1"), cookie)

	// then
	require.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/servers/g1/reservations", rec.Header().Get("Location"))
	renewed := sessionCookie(rec)
	require.NotNil(t, renewed)
	assert.NotEqual(t, cookie.Value, renewed.Value, "the session id must change at sign-in")

	// and the renewed session is signed in
	f.auth.EXPECT().User(mock.Anything, "u1").Return(&webuser.User{DiscordUserID: "u1", Username: "nyx"}, nil)
	f.access.EXPECT().AccessibleGuilds(mock.Anything, "u1").Return(nil, nil)
	f.access.EXPECT().IsSiteAdmin("u1").Return(false)
	dash := f.do(htmlGet("/dashboard"), renewed)
	assert.Equal(t, http.StatusOK, dash.Code)
}

func TestCallback_DefaultsToTheDashboard(t *testing.T) {
	// given
	f := newAuthFixture(t)
	state, cookie := f.login(t, "/login?to=https://evil.example/")
	f.auth.EXPECT().Complete(mock.Anything, "code-1").Return(&webuser.User{DiscordUserID: "u1"}, nil)

	// when
	rec := f.do(htmlGet("/auth/callback?state="+state+"&code=code-1"), cookie)

	// then
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/dashboard", rec.Header().Get("Location"))
}

func TestCallback_RefusesABadState(t *testing.T) {
	cases := map[string]func(state string) string{
		"wrong state":   func(string) string { return "/auth/callback?state=forged&code=c" },
		"missing state": func(string) string { return "/auth/callback?code=c" },
		"missing code":  func(state string) string { return "/auth/callback?state=" + state },
	}
	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			state, cookie := f.login(t, "/login")

			// when
			rec := f.do(htmlGet(target(state)), cookie)

			// then
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			f.auth.AssertNotCalled(t, "Complete", mock.Anything, mock.Anything)
		})
	}
}

func TestCallback_StateIsSingleUse(t *testing.T) {
	// given
	f := newAuthFixture(t)
	state, cookie := f.login(t, "/login")
	first := f.do(htmlGet("/auth/callback?state="+state+"&error=access_denied"), cookie)
	require.Equal(t, http.StatusSeeOther, first.Code)
	assert.Equal(t, "/", first.Header().Get("Location"))

	// when
	rec := f.do(htmlGet("/auth/callback?state="+state+"&code=c"), cookie)

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCallback_WithoutSessionIsRefused(t *testing.T) {
	// given
	f := newAuthFixture(t)

	// when
	rec := f.do(htmlGet("/auth/callback?state=abc&code=c"), nil)

	// then
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCallback_CompleteErrors(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
	}{
		"refused code":    {ports.ErrUnauthorized, http.StatusBadRequest},
		"discord is down": {ports.ErrUpstreamUnavailable, http.StatusServiceUnavailable},
		"other":           {assert.AnError, http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			state, cookie := f.login(t, "/login")
			f.auth.EXPECT().Complete(mock.Anything, "c").Return(nil, tc.err)

			// when
			rec := f.do(htmlGet("/auth/callback?state="+state+"&code=c"), cookie)

			// then
			assert.Equal(t, tc.status, rec.Code)
		})
	}
}

func TestLogout(t *testing.T) {
	// given
	f := newAuthFixture(t)
	cookie := f.signIn(t, "u1")

	// when
	forged := httptest.NewRequest(http.MethodPost, "/logout", nil)
	forged.Header.Set("Sec-Fetch-Site", "cross-site")
	refused := f.do(forged, cookie)
	rec := f.do(sameOriginPost("/logout"), cookie)
	after := f.do(htmlGet("/dashboard"), cookie)

	// then
	assert.Equal(t, http.StatusForbidden, refused.Code)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/", rec.Header().Get("Location"))
	assert.Equal(t, http.StatusSeeOther, after.Code)
	assert.Equal(t, "/login?to=%2Fdashboard", after.Header().Get("Location"))
}

func TestRequireAuth(t *testing.T) {
	t.Run("anonymous page view goes to login and back", func(t *testing.T) {
		// given
		f := newAuthFixture(t)

		// when
		rec := f.do(htmlGet("/servers/g1/reservations?scope=past"), nil)

		// then
		assert.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/login?to="+url.QueryEscape("/servers/g1/reservations?scope=past"), rec.Header().Get("Location"))
	})
	t.Run("anonymous htmx request asks for a full navigation", func(t *testing.T) {
		// given
		f := newAuthFixture(t)
		r := htmlGet("/dashboard")
		r.Header.Set("HX-Request", "true")

		// when
		rec := f.do(r, nil)

		// then
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
		assert.Equal(t, "/login", rec.Header().Get("HX-Redirect"))
	})
	t.Run("deleted user is signed out", func(t *testing.T) {
		// given
		f := newAuthFixture(t)
		ctx, _ := f.srv.sessions.Load(context.Background(), "")
		f.srv.sessions.Put(ctx, SessionUserKey, "gone")
		token, _, _ := f.srv.sessions.Commit(ctx)
		f.auth.EXPECT().User(mock.Anything, "gone").Return(nil, ports.ErrNotFound)

		// when
		rec := f.do(htmlGet("/dashboard"), &http.Cookie{Name: sessionCookieName, Value: token})

		// then
		assert.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/login?to=%2Fdashboard", rec.Header().Get("Location"))
		gone := sessionCookie(rec)
		require.NotNil(t, gone)
		assert.True(t, gone.MaxAge < 0 || gone.Value == "")
	})
	t.Run("user lookup failure is a 500", func(t *testing.T) {
		// given
		f := newAuthFixture(t)
		ctx, _ := f.srv.sessions.Load(context.Background(), "")
		f.srv.sessions.Put(ctx, SessionUserKey, "u1")
		token, _, _ := f.srv.sessions.Commit(ctx)
		f.auth.EXPECT().User(mock.Anything, "u1").Return(nil, assert.AnError)

		// when
		rec := f.do(htmlGet("/dashboard"), &http.Cookie{Name: sessionCookieName, Value: token})

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestDashboard_ListsServers(t *testing.T) {
	// given
	f := newAuthFixture(t)
	servers := []access.GuildAccess{
		guild("g1", "Celesta Community", true, reserveCaps),
		guild("g2", "Side Guild", false, adminCaps),
	}
	cookie := f.signIn(t, "u1", servers...)

	// when
	rec := f.do(htmlGet("/dashboard"), cookie)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Your servers")
	assert.Contains(t, body, "Quiet Nyx")
	assert.Contains(t, body, "Celesta Community")
	assert.Contains(t, body, "Premium")
	assert.Contains(t, body, "Inactive: premium required")
	assert.Contains(t, body, "Open reservations")
	assert.Contains(t, body, "Open settings")
	assert.Contains(t, body, `href="/servers/g1"`)
	assert.Contains(t, body, "client_id=4242")
}

func TestDashboard_EmptyInPolish(t *testing.T) {
	// given
	f := newAuthFixture(t)
	cookie := f.signIn(t, "u1")

	// when
	rec := f.do(htmlGet("/dashboard?lang=pl"), cookie)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Lettera nie ma jeszcze")
}

func TestDashboard_AccessErrors(t *testing.T) {
	cases := map[string]struct {
		err      error
		status   int
		location string
	}{
		"discord is down": {ports.ErrUpstreamUnavailable, http.StatusServiceUnavailable, ""},
		"token refused":   {ports.ErrUnauthorized, http.StatusSeeOther, "/login?to=%2Fdashboard"},
		"database error":  {assert.AnError, http.StatusInternalServerError, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			f.access.EXPECT().AccessibleGuilds(mock.Anything, "u1").Return(nil, tc.err).Once()
			cookie := f.signIn(t, "u1")

			// when
			rec := f.do(htmlGet("/dashboard"), cookie)

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, tc.location, rec.Header().Get("Location"))
		})
	}
}

func TestGuildRoot(t *testing.T) {
	cases := map[string]struct {
		userID   string
		access   access.GuildAccess
		location string
	}{
		"premium server":              {"u1", guild("g1", "G", true, reserveCaps), "/servers/g1/reservations"},
		"admin of a non-premium one":  {"u1", guild("g1", "G", false, adminCaps), "/servers/g1/settings"},
		"member of a non-premium one": {"u1", guild("g1", "G", false, reserveCaps), "/servers/g1/reservations"},
		"site admin":                  {"site-admin", guild("g1", "G", false, adminCaps), "/servers/g1/reservations"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			cookie := f.signIn(t, tc.userID)
			a := tc.access
			f.access.EXPECT().Access(mock.Anything, tc.userID, "g1").Return(&a, nil)

			// when
			rec := f.do(htmlGet("/servers/g1"), cookie)

			// then
			assert.Equal(t, http.StatusSeeOther, rec.Code)
			assert.Equal(t, tc.location, rec.Header().Get("Location"))
		})
	}
}

func TestGuildGuards(t *testing.T) {
	cases := map[string]struct {
		path   string
		caps   permission.Capabilities
		status int
	}{
		"viewer on a manage page":  {"/servers/g1/manage-only", viewerCaps, http.StatusForbidden},
		"viewer on a reserve page": {"/servers/g1/reserve-only", viewerCaps, http.StatusForbidden},
		"reserver on reserve page": {"/servers/g1/reserve-only", reserveCaps, http.StatusOK},
		"manager on admin page":    {"/servers/g1/admin-only", permission.Capabilities{Manage: true, View: true, Reserve: true}, http.StatusForbidden},
		"admin on admin page":      {"/servers/g1/admin-only", adminCaps, http.StatusOK},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			cookie := f.signIn(t, "u1")
			a := guild("g1", "G", true, tc.caps)
			f.access.EXPECT().Access(mock.Anything, "u1", "g1").Return(&a, nil)

			// when
			rec := f.do(htmlGet(tc.path), cookie)

			// then
			assert.Equal(t, tc.status, rec.Code)
		})
	}
}

func TestGuildGuards_ForeignServerIs404(t *testing.T) {
	// given
	f := newAuthFixture(t)
	cookie := f.signIn(t, "u1")
	f.access.EXPECT().Access(mock.Anything, "u1", "foreign").Return(nil, ports.ErrNotFound)

	// when
	rec := f.do(htmlGet("/servers/foreign/reservations"), cookie)

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "feature ok")
}

func TestGuildGuards_AccessErrors(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
	}{
		"discord is down": {ports.ErrUpstreamUnavailable, http.StatusServiceUnavailable},
		"token refused":   {ports.ErrUnauthorized, http.StatusSeeOther},
		"other":           {assert.AnError, http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			cookie := f.signIn(t, "u1")
			f.access.EXPECT().Access(mock.Anything, "u1", "g1").Return(nil, tc.err)

			// when
			rec := f.do(htmlGet("/servers/g1/reservations"), cookie)

			// then
			assert.Equal(t, tc.status, rec.Code)
		})
	}
}

func TestRequirePremium(t *testing.T) {
	cases := map[string]struct {
		userID  string
		premium bool
		caps    permission.Capabilities
		status  int
		body    string
	}{
		"premium server":          {"u1", true, viewerCaps, http.StatusOK, "feature ok"},
		"non-premium, member":     {"u1", false, viewerCaps, http.StatusForbidden, "Premium required"},
		"non-premium, admin":      {"u1", false, adminCaps, http.StatusForbidden, `href="/servers/g1/settings"`},
		"non-premium, site admin": {"site-admin", false, adminCaps, http.StatusOK, "feature ok"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			a := guild("g1", "Celesta", tc.premium, tc.caps)
			cookie := f.signIn(t, tc.userID, a)
			f.access.EXPECT().Access(mock.Anything, tc.userID, "g1").Return(&a, nil)

			// when
			rec := f.do(htmlGet("/servers/g1/premium-only"), cookie)

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.body)
		})
	}
}

func TestRequirePremium_PlainForHTMX(t *testing.T) {
	// given
	f := newAuthFixture(t)
	a := guild("g1", "Celesta", false, viewerCaps)
	cookie := f.signIn(t, "u1")
	f.access.EXPECT().Access(mock.Anything, "u1", "g1").Return(&a, nil)
	r := htmlGet("/servers/g1/premium-only")
	r.Header.Set("HX-Request", "true")

	// when
	rec := f.do(r, cookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "premium required\n", rec.Body.String())
}

func TestRequireSiteAdmin(t *testing.T) {
	// given
	f := newAuthFixture(t)
	user := f.signIn(t, "u1")
	admin := f.signIn(t, "site-admin")

	// when
	refused := f.do(htmlGet("/site-admin-only"), user)
	allowed := f.do(htmlGet("/site-admin-only"), admin)

	// then
	assert.Equal(t, http.StatusNotFound, refused.Code)
	assert.Equal(t, http.StatusOK, allowed.Code)
}

func TestNav_SignedInSidebar(t *testing.T) {
	// given
	f := newAuthFixture(t)
	celesta := guild("g1", "Celesta Community", true, reserveCaps)
	cookie := f.signIn(t, "site-admin", celesta, guild("g2", "Other", true, viewerCaps))
	exact := guild("g1", "Celesta Community", true, adminCaps)
	f.access.EXPECT().Access(mock.Anything, "site-admin", "g1").Return(&exact, nil)

	// when: a guild page remembers the server, and the dashboard keeps it selected
	_ = f.do(htmlGet("/servers/g1/premium-only"), cookie)
	rec := f.do(htmlGet("/dashboard"), cookie)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `href="/servers/g1/reservations"`)
	assert.Contains(t, body, `href="/admin/guilds"`)
	assert.Contains(t, body, `href="/servers/g2"`)
	assert.Contains(t, body, "Quiet Nyx")
}

func TestNav_UsesTheExactAccessOfTheCurrentServer(t *testing.T) {
	// given
	f := newAuthFixture(t)
	d := f.srv.deps()
	list := []access.GuildAccess{guild("g1", "Celesta", true, viewerCaps)}
	f.access.EXPECT().AccessibleGuilds(mock.Anything, "u1").Return(list, nil)
	f.access.EXPECT().IsSiteAdmin("u1").Return(false)
	user := &webuser.User{DiscordUserID: "u1", Username: "nyx"}
	ctx := context.WithValue(context.Background(), ctxUser, user)
	ctx = WithCurrentAccess(ctx, guild("g1", "Celesta", true, adminCaps))
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/stats", nil).WithContext(ctx)

	// when
	n := d.Nav(r, "g1")

	// then
	assert.True(t, n.Authenticated)
	assert.Equal(t, "nyx", n.Username)
	assert.True(t, n.IsAdmin)
	assert.True(t, n.CanManage)
	assert.Equal(t, "Celesta", n.CurrentGuildName)
	assert.Equal(t, "/servers/g1/stats", n.ReturnTo)
}

func TestNav_DegradesWhenTheServerListFails(t *testing.T) {
	// given
	f := newAuthFixture(t)
	d := f.srv.deps()
	f.access.EXPECT().AccessibleGuilds(mock.Anything, "u1").Return(nil, ports.ErrUpstreamUnavailable)
	f.access.EXPECT().IsSiteAdmin("u1").Return(false)
	ctx := context.WithValue(context.Background(), ctxUser, &webuser.User{DiscordUserID: "u1", Username: "nyx"})

	// when
	n := d.Nav(httptest.NewRequest(http.MethodGet, "/dashboard", nil).WithContext(ctx), "")

	// then
	assert.True(t, n.Authenticated)
	assert.Empty(t, n.Servers)
}

func TestNav_WithoutLoadedSessionIsSignedOut(t *testing.T) {
	// given: RecoverMiddleware renders outside LoadAndSave
	f := newAuthFixture(t)
	d := f.srv.deps()

	// when
	n := d.Nav(httptest.NewRequest(http.MethodGet, "/x", nil), "")

	// then
	assert.False(t, n.Authenticated)
}

func TestNav_UserLookupFailureIsSignedOut(t *testing.T) {
	// given
	f := newAuthFixture(t)
	d := f.srv.deps()
	ctx, _ := f.srv.sessions.Load(context.Background(), "")
	f.srv.sessions.Put(ctx, SessionUserKey, "u1")
	f.auth.EXPECT().User(mock.Anything, "u1").Return(nil, assert.AnError)

	// when
	n := d.Nav(httptest.NewRequest(http.MethodGet, "/x", nil).WithContext(ctx), "")

	// then
	assert.False(t, n.Authenticated)
}

func TestRandomState(t *testing.T) {
	// when
	a, errA := RandomState()
	b, errB := RandomState()

	// then
	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.Len(t, a, 32)
	assert.NotEqual(t, a, b)
}
