package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/dto/spot"
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

// GuildSettingsService backs the admin Settings and Channels pages. It lists the
// synced Discord options, validates each choice against them, stores it and signals the bot.
type GuildSettingsService interface {
	// Channels returns the synced text and announcement channels.
	Channels(ctx context.Context, guildID string) ([]*discord.Channel, error)
	Roles(ctx context.Context, guildID string) ([]*role.Role, error)
	// World returns "" when the guild has no world yet.
	World(ctx context.Context, guildID string) (string, error)
	// SetChannels stores the command and summary channels. An empty id selects the legacy default channel.
	SetChannels(ctx context.Context, guildID, commandChannelID, summaryChannelID string) error
	SetRoleIDs(ctx context.Context, guildID string, kind guildconfig.RoleKind, roleIDs []string) error
	SetWorld(ctx context.Context, guildID, world string) error
	// RequestResync asks the bot to sync channels and roles again. During the
	// cooldown it does nothing and returns the time left.
	RequestResync(ctx context.Context, guildID string) (retryAfter time.Duration, err error)
}

// SpotService manages the respawn list of a guild. Every id is checked against the guild.
type SpotService interface {
	List(ctx context.Context, guildID string, filter spot.ListFilter) (*spot.List, error)
	Create(ctx context.Context, guildID, name string) (*spot.Spot, error)
	Rename(ctx context.Context, guildID string, id int64, name string) error
	// Remove deletes a spot without reservations and archives any other, so history and stats stay.
	Remove(ctx context.Context, guildID string, id int64) (spot.RemoveOutcome, error)
	Restore(ctx context.Context, guildID string, id int64) error
	// ImportDefaults adds the default respawn names the guild does not have yet and returns how many it added.
	ImportDefaults(ctx context.Context, guildID string) (int64, error)
}
