package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/member"
	"spot-assistant/internal/core/dto/role"
	"spot-assistant/internal/core/dto/webuser"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/guildsworld"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/dto/summary"
)

type ReservationRepository interface {
	Find(ctx context.Context, id int64) (*reservation.Reservation, error)
	FindReservationWithSpot(ctx context.Context, id int64, guildID, authorDiscordID string) (*reservation.ReservationWithSpot, error)
	SelectUpcomingReservationsWithSpot(ctx context.Context, guildId string) ([]*reservation.ReservationWithSpot, error)
	SelectUpcomingReservationsWithSpotForSpot(ctx context.Context, guildId, spotName string) ([]*reservation.ReservationWithSpot, error)
	// SelectOverlappingReservations returns the upcoming reservations of the spot that overlap [startAt, endAt].
	SelectOverlappingReservations(ctx context.Context, spotID int64, startAt time.Time, endAt time.Time, guildId string) ([]*reservation.Reservation, error)
	SelectUpcomingMemberReservationsWithSpots(ctx context.Context, guild *guild.Guild, member *member.Member) ([]*reservation.ReservationWithSpot, error)

	// Creates a new reservation, and removes or shorten any existing conflicting reservations.
	// Returns removed or shortened conflicting reservations.
	CreateAndDeleteConflicting(ctx context.Context, member *member.Member, guild *guild.Guild, conflicts []*reservation.Reservation, spotId int64, startAt time.Time, endAt time.Time) ([]*reservation.ClippedOrRemovedReservation, error)

	// Deletes one of the upcoming member reservations in a given guild. Returns error if operation
	// did not succeed.
	DeletePresentMemberReservation(ctx context.Context, g *guild.Guild, m *member.Member, reservationId int64) error

	// SearchReservationsWithSpot returns one page of the guild reservations that match the filter.
	SearchReservationsWithSpot(ctx context.Context, filter reservation.SearchFilter) ([]*reservation.ReservationWithSpot, error)

	// CountReservations counts the reservations that match the filter. Limit and Offset are ignored.
	CountReservations(ctx context.Context, filter reservation.SearchFilter) (int64, error)

	// SelectGuildReservationWithSpot returns ports.ErrNotFound when the reservation is not in the guild.
	SelectGuildReservationWithSpot(ctx context.Context, guildID string, id int64) (*reservation.ReservationWithSpot, error)

	// UpdateReservation writes the spot, times and author of r (found by r.ID in the guild).
	// Returns ports.ErrNotFound or ports.ErrConflict (overlap with another reservation).
	UpdateReservation(ctx context.Context, guildID string, r reservation.Reservation) error

	// DeleteGuildReservation deletes any reservation of the guild. Returns ports.ErrNotFound.
	DeleteGuildReservation(ctx context.Context, guildID string, id int64) error

	// SelectOverlappingReservationsBySpotID returns past and upcoming reservations of the spot that
	// overlap [startAt, endAt]. excludeID (0 = none) is left out.
	SelectOverlappingReservationsBySpotID(ctx context.Context, guildID string, spotID int64, startAt time.Time, endAt time.Time, excludeID int64) ([]*reservation.Reservation, error)

	// SelectKnownAuthors returns up to 20 Discord users whose reservation author matches the pattern,
	// each with their latest author text.
	SelectKnownAuthors(ctx context.Context, guildID string, pattern string) ([]*reservation.KnownAuthor, error)
}

type SpotRepository interface {
	// SelectGuildSpots returns the guild spots ordered by name.
	SelectGuildSpots(ctx context.Context, guildID string, includeArchived bool) ([]*spot.Spot, error)

	// SelectGuildSpotByName returns the active spot with the name (case-insensitive), or ports.ErrNotFound.
	SelectGuildSpotByName(ctx context.Context, guildID string, name string) (*spot.Spot, error)

	// SelectGuildSpotsLike returns up to 15 active spots whose name contains the pattern.
	SelectGuildSpotsLike(ctx context.Context, guildID string, namePattern string) ([]*spot.Spot, error)

	// SelectGuildSpotByID returns an active or archived spot, or ports.ErrNotFound.
	SelectGuildSpotByID(ctx context.Context, guildID string, id int64) (*spot.Spot, error)

	// InsertSpot returns ports.ErrDuplicate when an active spot has the same name.
	InsertSpot(ctx context.Context, guildID string, name string) (*spot.Spot, error)

	// RenameSpot returns ports.ErrNotFound or ports.ErrDuplicate.
	RenameSpot(ctx context.Context, guildID string, id int64, name string) error

	// ArchiveSpot returns ports.ErrNotFound when no active spot has the id.
	ArchiveSpot(ctx context.Context, guildID string, id int64) error

	// RestoreSpot returns ports.ErrNotFound when no archived spot has the id,
	// or ports.ErrDuplicate when an active spot has the same name.
	RestoreSpot(ctx context.Context, guildID string, id int64) error

	// DeleteSpot returns ports.ErrNotFound when no spot has the id, or when any
	// reservation (of any guild) points at it.
	DeleteSpot(ctx context.Context, guildID string, id int64) error

	// CountSpotReservations counts all past and upcoming reservations of the spot.
	CountSpotReservations(ctx context.Context, guildID string, id int64) (int64, error)

	// SelectGuildSpotReservationCounts maps each guild spot with reservations to its counts. It
	// counts every reservation of the spot, as DeleteSpot does, even one with another guild_id.
	SelectGuildSpotReservationCounts(ctx context.Context, guildID string) (map[int64]spot.ReservationCounts, error)

	// InsertSpotsIgnoreDuplicates adds the names that are not active in the guild yet.
	// Returns the number of spots added.
	InsertSpotsIgnoreDuplicates(ctx context.Context, guildID string, names []string) (int64, error)
}

type BotPort interface {
	// Run Starts the bot instance, blocks until the bot is stopped.
	Run() error

	// FindChannelByName finds a channel by name in a given guild.
	FindChannelByName(g *guild.Guild, channelName string) (*discord.Channel, error)

	// FindChannelById finds a channel by id in a given guild.
	FindChannelById(g *guild.Guild, channelId string) (*discord.Channel, error)

	// SendLetterMessage sends a message to a guild channel
	// or a DM if guild is empty.
	SendLetterMessage(g *guild.Guild, ch *discord.Channel, sum *summary.Summary) error

	// SendDMOverbookedNotification sends a DM to a member about overbooking.
	SendDMOverbookedNotification(member *member.Member, request book.BookRequest, res *reservation.ClippedOrRemovedReservation) error

	// OpenDM opens a DM channel with a member.
	OpenDM(m *member.Member) (*discord.Channel, error)
}

// GuildConfigRepository stores the per-guild configuration (the guilds table).
// Methods that write one guild return ports.ErrNotFound when the guild is not stored.
type GuildConfigRepository interface {
	Get(ctx context.Context, guildID string) (*guildconfig.Config, error)
	ListByIDs(ctx context.Context, guildIDs []string) ([]*guildconfig.Config, error)
	ListAll(ctx context.Context) ([]*guildconfig.Config, error)

	// UpsertPresence stores the Discord data of a guild and sets bot_present.
	// It never changes the premium, channel or rank settings.
	UpsertPresence(ctx context.Context, guildID, name, icon, ownerID string) (*guildconfig.Config, error)
	SetBotPresent(ctx context.Context, guildID string, present bool) error
	SetPremium(ctx context.Context, guildID string, premium bool) error
	SetChannels(ctx context.Context, guildID, commandChannelID, summaryChannelID string) error
	SetRoleIDs(ctx context.Context, guildID string, kind guildconfig.RoleKind, roleIDs []string) error

	RequestResync(ctx context.Context, guildID string) error
	// ListResyncRequested returns the ids of the guilds (with the bot present) that wait for a resync.
	ListResyncRequested(ctx context.Context) ([]string, error)
	// MarkSynced sets synced_at and clears the resync request unless it was made after startedAt.
	MarkSynced(ctx context.Context, guildID string, startedAt time.Time) error
}

// GuildChannelRepository stores the channels the bot synced from Discord.
type GuildChannelRepository interface {
	// Replace swaps all stored channels of the guild in one transaction.
	Replace(ctx context.Context, guildID string, channels []*discord.Channel) error
	List(ctx context.Context, guildID string) ([]*discord.Channel, error)
}

// GuildRoleRepository stores the roles the bot synced from Discord.
type GuildRoleRepository interface {
	// Replace swaps all stored roles of the guild in one transaction.
	Replace(ctx context.Context, guildID string, roles []*role.Role) error
	// List returns the roles, highest position first.
	List(ctx context.Context, guildID string) ([]*role.Role, error)
}

// WebUserRepository stores the users that signed in to the web panel.
type WebUserRepository interface {
	// Upsert keeps the stored token.
	Upsert(ctx context.Context, user webuser.User) (*webuser.User, error)
	// Get returns ports.ErrNotFound for an unknown user.
	Get(ctx context.Context, discordUserID string) (*webuser.User, error)
	// SaveToken returns ports.ErrNotFound for an unknown user.
	SaveToken(ctx context.Context, discordUserID string, token webuser.Token) error
	// AccessToken returns ports.ErrNotFound for an unknown user.
	AccessToken(ctx context.Context, discordUserID string) (*webuser.Token, error)
}

type GuildRepository interface {
	// GetGuilds returns all guilds.
	GetGuilds() []*guild.Guild
}

type MemberRepository interface {
	// GetMemberByGuildAndId returns member by guild and id.
	GetMemberByGuildAndId(g *guild.Guild, memberId string) (*member.Member, error)
	// MemberHasRole checks if a member has a role.
	MemberHasRole(g *guild.Guild, m *member.Member, roleName string) bool
}

type WorldApi interface {
	GetOnlinePlayerNames(worldName string) ([]string, error)
	GetBaseURL() string
}

type ChartAdapter interface {
	NewChart(values []float64, legend []string) ([]byte, error)
}

type TextFormatter interface {
	FormatGenericError(err error) string
	FormatBookResponse(response book.BookResponse) string
	FormatBookError(response book.BookResponse, err error) string
	FormatOverbookedMemberNotification(member *member.Member,
		request book.BookRequest,
		res *reservation.ClippedOrRemovedReservation) string
}

type WorldNameRepository interface {
	UpsertGuildWorld(ctx context.Context, guildID string, worldName string) error
	SelectGuildWorld(ctx context.Context, guildID string) (*guildsworld.GuildsWorld, error)
}
