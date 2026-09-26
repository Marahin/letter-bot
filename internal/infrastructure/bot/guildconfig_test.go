package bot

import (
	"context"
	"errors"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/ports"
)

func TestCommandGateReply(t *testing.T) {
	premium := &guildconfig.Config{Premium: true}
	forever := &guildconfig.Config{PremiumForever: true}
	withChannel := &guildconfig.Config{Premium: true, CommandChannelID: "c1"}

	tests := []struct {
		name      string
		cfg       *guildconfig.Config
		command   string
		channelID string
		want      string
	}{
		{"not premium", &guildconfig.Config{}, "summary", "any", notPremiumMessage("https://letter.example")},
		{"premium forever without command channel works anywhere", forever, "book", "any", ""},
		{"premium without command channel works anywhere", premium, "unbook", "any", ""},
		{"book in the command channel", withChannel, "book", "c1", ""},
		{"book outside the command channel", withChannel, "book", "c2", "Please use this command in <#c1>."},
		{"unbook outside the command channel", withChannel, "unbook", "c2", "Please use this command in <#c1>."},
		{"summary outside the command channel", withChannel, "summary", "c2", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given / when
			got := commandGateReply(tt.cfg, tt.command, tt.channelID, "https://letter.example")

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNotPremiumMessage(t *testing.T) {
	// given / when / then
	assert.NotContains(t, notPremiumMessage(""), "See")
	assert.Contains(t, notPremiumMessage("https://letter.example"), "https://letter.example")
}

func TestMemberSubject(t *testing.T) {
	// given
	roles := []*role.Role{{ID: "r1", Name: "Postman"}}
	g := &guild.Guild{OwnerID: "owner", Roles: roles}

	tests := []struct {
		name  string
		m     *member.Member
		admin bool
	}{
		{"owner", &member.Member{ID: "owner"}, true},
		{"administrator", &member.Member{ID: "m1", Permissions: discordgo.PermissionAdministrator | discordgo.PermissionSendMessages}, true},
		{"regular member", &member.Member{ID: "m2", Roles: []string{"r1"}, Permissions: discordgo.PermissionSendMessages}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			s := memberSubject(g, tt.m)

			// then
			assert.Equal(t, tt.admin, s.IsAdmin)
			assert.Equal(t, tt.m.Roles, s.RoleIDs)
			assert.Equal(t, roles, s.GuildRoles)
		})
	}
}

func TestMemberSubject_EmptyOwnerIsNotAdmin(t *testing.T) {
	// given / when
	s := memberSubject(&guild.Guild{}, &member.Member{})

	// then
	assert.False(t, s.IsAdmin)
}

func TestLegacyChannelsToCreate(t *testing.T) {
	existingBoth := []*discordgo.Channel{{Name: discord.SummaryChannel}, {Name: discord.CommandChannel}}

	tests := []struct {
		name     string
		cfg      *guildconfig.Config
		existing []*discordgo.Channel
		want     []string
	}{
		{"nothing configured, nothing exists", &guildconfig.Config{}, nil, []string{discord.SummaryChannel, discord.CommandChannel}},
		{"nothing configured, both exist", &guildconfig.Config{}, existingBoth, nil},
		{"summary configured", &guildconfig.Config{SummaryChannelID: "s"}, nil, []string{discord.CommandChannel}},
		{"command configured", &guildconfig.Config{CommandChannelID: "c"}, nil, []string{discord.SummaryChannel}},
		{"both configured", &guildconfig.Config{SummaryChannelID: "s", CommandChannelID: "c"}, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given / when
			got := legacyChannelsToCreate(tt.cfg, tt.existing)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func newConfigBot(t *testing.T) (*Bot, *mocks.MockGuildConfigRepository) {
	configs := mocks.NewMockGuildConfigRepository(t)
	return &Bot{guildConfigs: configs, log: zap.NewNop().Sugar()}, configs
}

func TestBot_GuildConfig(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	stored := &guildconfig.Config{GuildID: "g1", PremiumForever: true}
	configs.On("Get", mocks.ContextMock, "g1").Return(stored, nil).Once()
	configs.On("Get", mocks.ContextMock, "g2").Return(nil, ports.ErrNotFound).Once()
	configs.On("Get", mocks.ContextMock, "g3").Return(nil, errors.New("db down")).Once()

	// when
	got1, err1 := b.guildConfig(context.Background(), "g1")
	got2, err2 := b.guildConfig(context.Background(), "g2")
	_, err3 := b.guildConfig(context.Background(), "g3")

	// then
	assert.NoError(t, err1)
	assert.Same(t, stored, got1)
	assert.NoError(t, err2)
	assert.Equal(t, &guildconfig.Config{GuildID: "g2"}, got2)
	assert.False(t, got2.IsPremium())
	assert.EqualError(t, err3, "db down")
}

func TestBot_GuildConfigsByID(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	configs.On("ListByIDs", mocks.ContextMock, []string{"g1", "g2"}).
		Return([]*guildconfig.Config{{GuildID: "g1", Premium: true}}, nil).Once()

	// when
	got, err := b.guildConfigsByID(context.Background(), []string{"g1", "g2"})

	// then
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.True(t, got["g1"].IsPremium())
	assert.Nil(t, got["g2"])
}

func TestBot_GuildConfigsByID_Error(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	configs.On("ListByIDs", mocks.ContextMock, []string{"g1"}).Return(nil, errors.New("db down")).Once()

	// when
	got, err := b.guildConfigsByID(context.Background(), []string{"g1"})

	// then
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestBot_ProcessResyncRequests(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	s, m := newTestSyncer(t)
	s.configs = configs
	b.syncer = s
	configs.On("ListResyncRequested", mocks.ContextMock).Return([]string{"g1", "g2"}, nil).Once()
	m.reader.On("GuildChannels", "g1").Return(nil, nil).Once()
	m.reader.On("GuildRoles", "g1").Return(nil, nil).Once()
	m.channels.On("Replace", mocks.ContextMock, "g1", []*discord.Channel{}).Return(nil).Once()
	m.roles.On("Replace", mocks.ContextMock, "g1", []*role.Role{}).Return(nil).Once()
	configs.On("MarkSynced", mocks.ContextMock, "g1", mock.Anything).Return(nil).Once()
	m.reader.On("GuildChannels", "g2").Return(nil, errors.New("missing access")).Once()

	// when
	b.processResyncRequests(context.Background())

	// then: g1 is marked synced, g2 keeps its request for the next tick
	configs.AssertNotCalled(t, "MarkSynced", mocks.ContextMock, "g2", mock.Anything)
}

func TestBot_ProcessResyncRequests_ListError(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	configs.On("ListResyncRequested", mocks.ContextMock).Return(nil, errors.New("db down")).Once()

	// when / then: no sync is attempted
	b.processResyncRequests(context.Background())
}

func TestBot_ApplyGuildConfig_NotPremium(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	online := mocks.NewMockOnlineCheckService(t)
	b.onlineCheckService = online
	online.On("ConfigureWorldNameForGuild", "g1").Return(errors.New("no world")).Once()
	configs.On("Get", mocks.ContextMock, "g1").Return(&guildconfig.Config{GuildID: "g1"}, nil).Once()

	// when / then: the guild is not touched on Discord (b.mgr is nil)
	b.ApplyGuildConfig(context.Background(), "g1")
}

func TestBot_ApplyGuildConfig_ConfigError(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	online := mocks.NewMockOnlineCheckService(t)
	b.onlineCheckService = online
	online.On("ConfigureWorldNameForGuild", "g1").Return(nil).Once()
	configs.On("Get", mocks.ContextMock, "g1").Return(nil, errors.New("db down")).Once()

	// when / then
	b.ApplyGuildConfig(context.Background(), "g1")
}

func TestBot_StorePresence(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	g := &guild.Guild{ID: "g1", Name: "Celesta", Icon: "icon", OwnerID: "owner"}
	stored := &guildconfig.Config{GuildID: "g1", Premium: true}
	configs.On("UpsertPresence", mocks.ContextMock, "g1", "Celesta", "icon", "owner").Return(stored, nil).Once()

	// when
	got := b.storePresence(context.Background(), g)

	// then
	assert.Same(t, stored, got)
}

func TestBot_StorePresence_FallsBackToStoredConfig(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	g := &guild.Guild{ID: "g1"}
	stored := &guildconfig.Config{GuildID: "g1", Premium: true}
	configs.On("UpsertPresence", mocks.ContextMock, "g1", "", "", "").Return(nil, errors.New("db down")).Once()
	configs.On("Get", mocks.ContextMock, "g1").Return(stored, nil).Once()

	// when
	got := b.storePresence(context.Background(), g)

	// then
	assert.Same(t, stored, got)
	assert.True(t, got.IsPremium())
}

func TestBot_StorePresence_BothFail(t *testing.T) {
	// given
	b, configs := newConfigBot(t)
	configs.On("UpsertPresence", mocks.ContextMock, "g1", "", "", "").Return(nil, errors.New("db down")).Once()
	configs.On("Get", mocks.ContextMock, "g1").Return(nil, errors.New("db down")).Once()

	// when
	got := b.storePresence(context.Background(), &guild.Guild{ID: "g1"})

	// then
	assert.Nil(t, got)
}

func TestBot_MarkAbsentGuilds(t *testing.T) {
	cases := map[string]struct {
		shard             *[2]int
		expectedID, count int
	}{
		"unsharded":  {shard: nil, expectedID: 0, count: 1},
		"one shard":  {shard: &[2]int{1, 3}, expectedID: 1, count: 3},
		"zero count": {shard: &[2]int{0, 0}, expectedID: 0, count: 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			b, configs := newConfigBot(t)
			ready := &discordgo.Ready{Shard: tc.shard, Guilds: []*discordgo.Guild{{ID: "g1"}, {ID: "g2"}}}
			configs.On("MarkAbsentExcept", mocks.ContextMock, tc.expectedID, tc.count, []string{"g1", "g2"}).Return(errors.New("db down")).Once()

			// when
			b.markAbsentGuilds(ready)

			// then: expectations are asserted on cleanup; the error is only logged
		})
	}
}
