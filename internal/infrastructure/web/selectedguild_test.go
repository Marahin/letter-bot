package web

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
)

func TestSelectedGuildID(t *testing.T) {
	free := guild("free", "Free", false, viewerCaps)
	paid := guild("paid", "Paid", true, viewerCaps)
	other := guild("other", "Other", false, viewerCaps)
	cases := map[string]struct {
		user     *webuser.User
		list     []access.GuildAccess
		expected string
	}{
		"no servers":                     {user: &webuser.User{DefaultGuildID: "free"}, expected: ""},
		"remembered server":              {user: &webuser.User{DefaultGuildID: "other"}, list: []access.GuildAccess{free, paid, other}, expected: "other"},
		"remembered server is gone":      {user: &webuser.User{DefaultGuildID: "gone"}, list: []access.GuildAccess{free, paid}, expected: "paid"},
		"nothing remembered, premium":    {user: &webuser.User{}, list: []access.GuildAccess{free, paid}, expected: "paid"},
		"nothing remembered, no premium": {user: &webuser.User{}, list: []access.GuildAccess{free, other}, expected: "free"},
		"no user":                        {list: []access.GuildAccess{free}, expected: "free"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// when
			got := selectedGuildID(tc.user, tc.list)

			// then
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestDashboard_PicksAServerWithoutARememberedOne(t *testing.T) {
	// given
	f := newAuthFixture(t)
	cookie := f.signInUser(t, &webuser.User{DiscordUserID: "u1", Username: "nyx"},
		guild("g1", "Free", false, viewerCaps), guild("g2", "Paid", true, viewerCaps))
	f.access.EXPECT().IsSiteAdmin("u1").Return(false).Maybe()

	// when
	rec := f.do(htmlGet("/dashboard"), cookie)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "data-server-picker")
	assert.Contains(t, body, `href="/servers/g2/reservations"`)
	assert.NotContains(t, body, "Choose a server")
}

func TestGuildPage_RemembersTheServerOnlyOnChange(t *testing.T) {
	// given
	f := newAuthFixture(t)
	g2 := guild("g2", "Paid", true, viewerCaps)
	cookie := f.signInUser(t, &webuser.User{DiscordUserID: "u1", Username: "nyx", DefaultGuildID: "g1"})
	remembered := f.signInUser(t, &webuser.User{DiscordUserID: "u2", Username: "ash", DefaultGuildID: "g2"})
	f.access.EXPECT().Access(mock.Anything, mock.Anything, "g2").Return(&g2, nil)
	f.auth.EXPECT().SetDefaultGuild(mock.Anything, "u1", "g2").Return(nil).Once()

	// when
	changed := f.do(htmlGet("/servers/g2/reservations"), cookie)
	unchanged := f.do(htmlGet("/servers/g2/reservations"), remembered)

	// then
	assert.Equal(t, http.StatusOK, changed.Code)
	assert.Equal(t, http.StatusOK, unchanged.Code)
}

func TestGuildPage_RendersWhenRememberingFails(t *testing.T) {
	// given
	f := newAuthFixture(t)
	g2 := guild("g2", "Paid", true, viewerCaps)
	cookie := f.signInUser(t, &webuser.User{DiscordUserID: "u1", Username: "nyx"})
	f.access.EXPECT().Access(mock.Anything, "u1", "g2").Return(&g2, nil)
	f.auth.EXPECT().SetDefaultGuild(mock.Anything, "u1", "g2").Return(assert.AnError).Once()

	// when
	rec := f.do(htmlGet("/servers/g2/reservations"), cookie)

	// then
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "feature ok", rec.Body.String())
}
