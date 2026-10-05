//go:build !devauth

package webapp

import (
	"spot-assistant/internal/infrastructure/discord/oauth"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
	"spot-assistant/internal/ports"
)

// newOAuthPort is the real OAuth adapter in a production build.
func newOAuthPort(c *oauth.Caching, _ *guildsqlc.GuildConfigRepository, _ *webusersqlc.WebUserRepository) ports.OAuthPort {
	return c
}
