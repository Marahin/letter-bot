package sqlc

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/db/postgresql"
)

// StatsRepository implements ports.StatsRepository.
type StatsRepository struct {
	q   *Queries
	tz  string
	loc *time.Location
}

// NewStatsRepository buckets days in loc; its name must be an IANA zone PostgreSQL knows (D31: the
// process zone). A location named "Local" (no TZ set) falls back to Europe/Berlin.
func NewStatsRepository(db DBTX, loc *time.Location) *StatsRepository {
	tz := loc.String()
	if tz == "Local" || tz == "" {
		tz = "Europe/Berlin"
	}
	return &StatsRepository{q: New(db), tz: tz, loc: loc}
}

func (r *StatsRepository) SpotTotals(ctx context.Context, f stats.Filter) ([]stats.SpotRow, error) {
	rows, err := r.q.SpotTotals(ctx, SpotTotalsParams{
		GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.SpotRow, len(rows))
	for i, row := range rows {
		out[i] = stats.SpotRow{
			SpotID:   row.SpotID,
			Name:     row.Name,
			Archived: row.Archived,
			Totals:   totals(row.Reservations, row.Seconds, row.ExpReservations, row.ExpSeconds, row.Exp),
		}
	}
	return out, nil
}

func (r *StatsRepository) PlayerTotals(ctx context.Context, f stats.Filter) ([]stats.PlayerRow, error) {
	rows, err := r.q.PlayerTotals(ctx, PlayerTotalsParams{
		GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.PlayerRow, len(rows))
	for i, row := range rows {
		out[i] = stats.PlayerRow{
			UserID: row.UserID,
			Name:   row.Name,
			Totals: totals(row.Reservations, row.Seconds, row.ExpReservations, row.ExpSeconds, row.Exp),
		}
	}
	return out, nil
}

func (r *StatsRepository) CharacterTotals(ctx context.Context, f stats.Filter) ([]stats.CharacterRow, error) {
	rows, err := r.q.CharacterTotals(ctx, CharacterTotalsParams{
		GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.CharacterRow, len(rows))
	for i, row := range rows {
		out[i] = stats.CharacterRow{
			Key:    row.CharacterKey,
			Name:   row.Name,
			Totals: totals(row.Reservations, row.Seconds, row.ExpReservations, row.ExpSeconds, row.Exp),
		}
	}
	return out, nil
}

func (r *StatsRepository) Daily(ctx context.Context, f stats.Filter) ([]stats.Day, error) {
	rows, err := r.q.Daily(ctx, DailyParams{
		Tz: r.tz, GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.Day, len(rows))
	for i, row := range rows {
		out[i] = stats.Day{
			Day:    r.day(row.Day),
			Totals: totals(row.Reservations, row.Seconds, row.ExpReservations, row.ExpSeconds, row.Exp),
		}
	}
	return out, nil
}

func (r *StatsRepository) ReservationDays(ctx context.Context, guildID string, from, to time.Time) ([]time.Time, error) {
	rows, err := r.q.ReservationDays(ctx, ReservationDaysParams{Tz: r.tz, GuildID: guildID, FromT: ts(from), ToT: ts(to)})
	if err != nil {
		return nil, err
	}
	out := make([]time.Time, len(rows))
	for i, d := range rows {
		out[i] = r.day(d)
	}
	return out, nil
}

func (r *StatsRepository) LatestPlayerName(ctx context.Context, guildID, userID string) (string, error) {
	name, err := r.q.LatestPlayerName(ctx, LatestPlayerNameParams{GuildID: guildID, UserID: userID})
	return name, postgresql.MapError(err)
}

func (r *StatsRepository) CharacterReservations(ctx context.Context, f stats.Filter, limit int) ([]stats.CharacterReservation, error) {
	rows, err := r.q.CharacterReservations(ctx, CharacterReservationsParams{
		CharacterKey: f.CharacterKey, GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), Lim: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]stats.CharacterReservation, len(rows))
	for i, row := range rows {
		cr := stats.CharacterReservation{
			ID:       row.ID,
			SpotID:   row.SpotID,
			SpotName: row.SpotName,
			Author:   row.Author,
			StartAt:  row.StartAt.Time,
			EndAt:    row.EndAt.Time,
			Status:   experience.Status(row.Status.String),
		}
		if row.Gain.Valid {
			g := row.Gain.Int64
			cr.Gain = &g
		}
		out[i] = cr
	}
	return out, nil
}

func (r *StatsRepository) day(d pgtype.Date) time.Time {
	y, m, dd := d.Time.Date()
	return time.Date(y, m, dd, 0, 0, 0, 0, r.loc)
}

func totals(reservations, seconds, expReservations, expSeconds, exp int64) stats.Totals {
	return stats.Totals{Reservations: reservations, Seconds: seconds, ExpReservations: expReservations, ExpSeconds: expSeconds, Exp: exp}
}

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
