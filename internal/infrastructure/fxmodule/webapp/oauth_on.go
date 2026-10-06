//go:build devauth

package webapp

import (
	"spot-assistant/internal/infrastructure/devauth"
	"spot-assistant/internal/infrastructure/discord/oauth"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/web"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
	"spot-assistant/internal/ports"
)

// newOAuthPort wraps the real OAuth adapter with the dev-only mock, so a user
// signed in through /dev/login resolves to the seeded servers. It does so only
// when /dev/login is on: otherwise /auth/callback would accept dev codes.
func newOAuthPort(cfg web.Config, c *oauth.Caching, configs *guildsqlc.GuildConfigRepository, users *webusersqlc.WebUserRepository) ports.OAuthPort {
	if !cfg.DevAuth || !devauth.IsDevBaseURL(cfg.BaseURL) {
		return c
	}
	return devauth.NewMockOAuth(c, configs, users)
}
