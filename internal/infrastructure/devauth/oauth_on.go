//go:build devauth

package devauth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/webuser"
	"spot-assistant/internal/ports"
)

// MockOAuth is the dev-only OAuth port: a mock user (signed in through
// /dev/login) resolves to the seeded servers without Discord. Every other user
// falls through to the real port, so a real sign-in works beside the bypass.
type MockOAuth struct {
	real    ports.OAuthPort
	configs ports.GuildConfigRepository
	users   ports.WebUserRepository
}

func NewMockOAuth(realPort ports.OAuthPort, configs ports.GuildConfigRepository, users ports.WebUserRepository) *MockOAuth {
	return &MockOAuth{real: realPort, configs: configs, users: users}
}

func (m *MockOAuth) AuthCodeURL(state string) string { return m.real.AuthCodeURL(state) }

// Exchange stores the mock user for a dev code, as the real exchange stores a Discord user.
func (m *MockOAuth) Exchange(ctx context.Context, code string) (*webuser.User, error) {
	id, ok := strings.CutPrefix(code, DevCodePrefix)
	if !ok {
		return m.real.Exchange(ctx, code)
	}
	mock, ok := ByID(id)
	if !ok {
		return nil, ports.ErrUnauthorized
	}
	user, err := m.users.Upsert(ctx, webuser.User{DiscordUserID: mock.ID, Username: mock.Username, GlobalName: mock.Label})
	if err != nil {
		return nil, fmt.Errorf("store mock user: %w", err)
	}
	return user, nil
}

// UserGuilds lists the stored, bot-present servers the mock user is a member of.
func (m *MockOAuth) UserGuilds(ctx context.Context, userID string) ([]access.UserGuild, error) {
	mock, ok := ByID(userID)
	if !ok {
		return m.real.UserGuilds(ctx, userID)
	}
	var out []access.UserGuild
	for _, id := range mock.Guilds {
		cfg, err := m.configs.Get(ctx, id)
		if errors.Is(err, ports.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("load seeded guild: %w", err)
		}
		if cfg.BotPresent {
			out = append(out, access.UserGuild{ID: cfg.GuildID, Name: cfg.Name, Icon: cfg.Icon, Admin: mock.Admin})
		}
	}
	return out, nil
}

func (m *MockOAuth) UserGuildMember(ctx context.Context, userID, guildID string) (*access.GuildMember, error) {
	mock, ok := ByID(userID)
	if !ok {
		return m.real.UserGuildMember(ctx, userID, guildID)
	}
	if !mock.memberOf(guildID) {
		return nil, ports.ErrNotFound
	}
	return &access.GuildMember{Username: mock.Username, GlobalName: mock.Label, RoleIDs: mock.RoleIDs}, nil
}
