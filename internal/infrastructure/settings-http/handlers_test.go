package settingshttp

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/infrastructure/guildsettings"
	"spot-assistant/internal/infrastructure/web/webtest"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

func adminAccess(cfg guildconfig.Config) *access.GuildAccess {
	cfg.GuildID = guildID
	if cfg.Name == "" {
		cfg.Name = "Celesta Community"
	}
	return &access.GuildAccess{Config: cfg, Caps: permission.Capabilities{Admin: true, Manage: true, View: true, Reserve: true, Overbook: true}}
}

func managerAccess() *access.GuildAccess {
	return &access.GuildAccess{
		Config: guildconfig.Config{GuildID: guildID, Name: "Celesta Community", Premium: true},
		Caps:   permission.Capabilities{Manage: true, View: true, Reserve: true, Overbook: true},
	}
}

func signedInAdmin(t *testing.T, cfg guildconfig.Config) (http.Handler, webtest.Mocks, *http.Cookie) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(adminAccess(cfg), nil)
	return h, m, cookie
}

func htmx(r *http.Request) *http.Request {
	r.Header.Set("HX-Request", "true")
	return r
}

func TestHandleSettings_RendersEverySection(t *testing.T) {
	// given
	h, m, cookie := signedInAdmin(t, guildconfig.Config{Premium: true, ReserveRoleIDs: []string{"r2"}})
	m.Settings.EXPECT().Roles(mock.Anything, guildID).Return([]*role.Role{
		{ID: "r1", Name: "Leader", Color: 0xF97316},
		{ID: "r2", Name: "Member"},
	}, nil)
	m.Settings.EXPECT().World(mock.Anything, guildID).Return("Celesta", nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/settings", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"Bot permissions", "Refresh server data", "Tibia world",
		"Manage ranks", "View-only ranks", "Reserve ranks", "Overbook ranks",
		"None checked: everyone on the server can reserve.",
		"None checked: members with the @Postman role can overbook.",
		`hx-post="/servers/g1/settings/ranks/overbook"`,
		`hx-post="/servers/g1/settings/refresh"`,
		"Leader", "background-color:#f97316",
		"guild_id=g1",
		`name="world" value="Celesta"`,
		`value="r2" checked`,
	} {
		assert.Contains(t, body, want)
	}
	assert.NotContains(t, body, "The bot is inactive on this server")
	assert.Equal(t, 1, strings.Count(body, `" checked`), "only r2 in the reserve list is checked")
}

func TestHandleSettings_NonPremiumShowsNotice(t *testing.T) {
	// given
	h, m, cookie := signedInAdmin(t, guildconfig.Config{})
	m.Settings.EXPECT().Roles(mock.Anything, guildID).Return(nil, nil)
	m.Settings.EXPECT().World(mock.Anything, guildID).Return("", nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/settings", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "The bot is inactive on this server")
	assert.Contains(t, body, "No roles copied yet.")
	assert.Contains(t, body, "No world set")
}

func TestHandleSettings_Errors(t *testing.T) {
	t.Run("roles", func(t *testing.T) {
		// given
		h, m, cookie := signedInAdmin(t, guildconfig.Config{})
		m.Settings.EXPECT().Roles(mock.Anything, guildID).Return(nil, errors.New("db down"))

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/settings", cookie))

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
	t.Run("world", func(t *testing.T) {
		// given
		h, m, cookie := signedInAdmin(t, guildconfig.Config{})
		m.Settings.EXPECT().Roles(mock.Anything, guildID).Return(nil, nil)
		m.Settings.EXPECT().World(mock.Anything, guildID).Return("", errors.New("db down"))

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/settings", cookie))

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestRoutes_ManagerIsForbidden(t *testing.T) {
	cases := map[string]*http.Request{
		"page":    webtest.Get("/servers/g1/settings", nil),
		"ranks":   webtest.Post("/servers/g1/settings/ranks/manage", url.Values{"role_ids": {"r1"}}, nil),
		"world":   webtest.Post("/servers/g1/settings/world", url.Values{"world": {"Antica"}}, nil),
		"refresh": webtest.Post("/servers/g1/settings/refresh", nil, nil),
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			d, m := webtest.NewDeps(t)
			h := webtest.Handler(d, Register)
			r.AddCookie(webtest.SignIn(t, d, m, "u1"))
			m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(managerAccess(), nil)

			// when
			rec := webtest.Serve(h, r)

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}

func TestRoutes_ForeignServerIsNotFound(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(nil, ports.ErrNotFound)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/settings", cookie))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRoutes_CrossOriginPostIsRefused(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	r := webtest.Post("/servers/g1/settings/ranks/manage", url.Values{"role_ids": {"r1"}}, webtest.SignIn(t, d, m, "u1"))
	r.Header.Set("Sec-Fetch-Site", "cross-site")

	// when
	rec := webtest.Serve(h, r)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleSetRanks(t *testing.T) {
	cases := map[string]struct {
		kind  guildconfig.RoleKind
		form  url.Values
		ids   []string
		saved string
	}{
		"manage":           {guildconfig.RoleKindManage, url.Values{"role_ids": {"r1", "r2"}}, []string{"r1", "r2"}, "Manage ranks saved."},
		"view":             {guildconfig.RoleKindView, url.Values{"role_ids": {"r1"}}, []string{"r1"}, "View-only ranks saved."},
		"reserve cleared":  {guildconfig.RoleKindReserve, url.Values{}, nil, "Reserve ranks saved."},
		"overbook cleared": {guildconfig.RoleKindOverbook, url.Values{}, nil, "Overbook ranks saved."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedInAdmin(t, guildconfig.Config{})
			m.Settings.EXPECT().SetRoleIDs(mock.Anything, guildID, tc.kind, tc.ids).Return(nil)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/settings/ranks/"+string(tc.kind), tc.form, cookie)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.saved)
		})
	}
}

func TestHandleSetRanks_UnknownKind(t *testing.T) {
	// given
	h, _, cookie := signedInAdmin(t, guildconfig.Config{})

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/settings/ranks/admin", url.Values{"role_ids": {"r1"}}, cookie)))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleSetRanks_StaleRole(t *testing.T) {
	// given
	h, m, cookie := signedInAdmin(t, guildconfig.Config{})
	m.Settings.EXPECT().SetRoleIDs(mock.Anything, guildID, guildconfig.RoleKindManage, []string{"gone"}).Return(guildsettings.ErrUnknownRole)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/settings/ranks/manage", url.Values{"role_ids": {"gone"}}, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "no longer on the server")
}

func TestHandleSetRanks_Error(t *testing.T) {
	// given
	h, m, cookie := signedInAdmin(t, guildconfig.Config{})
	m.Settings.EXPECT().SetRoleIDs(mock.Anything, guildID, guildconfig.RoleKindManage, []string{"r1"}).Return(errors.New("db down"))

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/settings/ranks/manage", url.Values{"role_ids": {"r1"}}, cookie)))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSetWorld(t *testing.T) {
	cases := map[string]struct {
		err  error
		code int
		want string
	}{
		"saved":   {nil, http.StatusOK, "World saved."},
		"invalid": {guildsettings.ErrUnknownWorld, http.StatusOK, "Select a world from the list."},
		"error":   {errors.New("db down"), http.StatusInternalServerError, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedInAdmin(t, guildconfig.Config{})
			m.Settings.EXPECT().SetWorld(mock.Anything, guildID, "Celesta").Return(tc.err)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/settings/world", url.Values{"world": {"Celesta"}}, cookie)))

			// then
			assert.Equal(t, tc.code, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestHandleRefresh(t *testing.T) {
	cases := map[string]struct {
		retry time.Duration
		err   error
		code  int
		want  string
	}{
		"requested": {0, nil, http.StatusOK, "Refresh requested."},
		"cooldown":  {90 * time.Second, nil, http.StatusOK, "Try again in 2 min."},
		"error":     {0, errors.New("db down"), http.StatusInternalServerError, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedInAdmin(t, guildconfig.Config{})
			m.Settings.EXPECT().RequestResync(mock.Anything, guildID).Return(tc.retry, tc.err)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/settings/refresh", nil, cookie)))

			// then
			assert.Equal(t, tc.code, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestHandleSettings_Polish(t *testing.T) {
	// given
	h, m, cookie := signedInAdmin(t, guildconfig.Config{Premium: true})
	m.Settings.EXPECT().Roles(mock.Anything, guildID).Return([]*role.Role{{ID: "r1", Name: "Leader"}}, nil)
	m.Settings.EXPECT().World(mock.Anything, guildID).Return("Celesta", nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/settings?lang=pl", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Nic nie zaznaczono: rezerwować może każdy na serwerze.")
}
