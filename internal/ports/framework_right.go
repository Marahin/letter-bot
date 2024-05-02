package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/guild"
	"spot-assistant/internal/core/dto/member"

	"spot-assistant/internal/core/dto/book"
	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/dto/summary"
)

type ReservationRepository interface {
	Find(ctx context.Context, id int64) (*reservation.Reservation, error)
	FindReservationWithSpot(ctx context.Context, id int64, guildID, authorDiscordID string) (*reservation.ReservationWithSpot, error)
	SelectUpcomingReservationsWithSpot(ctx context.Context, guildId string) ([]*reservation.ReservationWithSpot, error)
	SelectOverlappingReservations(ctx context.Context, spot string, startAt time.Time, endAt time.Time, guildId string) ([]*reservation.Reservation, error)
	SelectUpcomingMemberReservationsWithSpots(ctx context.Context, guild *guild.Guild, member *member.Member) ([]*reservation.ReservationWithSpot, error)

	// Creates a new reservation, and removes or shorten any existing conflicting reservations.
	// Returns removed or shortened conflicting reservations.
	CreateAndDeleteConflicting(ctx context.Context, member *member.Member, guild *guild.Guild, conflicts []*reservation.Reservation, spotId int64, startAt time.Time, endAt time.Time) ([]*reservation.ClippedOrRemovedReservation, error)

	// Deletes one of the upcoming member reservations in a given guild. Returns error if operation
	// did not succeed.
	DeletePresentMemberReservation(ctx context.Context, g *guild.Guild, m *member.Member, reservationId int64) error
}

type SpotRepository interface {
	// SelectAllSpots returns all spots.
	SelectAllSpots(ctx context.Context) ([]*spot.Spot, error)
}

type BotPort interface {
	// Run Starts the bot instance, blocks until the bot is stopped.
	Run() error

	// FindChannelByName finds a channel by name in a given guild.
	FindChannelByName(g *guild.Guild, channelName string) (*discord.Channel, error)

	// SendLetterMessage sends a message to a guild channel
	// or a DM if guild is empty.
	SendLetterMessage(g *guild.Guild, ch *discord.Channel, sum *summary.Summary) error

	// SendDMOverbookedNotification sends a DM to a member about overbooking.
	SendDMOverbookedNotification(member *member.Member, request book.BookRequest, res *reservation.ClippedOrRemovedReservation) error

	// OpenDM opens a DM channel with a member.
	OpenDM(m *member.Member) (*discord.Channel, error)
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
