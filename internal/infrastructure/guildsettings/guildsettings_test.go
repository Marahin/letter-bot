package guildsettings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/guildsworld"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

type fixture struct {
	svc      *Service
	configs  *mocks.MockGuildConfigRepository
	channels *mocks.MockGuildChannelRepository
	roles    *mocks.MockGuildRoleRepository
	worlds   *mocks.MockWorldNameRepository
	notifier *mocks.MockBotNotifier
}

func newFixture(t *testing.T) fixture {
	f := fixture{
		configs:  mocks.NewMockGuildConfigRepository(t),
		channels: mocks.NewMockGuildChannelRepository(t),
		roles:    mocks.NewMockGuildRoleRepository(t),
		worlds:   mocks.NewMockWorldNameRepository(t),
		notifier: mocks.NewMockBotNotifier(t),
	}
	f.svc = New(f.configs, f.channels, f.roles, f.worlds, f.notifier, nil)
	return f
}

func syncedChannels() []*discord.Channel {
	return []*discord.Channel{
		{ID: "cat", Name: "Tibia", Type: discord.ChannelTypeGuildCategory},
		{ID: "c1", Name: "letter", Type: discord.ChannelTypeGuildText},
		{ID: "c2", Name: "news", Type: discord.ChannelTypeGuildNews},
		{ID: "v1", Name: "voice", Type: discord.ChannelTypeGuildVoice},
	}
}

func syncedRoles() []*role.Role {
	return []*role.Role{{ID: "r1", Name: "Leader"}, {ID: "r2", Name: "Member"}}
}

func TestChannels_KeepsTextAndNewsOnly(t *testing.T) {
	// given
	f := newFixture(t)
	f.channels.EXPECT().List(mock.Anything, guildID).Return(syncedChannels(), nil)

	// when
	got, err := f.svc.Channels(context.Background(), guildID)

	// then
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "c1", got[0].ID)
	assert.Equal(t, "c2", got[1].ID)
}

func TestChannels_Error(t *testing.T) {
	// given
	f := newFixture(t)
	f.channels.EXPECT().List(mock.Anything, guildID).Return(nil, errors.New("db down"))

	// when
	_, err := f.svc.Channels(context.Background(), guildID)

	// then
	assert.ErrorContains(t, err, "db down")
}

func TestRoles(t *testing.T) {
	// given
	f := newFixture(t)
	f.roles.EXPECT().List(mock.Anything, guildID).Return(syncedRoles(), nil)

	// when
	got, err := f.svc.Roles(context.Background(), guildID)

	// then
	require.NoError(t, err)
	assert.Equal(t, syncedRoles(), got)
}

func TestRoles_Error(t *testing.T) {
	// given
	f := newFixture(t)
	f.roles.EXPECT().List(mock.Anything, guildID).Return(nil, errors.New("db down"))

	// when
	_, err := f.svc.Roles(context.Background(), guildID)

	// then
	assert.ErrorContains(t, err, "db down")
}

func TestWorld(t *testing.T) {
	cases := map[string]struct {
		row     *guildsworld.GuildsWorld
		err     error
		want    string
		wantErr bool
	}{
		"stored":  {row: &guildsworld.GuildsWorld{WorldName: "Celesta"}, want: "Celesta"},
		"not set": {err: ports.ErrNotFound, want: ""},
		"error":   {err: errors.New("db down"), wantErr: true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newFixture(t)
			f.worlds.EXPECT().SelectGuildWorld(mock.Anything, guildID).Return(tc.row, tc.err)

			// when
			got, err := f.svc.World(context.Background(), guildID)

			// then
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSetChannels_StoresAndSignals(t *testing.T) {
	// given
	f := newFixture(t)
	f.channels.EXPECT().List(mock.Anything, guildID).Return(syncedChannels(), nil)
	f.configs.EXPECT().SetChannels(mock.Anything, guildID, "c1", "").Return(nil)
	f.notifier.EXPECT().ConfigChanged(mock.Anything, guildID).Return(nil)
	f.notifier.EXPECT().SummaryChanged(mock.Anything, guildID).Return(errors.New("notify failed"))

	// when
	err := f.svc.SetChannels(context.Background(), guildID, "c1", "")

	// then
	assert.NoError(t, err)
}

func TestSetChannels_RefusesUnsyncedChannel(t *testing.T) {
	cases := map[string][2]string{
		"voice channel":   {"v1", ""},
		"category":        {"", "cat"},
		"unknown channel": {"c1", "gone"},
	}
	for name, ids := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			f := newFixture(t)
			f.channels.EXPECT().List(mock.Anything, guildID).Return(syncedChannels(), nil)

			// when
			err := f.svc.SetChannels(context.Background(), guildID, ids[0], ids[1])

			// then
			assert.ErrorIs(t, err, ErrUnknownChannel)
		})
	}
}

func TestSetChannels_Errors(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		// given
		f := newFixture(t)
		f.channels.EXPECT().List(mock.Anything, guildID).Return(nil, errors.New("db down"))

		// when
		err := f.svc.SetChannels(context.Background(), guildID, "", "")

		// then
		assert.ErrorContains(t, err, "db down")
	})
	t.Run("store", func(t *testing.T) {
		// given
		f := newFixture(t)
		f.channels.EXPECT().List(mock.Anything, guildID).Return(syncedChannels(), nil)
		f.configs.EXPECT().SetChannels(mock.Anything, guildID, "", "").Return(errors.New("db down"))

		// when
		err := f.svc.SetChannels(context.Background(), guildID, "", "")

		// then
		assert.ErrorContains(t, err, "db down")
	})
}

func TestSetRoleIDs_StoresDeduplicated(t *testing.T) {
	// given
	f := newFixture(t)
	f.roles.EXPECT().List(mock.Anything, guildID).Return(syncedRoles(), nil)
	f.configs.EXPECT().SetRoleIDs(mock.Anything, guildID, guildconfig.RoleKindReserve, []string{"r2", "r1"}).Return(nil)

	// when
	err := f.svc.SetRoleIDs(context.Background(), guildID, guildconfig.RoleKindReserve, []string{"r2", "r1", "r2"})

	// then
	assert.NoError(t, err)
}

func TestSetRoleIDs_EmptyClearsTheList(t *testing.T) {
	// given
	f := newFixture(t)
	f.roles.EXPECT().List(mock.Anything, guildID).Return(nil, nil)
	f.configs.EXPECT().SetRoleIDs(mock.Anything, guildID, guildconfig.RoleKindOverbook, []string{}).Return(nil)

	// when
	err := f.svc.SetRoleIDs(context.Background(), guildID, guildconfig.RoleKindOverbook, nil)

	// then
	assert.NoError(t, err)
}

func TestSetRoleIDs_Refusals(t *testing.T) {
	t.Run("unknown kind", func(t *testing.T) {
		// given
		f := newFixture(t)

		// when
		err := f.svc.SetRoleIDs(context.Background(), guildID, "admin", []string{"r1"})

		// then
		assert.ErrorIs(t, err, ErrUnknownRoleKind)
	})
	t.Run("unknown role", func(t *testing.T) {
		// given
		f := newFixture(t)
		f.roles.EXPECT().List(mock.Anything, guildID).Return(syncedRoles(), nil)

		// when
		err := f.svc.SetRoleIDs(context.Background(), guildID, guildconfig.RoleKindManage, []string{"r1", "gone"})

		// then
		assert.ErrorIs(t, err, ErrUnknownRole)
	})
	t.Run("list error", func(t *testing.T) {
		// given
		f := newFixture(t)
		f.roles.EXPECT().List(mock.Anything, guildID).Return(nil, errors.New("db down"))

		// when
		err := f.svc.SetRoleIDs(context.Background(), guildID, guildconfig.RoleKindManage, nil)

		// then
		assert.ErrorContains(t, err, "db down")
	})
	t.Run("store error", func(t *testing.T) {
		// given
		f := newFixture(t)
		f.roles.EXPECT().List(mock.Anything, guildID).Return(syncedRoles(), nil)
		f.configs.EXPECT().SetRoleIDs(mock.Anything, guildID, guildconfig.RoleKindView, []string{"r1"}).Return(errors.New("db down"))

		// when
		err := f.svc.SetRoleIDs(context.Background(), guildID, guildconfig.RoleKindView, []string{"r1"})

		// then
		assert.ErrorContains(t, err, "db down")
	})
}

func TestSetWorld_StoresCanonicalNameAndSignals(t *testing.T) {
	// given
	f := newFixture(t)
	f.worlds.EXPECT().UpsertGuildWorld(mock.Anything, guildID, "Celesta").Return(nil)
	f.notifier.EXPECT().ConfigChanged(mock.Anything, guildID).Return(nil)

	// when
	err := f.svc.SetWorld(context.Background(), guildID, " celesta ")

	// then
	assert.NoError(t, err)
}

func TestSetWorld_Refusals(t *testing.T) {
	t.Run("unknown world", func(t *testing.T) {
		// given
		f := newFixture(t)

		// when
		err := f.svc.SetWorld(context.Background(), guildID, "Atlantis")

		// then
		assert.ErrorIs(t, err, ErrUnknownWorld)
	})
	t.Run("store error", func(t *testing.T) {
		// given
		f := newFixture(t)
		f.worlds.EXPECT().UpsertGuildWorld(mock.Anything, guildID, "Antica").Return(errors.New("db down"))

		// when
		err := f.svc.SetWorld(context.Background(), guildID, "Antica")

		// then
		assert.ErrorContains(t, err, "db down")
	})
}

func TestRequestResync_Cooldown(t *testing.T) {
	// given
	f := newFixture(t)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	f.svc.now = func() time.Time { return now }
	f.configs.EXPECT().RequestResync(mock.Anything, guildID).Return(nil).Twice()
	f.notifier.EXPECT().ResyncRequested(mock.Anything, guildID).Return(errors.New("notify failed")).Twice()

	// when
	first, err1 := f.svc.RequestResync(context.Background(), guildID)
	now = now.Add(2 * time.Minute)
	second, err2 := f.svc.RequestResync(context.Background(), guildID)
	now = now.Add(3 * time.Minute)
	third, err3 := f.svc.RequestResync(context.Background(), guildID)

	// then
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)
	assert.Zero(t, first)
	assert.Equal(t, 3*time.Minute, second)
	assert.Zero(t, third)
}

func TestRequestResync_FailureDoesNotStartTheCooldown(t *testing.T) {
	// given
	f := newFixture(t)
	f.configs.EXPECT().RequestResync(mock.Anything, guildID).Return(errors.New("db down")).Once()
	f.configs.EXPECT().RequestResync(mock.Anything, guildID).Return(nil).Once()
	f.notifier.EXPECT().ResyncRequested(mock.Anything, guildID).Return(nil).Once()

	// when
	_, err1 := f.svc.RequestResync(context.Background(), guildID)
	retry, err2 := f.svc.RequestResync(context.Background(), guildID)

	// then
	assert.ErrorContains(t, err1, "db down")
	require.NoError(t, err2)
	assert.Zero(t, retry)
}
