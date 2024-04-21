package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/discord"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/dto/summary"
)

type ReservationRepository interface {
	Find(ctx context.Context, id int64) (*reservation.Reservation, error)
	FindReservationWithSpot(ctx context.Context, id int64, guildID, authorDiscordID string) (*reservation.ReservationWithSpot, error)
	SelectUpcomingReservationsWithSpot(ctx context.Context, guildId string) ([]*reservation.ReservationWithSpot, error)
	SelectUpcomingReservationsWithSpotBySpots(ctx context.Context, guildId string, spots []string) ([]*reservation.ReservationWithSpot, error)
	SelectOverlappingReservations(ctx context.Context, spot string, startAt time.Time, endAt time.Time, guildId string) ([]*reservation.Reservation, error)
	SelectUpcomingMemberReservationsWithSpots(ctx context.Context, guild *discord.Guild, member *discord.Member) ([]*reservation.ReservationWithSpot, error)

	// Creates a new reservation, and removes or shorten any existing conflicting reservations.
	// Returns removed or shortened conflicting reservations.
	CreateAndDeleteConflicting(ctx context.Context, member *discord.Member, guild *discord.Guild, conflicts []*reservation.Reservation, spotId int64, startAt time.Time, endAt time.Time) ([]*reservation.ClippedOrRemovedReservation, error)

	// Deletes one of the upcoming member reservations in a given guild. Returns error if operation
	// did not succeed.
	DeletePresentMemberReservation(ctx context.Context, g *discord.Guild, m *discord.Member, reservationId int64) error
}

type SpotRepository interface {
	SelectAllSpots(ctx context.Context) ([]*spot.Spot, error)
}

type BotPort interface {
	ChannelMessages(g *discord.Guild, ch *discord.Channel, limit int) ([]*discord.Message, error)
	CleanChannel(g *discord.Guild, channel *discord.Channel) error
	EnsureChannel(g *discord.Guild) error
	FindChannelByName(g *discord.Guild, channelName string) (*discord.Channel, error)
	EnsureRoles(g *discord.Guild) error
	GetGuilds() []*discord.Guild
	SendLetterMessage(g *discord.Guild, ch *discord.Channel, sum *summary.Summary) error
	SendDM(m *discord.Member, message string) error
	RegisterCommands(g *discord.Guild) error
	MemberHasRole(g *discord.Guild, m *discord.Member, roleName string) bool
	OpenDM(m *discord.Member) (*discord.Channel, error)
	GetMember(guild *discord.Guild, memberID string) (*discord.Member, error)
	// Should start background worker loop, which should then emit Tick event periodically.
	StartTicking()
}

type ChartAdapter interface {
	NewChart(values []float64, legend []string) ([]byte, error)
}
