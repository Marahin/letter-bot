package adminhttp

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/premium"
	"spot-assistant/internal/infrastructure/web/webtest"
	"spot-assistant/internal/ports"
)

func guilds() []*guildconfig.Config {
	return []*guildconfig.Config{
		{GuildID: "3", Name: "gone", BotPresent: false},
		{GuildID: "2", Name: "Side Guild", BotPresent: true},
		{GuildID: "806152499760201738", Name: "Celesta Community", BotPresent: true, Premium: true, PremiumForever: true},
		{GuildID: "4", BotPresent: true, Premium: true},
	}
}

func TestRoutes_AreHiddenFromNonAdmins(t *testing.T) {
	cases := map[string]*http.Request{
		"list":   webtest.Get("/admin/guilds", nil),
		"toggle": webtest.Post("/admin/guilds/2/premium", url.Values{"premium": {"true"}}, nil),
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			d, m := webtest.NewDeps(t)
			h := webtest.Handler(d, Register)
			r.AddCookie(webtest.SignIn(t, d, m, "u1"))

			// when
			rec := webtest.Serve(h, r)

			// then
			assert.Equal(t, http.StatusNotFound, rec.Code)
			m.Premium.AssertNotCalled(t, "List", mock.Anything)
			m.Premium.AssertNotCalled(t, "SetPremium", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestRoutes_AnonymousGoesToLogin(t *testing.T) {
	// given
	d, _ := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)

	// when
	rec := webtest.Serve(h, webtest.Get("/admin/guilds", nil))

	// then
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login?to=%2Fadmin%2Fguilds", rec.Header().Get("Location"))
}

func TestHandleGuilds(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignInSiteAdmin(t, d, m, "admin")
	m.Premium.EXPECT().List(mock.Anything).Return(guilds(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/admin/guilds", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Celesta Community")
	assert.Contains(t, body, "Premium forever")
	assert.Contains(t, body, "(no name yet)")
	assert.Contains(t, body, "Bot removed")
	assert.Contains(t, body, `hx-post="/admin/guilds/2/premium"`)
	assert.NotContains(t, body, `hx-post="/admin/guilds/806152499760201738/premium"`)
}

func TestHandleGuilds_ListError(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignInSiteAdmin(t, d, m, "admin")
	m.Premium.EXPECT().List(mock.Anything).Return(nil, assert.AnError)

	// when
	rec := webtest.Serve(h, webtest.Get("/admin/guilds", cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSetPremium_PlainFormRedirects(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignInSiteAdmin(t, d, m, "admin")
	m.Premium.EXPECT().SetPremium(mock.Anything, "2", true).Return(&guildconfig.Config{GuildID: "2", Premium: true}, nil)

	// when
	rec := webtest.Serve(h, webtest.Post("/admin/guilds/2/premium", url.Values{"premium": {"true"}}, cookie))

	// then
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/admin/guilds", rec.Header().Get("Location"))
}

func TestHandleSetPremium_HTMXGetsTheRow(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignInSiteAdmin(t, d, m, "admin")
	m.Premium.EXPECT().SetPremium(mock.Anything, "2", false).Return(&guildconfig.Config{GuildID: "2", Name: "Side Guild", BotPresent: true}, nil)
	r := webtest.Post("/admin/guilds/2/premium", url.Values{"premium": {"false"}}, cookie)
	r.Header.Set("HX-Request", "true")

	// when
	rec := webtest.Serve(h, r)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `id="guild-2"`)
	assert.Contains(t, body, "Turn on")
	assert.NotContains(t, body, "<html")
}

func TestHandleSetPremium_Refusals(t *testing.T) {
	cases := map[string]struct {
		value  string
		err    error
		status int
	}{
		"invalid value":   {"yes", nil, http.StatusBadRequest},
		"unknown server":  {"true", ports.ErrNotFound, http.StatusNotFound},
		"premium forever": {"false", premium.ErrPremiumForever, http.StatusConflict},
		"store error":     {"true", assert.AnError, http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			d, m := webtest.NewDeps(t)
			h := webtest.Handler(d, Register)
			cookie := webtest.SignInSiteAdmin(t, d, m, "admin")
			if tc.err != nil {
				m.Premium.EXPECT().SetPremium(mock.Anything, "2", tc.value == "true").Return(nil, tc.err)
			}

			// when
			rec := webtest.Serve(h, webtest.Post("/admin/guilds/2/premium", url.Values{"premium": {tc.value}}, cookie))

			// then
			assert.Equal(t, tc.status, rec.Code)
		})
	}
}

func TestHandleSetPremium_CrossSiteIsRefused(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignInSiteAdmin(t, d, m, "admin")
	r := webtest.Post("/admin/guilds/2/premium", url.Values{"premium": {"true"}}, cookie)
	r.Header.Set("Sec-Fetch-Site", "cross-site")

	// when
	rec := webtest.Serve(h, r)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
