package ports

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/stats"
)

type StatsRepository interface {
	// SpotTotals groups the filtered reservations by respawn.
	SpotTotals(ctx context.Context, f stats.Filter) ([]stats.SpotRow, error)
	// PlayerTotals groups the filtered reservations by author_discord_id; free-text authors are left out.
	PlayerTotals(ctx context.Context, f stats.Filter) ([]stats.PlayerRow, error)
	// CharacterTotals groups by each character of the author text.
	CharacterTotals(ctx context.Context, f stats.Filter) ([]stats.CharacterRow, error)
	// Daily groups by the local day of start_at, oldest first. Days without reservations are absent.
	Daily(ctx context.Context, f stats.Filter) ([]stats.Day, error)
	// ReservationDays returns the local days in [from, to) with at least one reservation, oldest first.
	ReservationDays(ctx context.Context, guildID string, from, to time.Time) ([]time.Time, error)
	// LatestPlayerName returns the author text of the user's latest reservation, or ErrNotFound.
	LatestPlayerName(ctx context.Context, guildID, userID string) (string, error)
	// CharacterReservations returns the newest filtered reservations of f.CharacterKey, at most limit.
	CharacterReservations(ctx context.Context, f stats.Filter, limit int) ([]stats.CharacterReservation, error)
}

type StatsService interface {
	Overview(ctx context.Context, guildID string, rng stats.Range) (*stats.Overview, error)
	Spots(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort) ([]stats.SpotRow, error)
	Players(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort) ([]stats.PlayerRow, error)
	Characters(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort) ([]stats.CharacterRow, error)
	// Spot returns ErrNotFound for a respawn of another guild.
	Spot(ctx context.Context, guildID string, spotID int64, rng stats.Range) (*stats.SpotDetail, error)
	// Player returns ErrNotFound for a user without reservations in the guild.
	Player(ctx context.Context, guildID, userID string, rng stats.Range) (*stats.PlayerDetail, error)
	// DataDays returns the days with reservations for the range picker.
	DataDays(ctx context.Context, guildID string, now time.Time) ([]time.Time, error)
}

type CharacterProfileService interface {
	// Profile never fails because of TibiaData; it reports that in the profile Source.
	Profile(ctx context.Context, guildID, name string, rng stats.Range) (*stats.CharacterProfile, error)
}
