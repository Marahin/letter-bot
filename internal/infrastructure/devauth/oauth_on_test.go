//go:build devauth

package devauth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/common/test/mocks"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

type oauthFixture struct {
	real    *mocks.MockOAuthPort
	configs *mocks.MockGuildConfigRepository
	users   *mocks.MockWebUserRepository
	m       *MockOAuth
}

func newOAuthFixture(t *testing.T) oauthFixture {
	f := oauthFixture{
		real:    mocks.NewMockOAuthPort(t),
		configs: mocks.NewMockGuildConfigRepository(t),
		users:   mocks.NewMockWebUserRepository(t),
	}
	f.m = NewMockOAuth(f.real, f.configs, f.users)
	return f
}

const (
	memberID = "700000000000000002"
	ownerUID = "700000000000000005"
)

func TestMockOAuth_AuthCodeURLDelegates(t *testing.T) {
	// given
	f := newOAuthFixture(t)
	f.real.EXPECT().AuthCodeURL("s").Return("https://discord.test/?state=s")

	// then
	assert.Equal(t, "https://discord.test/?state=s", f.m.AuthCodeURL("s"))
}

func TestMockOAuth_ExchangeStoresTheMockUser(t *testing.T) {
	// given
	ctx := context.Background()
	f := newOAuthFixture(t)
	want := webuser.User{DiscordUserID: memberID, Username: "dev-member", GlobalName: "Member (view and reserve)"}
	f.users.EXPECT().Upsert(ctx, want).Return(&want, nil)

	// when
	user, err := f.m.Exchange(ctx, DevCodePrefix+memberID)

	// then
	require.NoError(t, err)
	assert.Equal(t, memberID, user.DiscordUserID)
}

func TestMockOAuth_ExchangeRefusesAnUnknownMockUser(t *testing.T) {
	// when
	_, err := newOAuthFixture(t).m.Exchange(context.Background(), DevCodePrefix+"1")

	// then
	assert.ErrorIs(t, err, ports.ErrUnauthorized)
}

func TestMockOAuth_ExchangeStoreError(t *testing.T) {
	// given
	ctx := context.Background()
	f := newOAuthFixture(t)
	f.users.EXPECT().Upsert(ctx, webuser.User{DiscordUserID: memberID, Username: "dev-member", GlobalName: "Member (view and reserve)"}).Return(nil, assert.AnError)

	// when
	_, err := f.m.Exchange(ctx, DevCodePrefix+memberID)

	// then
	assert.ErrorIs(t, err, assert.AnError)
}

func TestMockOAuth_RealUsersFallThrough(t *testing.T) {
	// given
	ctx := context.Background()
	f := newOAuthFixture(t)
	f.real.EXPECT().Exchange(ctx, "real-code").Return(&webuser.User{DiscordUserID: "42"}, nil)
	f.real.EXPECT().UserGuilds(ctx, "42").Return([]access.UserGuild{{ID: "g"}}, nil)
	f.real.EXPECT().UserGuildMember(ctx, "42", "g").Return(&access.GuildMember{}, nil)

	// when
	user, exchangeErr := f.m.Exchange(ctx, "real-code")
	guilds, guildsErr := f.m.UserGuilds(ctx, "42")
	_, memberErr := f.m.UserGuildMember(ctx, "42", "g")

	// then
	require.NoError(t, exchangeErr)
	require.NoError(t, guildsErr)
	require.NoError(t, memberErr)
	assert.Equal(t, "42", user.DiscordUserID)
	assert.Len(t, guilds, 1)
}

func TestMockOAuth_UserGuildsListsTheSeededServers(t *testing.T) {
	// given the premium server stored and the locked one gone
	ctx := context.Background()
	f := newOAuthFixture(t)
	f.configs.EXPECT().Get(ctx, PremiumGuildID).Return(&guildconfig.Config{GuildID: PremiumGuildID, Name: "Letter E2E", BotPresent: true}, nil)
	f.configs.EXPECT().Get(ctx, LockedGuildID).Return(nil, ports.ErrNotFound)

	// when
	guilds, err := f.m.UserGuilds(ctx, ownerUID)

	// then
	require.NoError(t, err)
	assert.Equal(t, []access.UserGuild{{ID: PremiumGuildID, Name: "Letter E2E", Admin: true}}, guilds)
}

func TestMockOAuth_UserGuildsSkipsABotAbsentServerAndFailsOnAnError(t *testing.T) {
	// given
	ctx := context.Background()
	f := newOAuthFixture(t)
	f.configs.EXPECT().Get(ctx, PremiumGuildID).Return(&guildconfig.Config{GuildID: PremiumGuildID}, nil)
	f.configs.EXPECT().Get(ctx, LockedGuildID).Return(nil, assert.AnError)

	// when
	_, err := f.m.UserGuilds(ctx, memberID)

	// then
	assert.ErrorIs(t, err, assert.AnError)
}

func TestMockOAuth_UserGuildMember(t *testing.T) {
	// given
	ctx := context.Background()
	f := newOAuthFixture(t)

	// when
	manager, err := f.m.UserGuildMember(ctx, "700000000000000001", PremiumGuildID)
	_, outsiderErr := f.m.UserGuildMember(ctx, "700000000000000003", PremiumGuildID)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{ManageRoleID}, manager.RoleIDs)
	assert.ErrorIs(t, outsiderErr, ports.ErrNotFound)
}
