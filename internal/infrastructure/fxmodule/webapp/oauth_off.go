//go:build !devauth

package webapp

import (
	"spot-assistant/internal/infrastructure/discord/oauth"
	guildsqlc "spot-assistant/internal/infrastructure/guild/postgresql/sqlc"
	"spot-assistant/internal/infrastructure/web"
	webusersqlc "spot-assistant/internal/infrastructure/webuser/postgresql/sqlc"
	"spot-assistant/internal/ports"
)

// newOAuthPort is the real OAuth adapter in a production build.
func newOAuthPort(_ web.Config, c *oauth.Caching, _ *guildsqlc.GuildConfigRepository, _ *webusersqlc.WebUserRepository) ports.OAuthPort {
	return c
}
