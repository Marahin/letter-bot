package sqlc

import (
	"context"
	"errors"
	"fmt"
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

// NewStatsRepository buckets days in the IANA zone, which PostgreSQL must know too. It must be the
// process zone (TZ, D31), because the handlers read days in time.Local.
func NewStatsRepository(db DBTX, zone string) (*StatsRepository, error) {
	if zone == "" {
		return nil, errors.New("stats: no time zone; set TZ, e.g. Europe/Berlin")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("stats: time zone %q: %w", zone, err)
	}
	return &StatsRepository{q: New(db), tz: zone, loc: loc}, nil
}

func (r *StatsRepository) SpotTotals(ctx context.Context, q stats.Query) (stats.Page[stats.SpotRow], error) {
	f := q.Filter
	rows, err := r.q.SpotTotals(ctx, SpotTotalsParams{
		SortKey: string(q.Sort.Key), Ascending: q.Sort.Asc, Lim: int32(q.Limit),
		GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
	})
	if err != nil {
		return stats.Page[stats.SpotRow]{}, err
	}
	page := stats.Page[stats.SpotRow]{Rows: make([]stats.SpotRow, len(rows))}
	for i, row := range rows {
		page.Total = int(row.TotalRows)
		page.Rows[i] = stats.SpotRow{
			SpotID:   row.SpotID,
			Name:     row.Name,
			Archived: row.Archived,
			Totals:   totals(row.Reservations, row.Seconds, row.ExpReservations, row.ExpSeconds, row.Exp),
		}
	}
	return page, nil
}

func (r *StatsRepository) PlayerTotals(ctx context.Context, q stats.Query) (stats.Page[stats.PlayerRow], error) {
	f := q.Filter
	rows, err := r.q.PlayerTotals(ctx, PlayerTotalsParams{
		SortKey: string(q.Sort.Key), Ascending: q.Sort.Asc, Lim: int32(q.Limit),
		GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
	})
	if err != nil {
		return stats.Page[stats.PlayerRow]{}, err
	}
	page := stats.Page[stats.PlayerRow]{Rows: make([]stats.PlayerRow, len(rows))}
	for i, row := range rows {
		page.Total = int(row.TotalRows)
		page.Rows[i] = stats.PlayerRow{
			UserID: row.UserID,
			Name:   row.Name,
			Totals: totals(row.Reservations, row.Seconds, row.ExpReservations, row.ExpSeconds, row.Exp),
		}
	}
	return page, nil
}

func (r *StatsRepository) CharacterTotals(ctx context.Context, q stats.Query) (stats.Page[stats.CharacterRow], error) {
	f := q.Filter
	rows, err := r.q.CharacterTotals(ctx, CharacterTotalsParams{
		SortKey: string(q.Sort.Key), Ascending: q.Sort.Asc, Lim: int32(q.Limit),
		GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
	})
	if err != nil {
		return stats.Page[stats.CharacterRow]{}, err
	}
	page := stats.Page[stats.CharacterRow]{Rows: make([]stats.CharacterRow, len(rows))}
	for i, row := range rows {
		page.Total = int(row.TotalRows)
		page.Rows[i] = stats.CharacterRow{
			Key:    row.CharacterKey,
			Name:   row.Name,
			Totals: totals(row.Reservations, row.Seconds, row.ExpReservations, row.ExpSeconds, row.Exp),
		}
	}
	return page, nil
}

func (r *StatsRepository) CharacterLeaderboards(ctx context.Context, f stats.Filter, n int, minExpSeconds int64) (stats.CharacterBoards, error) {
	rows, err := r.q.CharacterLeaderboards(ctx, CharacterLeaderboardsParams{
		GuildID: f.GuildID, FromT: ts(f.From), ToT: ts(f.To), SpotID: f.SpotID, UserID: f.UserID, CharacterKey: f.CharacterKey,
		Lim: int32(n), MinExpSeconds: minExpSeconds,
	})
	if err != nil {
		return stats.CharacterBoards{}, err
	}
	var boards stats.CharacterBoards
	for _, row := range rows {
		boards.Characters = int(row.TotalRows)
		c := stats.CharacterRow{
			Key:    row.CharacterKey.String,
			Name:   row.Name,
			Totals: totals(row.Reservations.Int64, row.Seconds.Int64, row.ExpReservations.Int64, row.ExpSeconds.Int64, row.Exp.Int64),
		}
		switch row.Board.String {
		case "exp":
			boards.ByExp = append(boards.ByExp, c)
		case "exp_h":
			boards.ByExpPerHour = append(boards.ByExpPerHour, c)
		}
	}
	return boards, nil
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
