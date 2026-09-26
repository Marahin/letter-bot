package oauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newCaching(t *testing.T) (*Caching, *mocks.MockOAuthPort, *clock) {
	inner := mocks.NewMockOAuthPort(t)
	clk := &clock{t: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	c := NewCaching(inner, nil)
	c.now = clk.now
	return c, inner, clk
}

func TestCaching_UserGuilds_ServesFromCacheUntilTTL(t *testing.T) {
	// given
	ctx := context.Background()
	c, inner, clk := newCaching(t)
	first := []access.UserGuild{{ID: "1"}}
	second := []access.UserGuild{{ID: "2"}}
	inner.EXPECT().UserGuilds(ctx, "u1").Return(first, nil).Once()
	inner.EXPECT().UserGuilds(ctx, "u1").Return(second, nil).Once()

	// when
	a, _ := c.UserGuilds(ctx, "u1")
	clk.t = clk.t.Add(userGuildsTTL - time.Second)
	b, _ := c.UserGuilds(ctx, "u1")
	clk.t = clk.t.Add(2 * time.Second)
	d, err := c.UserGuilds(ctx, "u1")

	// then
	require.NoError(t, err)
	assert.Equal(t, first, a)
	assert.Equal(t, first, b)
	assert.Equal(t, second, d)
}

func TestCaching_UserGuilds_StaleEntries(t *testing.T) {
	cases := map[string]struct {
		refetchErr error
		servesOld  bool
	}{
		"upstream down serves stale":  {ports.ErrUpstreamUnavailable, true},
		"refused token is not served": {ports.ErrUnauthorized, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			c, inner, clk := newCaching(t)
			old := []access.UserGuild{{ID: "1"}}
			inner.EXPECT().UserGuilds(ctx, "u1").Return(old, nil).Once()
			inner.EXPECT().UserGuilds(ctx, "u1").Return(nil, tc.refetchErr).Once()
			_, _ = c.UserGuilds(ctx, "u1")
			clk.t = clk.t.Add(userGuildsTTL + time.Second)

			// when
			got, err := c.UserGuilds(ctx, "u1")

			// then
			if tc.servesOld {
				require.NoError(t, err)
				assert.Equal(t, old, got)
			} else {
				assert.ErrorIs(t, err, tc.refetchErr)
				assert.Nil(t, got)
			}
		})
	}
}

func TestCaching_UserGuilds_ColdErrorPropagatesAndIsNotCached(t *testing.T) {
	// given
	ctx := context.Background()
	c, inner, _ := newCaching(t)
	inner.EXPECT().UserGuilds(ctx, "u1").Return(nil, ports.ErrUpstreamUnavailable).Once()
	inner.EXPECT().UserGuilds(ctx, "u1").Return([]access.UserGuild{{ID: "1"}}, nil).Once()

	// when
	_, errFirst := c.UserGuilds(ctx, "u1")
	got, errSecond := c.UserGuilds(ctx, "u1")

	// then
	assert.ErrorIs(t, errFirst, ports.ErrUpstreamUnavailable)
	require.NoError(t, errSecond)
	assert.Len(t, got, 1)
}

func TestCaching_UserGuildMember(t *testing.T) {
	// given
	ctx := context.Background()
	c, inner, clk := newCaching(t)
	m1 := &access.GuildMember{Nick: "a"}
	m2 := &access.GuildMember{Nick: "b"}
	inner.EXPECT().UserGuildMember(ctx, "u1", "g1").Return(m1, nil).Once()
	inner.EXPECT().UserGuildMember(ctx, "u1", "g2").Return(m2, nil).Once()
	inner.EXPECT().UserGuildMember(ctx, "u1", "g1").Return(nil, ports.ErrUpstreamUnavailable).Once()
	inner.EXPECT().UserGuildMember(ctx, "u1", "g2").Return(nil, ports.ErrNotFound).Once()

	// when
	a, _ := c.UserGuildMember(ctx, "u1", "g1")
	b, _ := c.UserGuildMember(ctx, "u1", "g2")
	cached, _ := c.UserGuildMember(ctx, "u1", "g1")
	clk.t = clk.t.Add(memberTTL + time.Second)
	stale, errStale := c.UserGuildMember(ctx, "u1", "g1")
	left, errLeft := c.UserGuildMember(ctx, "u1", "g2")

	// then
	assert.Same(t, m1, a)
	assert.Same(t, m2, b)
	assert.Same(t, m1, cached)
	require.NoError(t, errStale)
	assert.Same(t, m1, stale)
	assert.Nil(t, left)
	assert.ErrorIs(t, errLeft, ports.ErrNotFound)
}

func TestCaching_ExchangeDropsTheUsersEntries(t *testing.T) {
	// given
	ctx := context.Background()
	c, inner, _ := newCaching(t)
	inner.EXPECT().UserGuilds(ctx, "u1").Return([]access.UserGuild{{ID: "old"}}, nil).Once()
	inner.EXPECT().UserGuildMember(ctx, "u1", "g").Return(&access.GuildMember{Nick: "old"}, nil).Once()
	inner.EXPECT().UserGuildMember(ctx, "u2", "g").Return(&access.GuildMember{Nick: "other"}, nil).Once()
	inner.EXPECT().Exchange(ctx, "code").Return(&webuser.User{DiscordUserID: "u1"}, nil)
	inner.EXPECT().UserGuilds(ctx, "u1").Return([]access.UserGuild{{ID: "new"}}, nil).Once()
	inner.EXPECT().UserGuildMember(ctx, "u1", "g").Return(&access.GuildMember{Nick: "new"}, nil).Once()
	_, _ = c.UserGuilds(ctx, "u1")
	_, _ = c.UserGuildMember(ctx, "u1", "g")
	_, _ = c.UserGuildMember(ctx, "u2", "g")

	// when
	user, err := c.Exchange(ctx, "code")
	guilds, _ := c.UserGuilds(ctx, "u1")
	member, _ := c.UserGuildMember(ctx, "u1", "g")
	other, _ := c.UserGuildMember(ctx, "u2", "g")

	// then
	require.NoError(t, err)
	assert.Equal(t, "u1", user.DiscordUserID)
	assert.Equal(t, "new", guilds[0].ID)
	assert.Equal(t, "new", member.Nick)
	assert.Equal(t, "other", other.Nick)
}

func TestCaching_PassThrough(t *testing.T) {
	// given
	ctx := context.Background()
	c, inner, _ := newCaching(t)
	inner.EXPECT().AuthCodeURL("s").Return("url")
	inner.EXPECT().Exchange(ctx, "bad").Return(nil, ports.ErrUnauthorized)

	// when
	u := c.AuthCodeURL("s")
	_, err := c.Exchange(ctx, "bad")

	// then
	assert.Equal(t, "url", u)
	assert.ErrorIs(t, err, ports.ErrUnauthorized)
}

func TestCaching_PrunesExpiredEntries(t *testing.T) {
	// given
	ctx := context.Background()
	c, inner, clk := newCaching(t)
	inner.EXPECT().UserGuilds(ctx, "new").Return(nil, nil)
	inner.EXPECT().UserGuildMember(ctx, "new", "g").Return(&access.GuildMember{}, nil)
	for i := 0; i <= pruneAbove; i++ {
		id := string(rune('a'+i%26)) + time.Duration(i).String()
		c.guilds[id] = cachedGuilds{expires: clk.t}
		c.members[memberKey{userID: id, guildID: "g"}] = cachedMember{expires: clk.t}
	}

	// when
	_, _ = c.UserGuilds(ctx, "new")
	_, _ = c.UserGuildMember(ctx, "new", "g")

	// then
	assert.Len(t, c.guilds, 1)
	assert.Len(t, c.members, 1)
}
