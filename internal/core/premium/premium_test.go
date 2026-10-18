package premium

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/ports"
)

func newService(t *testing.T) (*Service, *mocks.MockGuildConfigRepository, *mocks.MockBotNotifier) {
	configs := mocks.NewMockGuildConfigRepository(t)
	notifier := mocks.NewMockBotNotifier(t)
	return New(configs, notifier, nil), configs, notifier
}

func TestService_List(t *testing.T) {
	// given
	ctx := context.Background()
	s, configs, _ := newService(t)
	configs.EXPECT().ListAll(ctx).Return([]*guildconfig.Config{{GuildID: "g"}}, nil)

	// when
	list, err := s.List(ctx)

	// then
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestService_SetPremium_TurnsOnAndNotifies(t *testing.T) {
	// given
	ctx := context.Background()
	s, configs, notifier := newService(t)
	configs.EXPECT().Get(ctx, "g").Return(&guildconfig.Config{GuildID: "g"}, nil)
	configs.EXPECT().SetPremium(ctx, "g", true).Return(nil)
	notifier.EXPECT().ConfigChanged(ctx, "g").Return(nil)

	// when
	cfg, err := s.SetPremium(ctx, "g", true)

	// then
	assert.NoError(t, err)
	assert.True(t, cfg.Premium)
}

func TestService_SetPremium_NotifierFailureIsNotAnError(t *testing.T) {
	// given
	ctx := context.Background()
	s, configs, notifier := newService(t)
	configs.EXPECT().Get(ctx, "g").Return(&guildconfig.Config{GuildID: "g", Premium: true}, nil)
	configs.EXPECT().SetPremium(ctx, "g", false).Return(nil)
	notifier.EXPECT().ConfigChanged(ctx, "g").Return(errors.New("conn closed"))

	// when
	cfg, err := s.SetPremium(ctx, "g", false)

	// then
	assert.NoError(t, err)
	assert.False(t, cfg.Premium)
}

func TestService_SetPremium_Unchanged(t *testing.T) {
	// given
	ctx := context.Background()
	s, configs, _ := newService(t)
	configs.EXPECT().Get(ctx, "g").Return(&guildconfig.Config{GuildID: "g", Premium: true}, nil)

	// when
	cfg, err := s.SetPremium(ctx, "g", true)

	// then
	assert.NoError(t, err)
	assert.True(t, cfg.Premium)
}

func TestService_SetPremium_RefusesToTurnOffForever(t *testing.T) {
	// given
	ctx := context.Background()
	s, configs, _ := newService(t)
	configs.EXPECT().Get(ctx, "g").Return(&guildconfig.Config{GuildID: "g", Premium: true, PremiumForever: true}, nil)

	// when
	_, err := s.SetPremium(ctx, "g", false)

	// then
	assert.ErrorIs(t, err, ErrPremiumForever)
}

func TestService_SetPremium_Errors(t *testing.T) {
	t.Run("unknown guild", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, configs, _ := newService(t)
		configs.EXPECT().Get(ctx, "g").Return(nil, ports.ErrNotFound)

		// when
		_, err := s.SetPremium(ctx, "g", true)

		// then
		assert.ErrorIs(t, err, ports.ErrNotFound)
	})
	t.Run("write", func(t *testing.T) {
		// given
		ctx := context.Background()
		s, configs, _ := newService(t)
		configs.EXPECT().Get(ctx, "g").Return(&guildconfig.Config{GuildID: "g"}, nil)
		configs.EXPECT().SetPremium(ctx, "g", true).Return(errors.New("db down"))

		// when
		_, err := s.SetPremium(ctx, "g", true)

		// then
		assert.ErrorContains(t, err, "db down")
	})
}
