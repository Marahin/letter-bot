package ports

import (
	"context"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/webuser"
)

// OAuthPort is Discord's OAuth2 API, used with the signed-in user's own token. The
// adapter stores and refreshes the tokens, so callers deal only in user ids.
// Errors wrap ErrUpstreamUnavailable (retry later) or ErrUnauthorized (sign in again).
type OAuthPort interface {
	AuthCodeURL(state string) string
	// Exchange swaps an auth code for tokens, stores the user and the tokens, and returns the user.
	Exchange(ctx context.Context, code string) (*webuser.User, error)
	UserGuilds(ctx context.Context, userID string) ([]access.UserGuild, error)
	// UserGuildMember returns ErrNotFound when the user is not a member of the guild.
	UserGuildMember(ctx context.Context, userID, guildID string) (*access.GuildMember, error)
}

// AuthService is the Discord sign-in flow of the web panel.
type AuthService interface {
	LoginURL(state string) string
	Complete(ctx context.Context, code string) (*webuser.User, error)
	// User returns ErrNotFound for an unknown user.
	User(ctx context.Context, userID string) (*webuser.User, error)
}

// GuildAccessService decides which stored guilds a web user may open, and with which capabilities.
type GuildAccessService interface {
	// AccessibleGuilds returns the bot-present guilds the user may view.
	AccessibleGuilds(ctx context.Context, userID string) ([]access.GuildAccess, error)
	// Access returns ErrNotFound when the user may not view the guild, so a caller
	// cannot tell a foreign guild from a missing one.
	Access(ctx context.Context, userID, guildID string) (*access.GuildAccess, error)
	// Member returns the user's member record in the guild (nick and roles).
	Member(ctx context.Context, userID, guildID string) (*access.GuildMember, error)
	IsSiteAdmin(userID string) bool
}

// PremiumService is the site-admin premium switch.
type PremiumService interface {
	List(ctx context.Context) ([]*guildconfig.Config, error)
	SetPremium(ctx context.Context, guildID string, premium bool) error
}

// BotNotifier sends best-effort signals from the web to the bot.
type BotNotifier interface {
	SummaryChanged(ctx context.Context, guildID string) error
	ResyncRequested(ctx context.Context, guildID string) error
	ConfigChanged(ctx context.Context, guildID string) error
	Overbooked(ctx context.Context, payload []byte) error
}
