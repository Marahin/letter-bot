package channelshttp

import (
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/infrastructure/web/webtest"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

func signedIn(t *testing.T, cfg guildconfig.Config, caps permission.Capabilities) (http.Handler, webtest.Mocks, *http.Cookie) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	cfg.GuildID = guildID
	cfg.Name = "Celesta Community"
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(&access.GuildAccess{Config: cfg, Caps: caps}, nil)
	return h, m, cookie
}

func admin() permission.Capabilities {
	return permission.Capabilities{Admin: true, Manage: true, View: true, Reserve: true, Overbook: true}
}

func synced() []*discord.Channel {
	return []*discord.Channel{
		{ID: "c1", Name: "letter", Type: discord.ChannelTypeGuildText},
		{ID: "c2", Name: "letter-summary", Type: discord.ChannelTypeGuildText},
	}
}

func htmxPost(target string, form url.Values, cookie *http.Cookie) *http.Request {
	r := webtest.Post(target, form, cookie)
	r.Header.Set("HX-Request", "true")
	return r
}

func TestHandleChannels(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, guildconfig.Config{Premium: true, CommandChannelID: "c1"}, admin())
	m.Settings.EXPECT().Channels(mock.Anything, guildID).Return(synced(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/channels", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"Command channel", "Summary channel",
		"Any channel (default)", "Default: #letter-summary",
		`name="command_channel_id" value="c1"`,
		`name="summary_channel_id" value=""`,
		`hx-post="/servers/g1/channels"`,
		`href="/servers/g1/settings"`,
	} {
		assert.Contains(t, body, want)
	}
	assert.NotContains(t, body, "no longer on the server")
}

func TestHandleChannels_WarnsAboutDeletedChannel(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, guildconfig.Config{Premium: true, SummaryChannelID: "gone"}, admin())
	m.Settings.EXPECT().Channels(mock.Anything, guildID).Return(nil, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/channels", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "The saved summary channel is no longer on the server.")
	assert.Contains(t, body, "No channels copied yet.")
}

func TestHandleChannels_Error(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, guildconfig.Config{Premium: true}, admin())
	m.Settings.EXPECT().Channels(mock.Anything, guildID).Return(nil, errors.New("db down"))

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/channels", cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRoutes_ManagerIsForbidden(t *testing.T) {
	for name, r := range map[string]*http.Request{
		"page": webtest.Get("/servers/g1/channels", nil),
		"save": webtest.Post("/servers/g1/channels", url.Values{"command_channel_id": {"c1"}}, nil),
	} {
		t.Run(name, func(t *testing.T) {
			// given
			d, m := webtest.NewDeps(t)
			h := webtest.Handler(d, Register)
			r.AddCookie(webtest.SignIn(t, d, m, "u1"))
			m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(&access.GuildAccess{
				Config: guildconfig.Config{GuildID: guildID, Premium: true},
				Caps:   permission.Capabilities{Manage: true, View: true, Reserve: true},
			}, nil)

			// when
			rec := webtest.Serve(h, r)

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}

func TestRoutes_NonPremiumIsLocked(t *testing.T) {
	for name, r := range map[string]*http.Request{
		"page": webtest.Get("/servers/g1/channels", nil),
		"save": webtest.Post("/servers/g1/channels", url.Values{"command_channel_id": {"c1"}}, nil),
	} {
		t.Run(name, func(t *testing.T) {
			// given an admin of a server without premium
			d, m := webtest.NewDeps(t)
			h := webtest.Handler(d, Register)
			r.AddCookie(webtest.SignIn(t, d, m, "u1"))
			m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(&access.GuildAccess{
				Config: guildconfig.Config{GuildID: guildID, Name: "Celesta Community"},
				Caps:   admin(),
			}, nil)

			// when
			rec := webtest.Serve(h, r)

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Contains(t, rec.Body.String(), "Unlock the feature with Premium. Join the Discord and get on board!")
		})
	}
}

func TestHandleSetChannels(t *testing.T) {
	cases := map[string]struct {
		err  error
		code int
		want string
	}{
		"saved": {nil, http.StatusOK, "Channels saved."},
		"stale": {ports.ErrUnknownChannel, http.StatusOK, "A selected channel is no longer on the server."},
		"error": {errors.New("db down"), http.StatusInternalServerError, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, guildconfig.Config{Premium: true}, admin())
			m.Settings.EXPECT().SetChannels(mock.Anything, guildID, "c1", "").Return(tc.err)
			form := url.Values{"command_channel_id": {" c1 "}, "summary_channel_id": {""}}

			// when
			rec := webtest.Serve(h, htmxPost("/servers/g1/channels", form, cookie))

			// then
			assert.Equal(t, tc.code, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}
