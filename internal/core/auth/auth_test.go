package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

func TestService_LoginURL(t *testing.T) {
	// given
	oauth := mocks.NewMockOAuthPort(t)
	oauth.EXPECT().AuthCodeURL("state-1").Return("https://discord.test/authorize?state=state-1")
	s := New(oauth, mocks.NewMockWebUserRepository(t))

	// when
	url := s.LoginURL("state-1")

	// then
	assert.Equal(t, "https://discord.test/authorize?state=state-1", url)
}

func TestService_Complete(t *testing.T) {
	// given
	ctx := context.Background()
	oauth := mocks.NewMockOAuthPort(t)
	oauth.EXPECT().Exchange(ctx, "code-1").Return(&webuser.User{DiscordUserID: "u1", Username: "nyx"}, nil)
	s := New(oauth, mocks.NewMockWebUserRepository(t))

	// when
	user, err := s.Complete(ctx, "code-1")

	// then
	require.NoError(t, err)
	assert.Equal(t, "u1", user.DiscordUserID)
}

func TestService_Complete_WrapsExchangeError(t *testing.T) {
	// given
	ctx := context.Background()
	oauth := mocks.NewMockOAuthPort(t)
	oauth.EXPECT().Exchange(ctx, "bad").Return(nil, ports.ErrUnauthorized)
	s := New(oauth, mocks.NewMockWebUserRepository(t))

	// when
	user, err := s.Complete(ctx, "bad")

	// then
	assert.Nil(t, user)
	assert.ErrorIs(t, err, ports.ErrUnauthorized)
}

func TestService_User(t *testing.T) {
	// given
	ctx := context.Background()
	users := mocks.NewMockWebUserRepository(t)
	users.EXPECT().Get(ctx, "u1").Return(&webuser.User{DiscordUserID: "u1"}, nil)
	users.EXPECT().Get(ctx, "u2").Return(nil, ports.ErrNotFound)
	s := New(mocks.NewMockOAuthPort(t), users)

	// when
	found, errFound := s.User(ctx, "u1")
	missing, errMissing := s.User(ctx, "u2")

	// then
	require.NoError(t, errFound)
	assert.Equal(t, "u1", found.DiscordUserID)
	assert.Nil(t, missing)
	assert.True(t, errors.Is(errMissing, ports.ErrNotFound))
}
