package adminhttp

import (
	"context"

	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/infrastructure/i18n"
)

// displayName names a server the bot never saw the name of.
func displayName(ctx context.Context, g *guildconfig.Config) string {
	if g.Name != "" {
		return g.Name
	}
	return i18n.T(ctx, "admin.guilds.unnamed")
}
