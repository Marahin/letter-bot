package guildaccess

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/ports"
)

const (
	userID  = "u1"
	adminID = "site-admin"
)

type fixture struct {
	oauth   *mocks.MockOAuthPort
	configs *mocks.MockGuildConfigRepository
	roles   *mocks.MockGuildRoleRepository
	svc     *Service
}

func newFixture(t *testing.T) fixture {
	f := fixture{
		oauth:   mocks.NewMockOAuthPort(t),
		configs: mocks.NewMockGuildConfigRepository(t),
		roles:   mocks.NewMockGuildRoleRepository(t),
	}
	f.svc = New(f.oauth, f.configs, f.roles, []string{" " + adminID + " ", ""}, nil)
	return f
}

func cfg(id, name string, mutate ...func(*guildconfig.Config)) *guildconfig.Config {
	c := &guildconfig.Config{GuildID: id, Name: name, BotPresent: true}
	for _, m := range mutate {
		m(c)
	}
	return c
}

func TestService_IsSiteAdmin(t *testing.T) {
	// given
	f := newFixture(t)

	// when / then
	assert.True(t, f.svc.IsSiteAdmin(adminID))
	assert.False(t, f.svc.IsSiteAdmin(userID))
	assert.False(t, f.svc.IsSiteAdmin(""))
}

func TestService_AccessibleGuilds_SiteAdminGetsEveryStoredGuild(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.configs.EXPECT().ListAll(ctx).Return([]*guildconfig.Config{
		cfg("1", "Alpha"),
		cfg("2", "beta"),
		cfg("3", "gone", func(c *guildconfig.Config) { c.BotPresent = false }),
	}, nil)

	// when
	list, err := f.svc.AccessibleGuilds(ctx, adminID)

	// then
	require.Len(t, list, 3)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2", "3"}, []string{list[0].Config.GuildID, list[1].Config.GuildID, list[2].Config.GuildID})
	assert.Equal(t, fullCapabilities, list[2].Caps)
}

func TestService_AccessibleGuilds_SiteAdminListError(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.configs.EXPECT().ListAll(ctx).Return(nil, errors.New("db down"))

	// when
	list, err := f.svc.AccessibleGuilds(ctx, adminID)

	// then
	assert.Nil(t, list)
	assert.ErrorContains(t, err, "db down")
}

func TestService_AccessibleGuilds(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{
		{ID: "admin", Name: "Admin Guild", Admin: true},
		{ID: "open", Name: "Open Guild", Icon: "oauth-icon"},
		{ID: "ranked", Name: "Ranked"},
		{ID: "denied", Name: "Denied"},
		{ID: "absent", Name: "Bot Absent"},
		{ID: "unknown", Name: "Not Stored"},
		{ID: "left", Name: "Left"},
		{ID: "broken", Name: "Broken"},
	}, nil)
	f.configs.EXPECT().ListPresentByIDs(ctx, []string{"admin", "open", "ranked", "denied", "absent", "unknown", "left", "broken"}).Return([]*guildconfig.Config{
		cfg("open", ""),
		cfg("broken", "Broken", func(c *guildconfig.Config) { c.ReserveRoleIDs = []string{"r"} }),
		cfg("denied", "Denied", func(c *guildconfig.Config) { c.ReserveRoleIDs = []string{"r"} }),
		cfg("left", "Left", func(c *guildconfig.Config) { c.ReserveRoleIDs = []string{"r"} }),
		cfg("ranked", "Ranked", func(c *guildconfig.Config) { c.ReserveRoleIDs = []string{"r"} }),
		cfg("admin", "Zeta", func(c *guildconfig.Config) { c.ReserveRoleIDs = []string{"r"} }),
	}, nil)
	f.oauth.EXPECT().UserGuildMember(ctx, userID, "ranked").Return(&access.GuildMember{RoleIDs: []string{"r"}}, nil)
	f.oauth.EXPECT().UserGuildMember(ctx, userID, "denied").Return(&access.GuildMember{RoleIDs: []string{"x"}}, nil)
	f.oauth.EXPECT().UserGuildMember(ctx, userID, "left").Return(nil, ports.ErrNotFound)
	f.oauth.EXPECT().UserGuildMember(ctx, userID, "broken").Return(nil, ports.ErrUpstreamUnavailable)

	// when
	list, err := f.svc.AccessibleGuilds(ctx, userID)

	// then
	require.NoError(t, err)
	names := make([]string, 0, len(list))
	for _, a := range list {
		names = append(names, a.Config.Name)
	}
	assert.Equal(t, []string{"Open Guild", "Ranked", "Zeta"}, names)
	assert.Equal(t, "oauth-icon", list[0].Config.Icon)
	assert.Equal(t, permission.Capabilities{View: true, Reserve: true}, list[0].Caps)
	assert.True(t, list[2].Caps.Admin)
}

func TestService_AccessibleGuilds_KeepsTheStoredOrder(t *testing.T) {
	// given stored names that differ from the Discord ones
	ctx := context.Background()
	f := newFixture(t)
	f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "a", Name: "Zulu"}, {ID: "b", Name: "Alpha"}}, nil)
	f.configs.EXPECT().ListPresentByIDs(ctx, []string{"a", "b"}).Return([]*guildconfig.Config{cfg("b", "Bravo"), cfg("a", "Charlie")}, nil)

	// when
	list, err := f.svc.AccessibleGuilds(ctx, userID)

	// then
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "Bravo", list[0].Config.Name)
	assert.Equal(t, "Charlie", list[1].Config.Name)
}

func TestService_AccessibleGuilds_NoGuilds(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.oauth.EXPECT().UserGuilds(ctx, userID).Return(nil, nil)

	// when
	list, err := f.svc.AccessibleGuilds(ctx, userID)

	// then
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestService_AccessibleGuilds_Errors(t *testing.T) {
	t.Run("user guilds", func(t *testing.T) {
		// given
		ctx := context.Background()
		f := newFixture(t)
		f.oauth.EXPECT().UserGuilds(ctx, userID).Return(nil, ports.ErrUpstreamUnavailable)

		// when
		_, err := f.svc.AccessibleGuilds(ctx, userID)

		// then
		assert.ErrorIs(t, err, ports.ErrUpstreamUnavailable)
	})
	t.Run("configs", func(t *testing.T) {
		// given
		ctx := context.Background()
		f := newFixture(t)
		f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "1"}}, nil)
		f.configs.EXPECT().ListPresentByIDs(ctx, []string{"1"}).Return(nil, errors.New("db down"))

		// when
		_, err := f.svc.AccessibleGuilds(ctx, userID)

		// then
		assert.ErrorContains(t, err, "db down")
	})
}

func TestService_Access_SiteAdmin(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G", func(c *guildconfig.Config) { c.BotPresent = false }), nil)

	// when
	a, err := f.svc.Access(ctx, adminID, "g")

	// then
	require.NoError(t, err)
	assert.Equal(t, fullCapabilities, a.Caps)
}

func TestService_Access_GuildAdmin(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G"), nil)
	f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "other"}, {ID: "g", Admin: true}}, nil)

	// when
	a, err := f.svc.Access(ctx, userID, "g")

	// then
	require.NoError(t, err)
	assert.Equal(t, permission.Capabilities{Admin: true, Manage: true, View: true, Reserve: true, Overbook: true}, a.Caps)
}

func TestService_Access_MemberWithRanks(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G", func(c *guildconfig.Config) {
		c.ManageRoleIDs = []string{"m"}
		c.OverbookRoleIDs = []string{"o"}
	}), nil)
	f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "g"}}, nil)
	f.oauth.EXPECT().UserGuildMember(ctx, userID, "g").Return(&access.GuildMember{RoleIDs: []string{"o"}}, nil)

	// when
	a, err := f.svc.Access(ctx, userID, "g")

	// then
	require.NoError(t, err)
	assert.Equal(t, permission.Capabilities{View: true, Reserve: true, Overbook: true}, a.Caps)
}

func TestService_Access_PostmanRoleWhenNoOverbookRank(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G"), nil)
	f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "g"}}, nil)
	f.oauth.EXPECT().UserGuildMember(ctx, userID, "g").Return(&access.GuildMember{RoleIDs: []string{"p"}}, nil)
	f.roles.EXPECT().List(ctx, "g").Return([]*role.Role{{ID: "p", Name: "Postman"}}, nil)

	// when
	a, err := f.svc.Access(ctx, userID, "g")

	// then
	require.NoError(t, err)
	assert.True(t, a.Caps.Overbook)
	assert.False(t, a.Caps.Manage)
}

func TestService_Access_NotFound(t *testing.T) {
	cases := map[string]func(ctx context.Context, f fixture){
		"guild not stored": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(nil, ports.ErrNotFound)
		},
		"bot absent": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G", func(c *guildconfig.Config) { c.BotPresent = false }), nil)
		},
		"not in the user's guild list": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G"), nil)
			f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "other", Admin: true}}, nil)
		},
		"member lookup says not a member": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G"), nil)
			f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "g"}}, nil)
			f.oauth.EXPECT().UserGuildMember(ctx, userID, "g").Return(nil, ports.ErrNotFound)
		},
		"no rank grants view": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G", func(c *guildconfig.Config) {
				c.ReserveRoleIDs = []string{"r"}
				c.OverbookRoleIDs = []string{"o"}
			}), nil)
			f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "g"}}, nil)
			f.oauth.EXPECT().UserGuildMember(ctx, userID, "g").Return(&access.GuildMember{RoleIDs: []string{"x"}}, nil)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			f := newFixture(t)
			setup(ctx, f)

			// when
			a, err := f.svc.Access(ctx, userID, "g")

			// then
			assert.Nil(t, a)
			assert.ErrorIs(t, err, ports.ErrNotFound)
		})
	}
}

func TestService_Access_Errors(t *testing.T) {
	cases := map[string]func(ctx context.Context, f fixture){
		"config": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(nil, ports.ErrUpstreamUnavailable)
		},
		"user guilds": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G"), nil)
			f.oauth.EXPECT().UserGuilds(ctx, userID).Return(nil, ports.ErrUpstreamUnavailable)
		},
		"member": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G"), nil)
			f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "g"}}, nil)
			f.oauth.EXPECT().UserGuildMember(ctx, userID, "g").Return(nil, ports.ErrUpstreamUnavailable)
		},
		"roles": func(ctx context.Context, f fixture) {
			f.configs.EXPECT().Get(ctx, "g").Return(cfg("g", "G"), nil)
			f.oauth.EXPECT().UserGuilds(ctx, userID).Return([]access.UserGuild{{ID: "g"}}, nil)
			f.oauth.EXPECT().UserGuildMember(ctx, userID, "g").Return(&access.GuildMember{}, nil)
			f.roles.EXPECT().List(ctx, "g").Return(nil, ports.ErrUpstreamUnavailable)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			ctx := context.Background()
			f := newFixture(t)
			setup(ctx, f)

			// when
			a, err := f.svc.Access(ctx, userID, "g")

			// then
			assert.Nil(t, a)
			assert.ErrorIs(t, err, ports.ErrUpstreamUnavailable)
			assert.NotErrorIs(t, err, ports.ErrNotFound)
		})
	}
}

func TestService_Member(t *testing.T) {
	// given
	ctx := context.Background()
	f := newFixture(t)
	f.oauth.EXPECT().UserGuildMember(ctx, userID, "g").Return(&access.GuildMember{Nick: "Nyx"}, nil)

	// when
	m, err := f.svc.Member(ctx, userID, "g")

	// then
	require.NoError(t, err)
	assert.Equal(t, "Nyx", m.Nick)
	f.oauth.AssertNotCalled(t, "UserGuilds", mock.Anything, mock.Anything)
}
