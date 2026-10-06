package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/infrastructure/i18n"
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
	cfg := Config{BaseURL: "http://localhost:8080", Discord: DiscordConfig{ClientID: "4242", InviteLink: testInvite}}
	f := &authFixture{
		auth:    mocks.NewMockAuthService(t),
		access:  mocks.NewMockGuildAccessService(t),
		premium: mocks.NewMockPremiumService(t),
	}
	f.srv = newServerWithSessions(cfg, zap.NewNop().Sugar(), NewSessionManager(cfg, nil)).
		WithLandingTool(templ.NopComponent).
		WithServices(Services{Auth: f.auth, Access: f.access, Premium: f.premium})
	f.srv.Mount(func(r *Router, d *Deps) {
		ok := func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("feature ok")) }
		r.Get("/servers/{id}/manage-only", d.RequireAuth(d.RequireManage(ok)))
		r.Get("/servers/{id}/reserve-only", d.RequireAuth(d.RequireReserve(ok)))
		r.Get("/servers/{id}/admin-only", d.RequireAuth(d.RequireAdmin(ok)))
		r.Get("/servers/{id}/reservations", d.RequireAuth(d.RequireView(ok)))
		r.Get("/site-admin-only", d.RequireAuth(d.RequireSiteAdmin(ok)))
		r.Get("/servers/{id}/public", d.PublicView(func(w http.ResponseWriter, r *http.Request) {
			current, _ := CurrentAccessFrom(r.Context())
			_, _ = w.Write([]byte("public ok, view=" + strconv.FormatBool(current.Caps.View)))
		}))
	})
	f.h = f.srv.Handler()
	return f
}

// signIn stores a session for userID and returns its cookie.
func (f *authFixture) signIn(t *testing.T, userID string, servers ...access.GuildAccess) *http.Cookie {
	t.Helper()
	f.auth.EXPECT().SetDefaultGuild(mock.Anything, userID, mock.Anything).Return(nil).Maybe()
	return f.signInUser(t, &webuser.User{DiscordUserID: userID, Username: "nyx", GlobalName: "Quiet Nyx"}, servers...)
}

// signInUser is signIn for a given user row, with no SetDefaultGuild expectation.
func (f *authFixture) signInUser(t *testing.T, user *webuser.User, servers ...access.GuildAccess) *http.Cookie {
	t.Helper()
	userID := user.DiscordUserID
	f.auth.EXPECT().User(mock.Anything, userID).Return(user, nil).Maybe()
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
	f.auth.EXPECT().Logout(mock.Anything, "u1").Return(nil).Once()

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

func TestLogout_TokenClearFailureStillSignsOut(t *testing.T) {
	// given
	f := newAuthFixture(t)
	cookie := f.signIn(t, "u1")
	f.auth.EXPECT().Logout(mock.Anything, "u1").Return(errors.New("db down")).Once()

	// when
	rec := f.do(sameOriginPost("/logout"), cookie)
	after := f.do(htmlGet("/dashboard"), cookie)

	// then
	assert.Equal(t, http.StatusSeeOther, rec.Code)
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
	assert.Contains(t, body, "Unlock with Premium")
	assert.NotContains(t, body, "Open settings")
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
		userID string
		access access.GuildAccess
	}{
		"premium server":             {"u1", guild("g1", "G", true, reserveCaps)},
		"site admin on a locked one": {"site-admin", guild("g1", "G", false, adminCaps)},
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
			assert.Equal(t, "/servers/g1/reservations", rec.Header().Get("Location"))
		})
	}
}

func TestGuildRoot_LockedServerShowsTheLock(t *testing.T) {
	// given an admin of a server without premium
	f := newAuthFixture(t)
	a := guild("g1", "G", false, adminCaps)
	cookie := f.signIn(t, "u1", a)
	f.access.EXPECT().Access(mock.Anything, "u1", "g1").Return(&a, nil)

	// when
	rec := f.do(htmlGet("/servers/g1"), cookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "data-premium-lock")
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

func TestPremiumGate(t *testing.T) {
	cases := map[string]struct {
		userID  string
		premium bool
		caps    permission.Capabilities
		path    string
		status  int
	}{
		"premium server, viewer":          {"u1", true, viewerCaps, "/servers/g1/reservations", http.StatusOK},
		"locked, viewer":                  {"u1", false, viewerCaps, "/servers/g1/reservations", http.StatusForbidden},
		"locked, reserver":                {"u1", false, reserveCaps, "/servers/g1/reserve-only", http.StatusForbidden},
		"locked, viewer on a manage page": {"u1", false, viewerCaps, "/servers/g1/manage-only", http.StatusForbidden},
		"locked, admin":                   {"u1", false, adminCaps, "/servers/g1/admin-only", http.StatusForbidden},
		"locked, site admin":              {"site-admin", false, adminCaps, "/servers/g1/admin-only", http.StatusOK},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			a := guild("g1", "Celesta", tc.premium, tc.caps)
			cookie := f.signIn(t, tc.userID, a)
			f.access.EXPECT().Access(mock.Anything, tc.userID, "g1").Return(&a, nil)

			// when
			rec := f.do(htmlGet(tc.path), cookie)

			// then
			assert.Equal(t, tc.status, rec.Code)
			body := rec.Body.String()
			if tc.status == http.StatusOK {
				assert.Equal(t, "feature ok", body)
				return
			}
			assert.Contains(t, body, "data-premium-lock")
			assert.Contains(t, body, "Unlock the feature with Premium. Join the Discord and get on board!")
			assert.Contains(t, body, `href="`+testInvite+`"`)
			assert.Contains(t, body, "Back to dashboard")
		})
	}
}

func TestPremiumGate_LockComesBeforeTheRank(t *testing.T) {
	// given a viewer on a manage page of a locked server
	f := newAuthFixture(t)
	a := guild("g1", "Celesta", false, viewerCaps)
	cookie := f.signIn(t, "u1", a)
	f.access.EXPECT().Access(mock.Anything, "u1", "g1").Return(&a, nil)

	// when
	rec := f.do(htmlGet("/servers/g1/manage-only"), cookie)

	// then the lock shows, not the rank refusal
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), "data-premium-lock")
	assert.NotContains(t, rec.Body.String(), "You don't have access")
}

func TestPremiumGate_PlainForHTMX(t *testing.T) {
	// given
	f := newAuthFixture(t)
	a := guild("g1", "Celesta", false, viewerCaps)
	cookie := f.signIn(t, "u1")
	f.access.EXPECT().Access(mock.Anything, "u1", "g1").Return(&a, nil)
	r := htmlGet("/servers/g1/reservations")
	r.Header.Set("HX-Request", "true")

	// when
	rec := f.do(r, cookie)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, "premium required\n", rec.Body.String())
}

func TestPremiumRequired_WithoutAnInviteHasNoSupportLink(t *testing.T) {
	// when
	out := render(context.Background(), t, PremiumRequired("http://x", Nav{Authenticated: true, Username: "Knight"}))

	// then
	assert.Contains(t, out, "Unlock the feature with Premium.")
	assert.NotContains(t, out, "data-support-link")
	assert.Contains(t, out, "Back to dashboard")
}

func TestPremiumRequired_AnonymousGoesBackHome(t *testing.T) {
	// when
	out := render(context.Background(), t, PremiumRequired("http://x", Nav{SupportURL: testInvite}))

	// then
	assert.Contains(t, out, "Back to home")
	assert.Contains(t, out, "Join the Discord")
	assert.NotContains(t, out, "Back to dashboard")
}

func TestPremiumRequired_InPolish(t *testing.T) {
	// given
	ctx := i18n.WithLocale(context.Background(), i18n.Normalize("pl"))

	// when
	out := render(ctx, t, PremiumRequired("http://x", Nav{SupportURL: testInvite}))

	// then
	assert.Contains(t, out, "Odblokuj tę funkcję w Premium. Dołącz do Discorda i wskakuj na pokład!")
	assert.Contains(t, out, "Dołącz do Discorda</a>")
}

func TestPublicView(t *testing.T) {
	cases := map[string]struct {
		userID    string
		premium   bool
		member    bool
		accessErr error
		status    int
		body      string
	}{
		"anonymous":                  {premium: true, status: http.StatusOK, body: "view=false"},
		"anonymous on a locked one":  {status: http.StatusForbidden, body: "data-premium-lock"},
		"member":                     {userID: "u1", premium: true, member: true, status: http.StatusOK, body: "view=true"},
		"member on a locked one":     {userID: "u1", member: true, status: http.StatusForbidden, body: "data-premium-lock"},
		"site admin on a locked one": {userID: "site-admin", member: true, status: http.StatusOK, body: "view=true"},
		"not a member":               {userID: "u1", premium: true, accessErr: ports.ErrNotFound, status: http.StatusOK, body: "view=false"},
		"discord is down":            {userID: "u1", premium: true, accessErr: ports.ErrUpstreamUnavailable, status: http.StatusOK, body: "view=false"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			var cookie *http.Cookie
			public := guild("g1", "Celesta", tc.premium, permission.Capabilities{})
			f.access.EXPECT().Public(mock.Anything, "g1").Return(&public, nil).Maybe()
			if tc.userID != "" {
				cookie = f.signIn(t, tc.userID)
				member := guild("g1", "Celesta", tc.premium, adminCaps)
				if tc.member {
					f.access.EXPECT().Access(mock.Anything, tc.userID, "g1").Return(&member, nil)
				} else {
					f.access.EXPECT().Access(mock.Anything, tc.userID, "g1").Return(nil, tc.accessErr)
				}
			}

			// when
			rec := f.do(htmlGet("/servers/g1/public"), cookie)

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.body)
		})
	}
}

func TestPublicView_StaleSessionIsDestroyedAndTheVisitorGoesOn(t *testing.T) {
	// given a session whose user row is gone
	f := newAuthFixture(t)
	f.auth.EXPECT().User(mock.Anything, "gone").Return(nil, ports.ErrNotFound)
	cookie := f.signInUser(t, &webuser.User{DiscordUserID: "ignored"})
	ctx, err := f.srv.sessions.Load(context.Background(), cookie.Value)
	require.NoError(t, err)
	f.srv.sessions.Put(ctx, SessionUserKey, "gone")
	_, _, err = f.srv.sessions.Commit(ctx)
	require.NoError(t, err)
	public := guild("g1", "Celesta", true, permission.Capabilities{})
	f.access.EXPECT().Public(mock.Anything, "g1").Return(&public, nil)

	// when
	rec := f.do(htmlGet("/servers/g1/public"), cookie)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "public ok, view=false", rec.Body.String())
	assert.Equal(t, -1, sessionCookie(rec).MaxAge, "the stale session is destroyed")
}

func TestPublicView_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		status int
	}{
		"unknown server": {ports.ErrNotFound, http.StatusNotFound},
		"database error": {assert.AnError, http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			// given
			f := newAuthFixture(t)
			f.access.EXPECT().Public(mock.Anything, "g1").Return(nil, tc.err)

			// when
			rec := f.do(htmlGet("/servers/g1/public"), nil)

			// then
			assert.Equal(t, tc.status, rec.Code)
		})
	}
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
	user := &webuser.User{DiscordUserID: "site-admin", Username: "nyx", GlobalName: "Quiet Nyx", DefaultGuildID: "g1"}
	cookie := f.signInUser(t, user, guild("g2", "Other", true, viewerCaps), celesta)

	// when: the dashboard keeps the remembered server selected
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
