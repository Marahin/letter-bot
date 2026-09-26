package bot

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/role"
)

func TestSyncableChannels_KeepsTextNewsAndCategories(t *testing.T) {
	// given
	input := []*discordgo.Channel{
		{ID: "1", Name: "general", Type: discordgo.ChannelTypeGuildText, ParentID: "3", Position: 2},
		{ID: "2", Name: "voice", Type: discordgo.ChannelTypeGuildVoice},
		{ID: "3", Name: "Main", Type: discordgo.ChannelTypeGuildCategory},
		{ID: "4", Name: "news", Type: discordgo.ChannelTypeGuildNews},
		{ID: "5", Name: "forum", Type: discordgo.ChannelTypeGuildForum},
	}

	// when
	got := syncableChannels(input)

	// then
	assert.Equal(t, []*discord.Channel{
		{ID: "1", Name: "general", Type: discord.ChannelTypeGuildText, ParentID: "3", Position: 2},
		{ID: "3", Name: "Main", Type: discord.ChannelTypeGuildCategory},
		{ID: "4", Name: "news", Type: discord.ChannelTypeGuildNews},
	}, got)
}

func TestSyncableRoles_DropsEveryoneAndManagedRoles(t *testing.T) {
	// given
	input := []*discordgo.Role{
		{ID: "g1", Name: "@everyone"},
		{ID: "r1", Name: "Leader", Position: 5, Color: 123},
		{ID: "r2", Name: "Some Bot", Managed: true},
	}

	// when
	got := syncableRoles("g1", input)

	// then
	assert.Equal(t, []*role.Role{{ID: "r1", Name: "Leader", Position: 5, Color: 123}}, got)
}

type syncerMocks struct {
	reader   *mocks.MockDiscordGuildReader
	channels *mocks.MockGuildChannelRepository
	roles    *mocks.MockGuildRoleRepository
	configs  *mocks.MockGuildConfigRepository
}

func newTestSyncer(t *testing.T) (*GuildSyncer, syncerMocks) {
	m := syncerMocks{
		reader:   mocks.NewMockDiscordGuildReader(t),
		channels: mocks.NewMockGuildChannelRepository(t),
		roles:    mocks.NewMockGuildRoleRepository(t),
		configs:  mocks.NewMockGuildConfigRepository(t),
	}
	return NewGuildSyncer(m.reader, m.channels, m.roles, m.configs), m
}

func TestGuildSyncer_Sync(t *testing.T) {
	// given
	s, m := newTestSyncer(t)
	m.reader.On("GuildChannels", "g1").Return([]*discordgo.Channel{{ID: "c1", Name: "letter", Type: discordgo.ChannelTypeGuildText}}, nil).Once()
	m.reader.On("GuildRoles", "g1").Return([]*discordgo.Role{{ID: "g1"}, {ID: "r1", Name: "Postman"}}, nil).Once()
	m.channels.On("Replace", mocks.ContextMock, "g1", []*discord.Channel{{ID: "c1", Name: "letter"}}).Return(nil).Once()
	m.roles.On("Replace", mocks.ContextMock, "g1", []*role.Role{{ID: "r1", Name: "Postman"}}).Return(nil).Once()
	var startedAt time.Time
	m.configs.On("MarkSynced", mocks.ContextMock, "g1", mock.AnythingOfType("time.Time")).Return(nil).Once().
		Run(func(args mock.Arguments) { startedAt = args.Get(2).(time.Time) })
	before := time.Now()

	// when
	err := s.Sync(context.Background(), "g1")

	// then
	assert.NoError(t, err)
	assert.False(t, startedAt.Before(before), "startedAt is taken when the sync starts")
}

func TestGuildSyncer_Sync_Errors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name  string
		setup func(m syncerMocks)
		want  string
	}{
		{
			name: "channels fetch",
			setup: func(m syncerMocks) {
				m.reader.On("GuildChannels", "g1").Return(nil, boom)
			},
			want: "fetch channels: boom",
		},
		{
			name: "roles fetch",
			setup: func(m syncerMocks) {
				m.reader.On("GuildChannels", "g1").Return(nil, nil)
				m.reader.On("GuildRoles", "g1").Return(nil, boom)
			},
			want: "fetch roles: boom",
		},
		{
			name: "channels store",
			setup: func(m syncerMocks) {
				m.reader.On("GuildChannels", "g1").Return(nil, nil)
				m.reader.On("GuildRoles", "g1").Return(nil, nil)
				m.channels.On("Replace", mocks.ContextMock, "g1", []*discord.Channel{}).Return(boom)
			},
			want: "store channels: boom",
		},
		{
			name: "roles store",
			setup: func(m syncerMocks) {
				m.reader.On("GuildChannels", "g1").Return(nil, nil)
				m.reader.On("GuildRoles", "g1").Return(nil, nil)
				m.channels.On("Replace", mocks.ContextMock, "g1", []*discord.Channel{}).Return(nil)
				m.roles.On("Replace", mocks.ContextMock, "g1", []*role.Role{}).Return(boom)
			},
			want: "store roles: boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			s, m := newTestSyncer(t)
			tt.setup(m)

			// when
			err := s.Sync(context.Background(), "g1")

			// then
			assert.EqualError(t, err, tt.want)
		})
	}
}

func TestGuildSyncer_Schedule_CoalescesEvents(t *testing.T) {
	// given
	s, m := newTestSyncer(t)
	s.scheduler = newDebouncer(10 * time.Millisecond)
	synced := make(chan struct{}, 2)
	m.reader.On("GuildChannels", "g1").Return(nil, nil).Once()
	m.reader.On("GuildRoles", "g1").Return(nil, nil).Once()
	m.channels.On("Replace", mocks.ContextMock, "g1", []*discord.Channel{}).Return(nil).Once()
	m.roles.On("Replace", mocks.ContextMock, "g1", []*role.Role{}).Return(nil).Once()
	m.configs.On("MarkSynced", mocks.ContextMock, "g1", mock.Anything).Return(errors.New("logged only")).Once().
		Run(func(_ mock.Arguments) { synced <- struct{}{} })

	// when
	s.Schedule("g1")
	s.Schedule("g1")
	s.Schedule("g1")

	// then
	select {
	case <-synced:
	case <-time.After(time.Second):
		t.Fatal("sync did not run")
	}
	time.Sleep(30 * time.Millisecond)
	assert.Empty(t, synced)
}
