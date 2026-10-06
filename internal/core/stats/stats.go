// Package stats builds the reservation and experience statistics of a guild.
package stats

import (
	"context"
	"time"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/ports"
)

const (
	DefaultDays = 30
	MaxDays     = 400
	// LeaderboardSize is the number of rows in each overview leaderboard.
	LeaderboardSize = 10
	// BreakdownSize is the number of rows in each table of a detail page.
	BreakdownSize = 25
	// MinExpPerHourHours keeps a single short lucky hunt off the experience-per-hour leaderboard.
	MinExpPerHourHours = 3.0
	// DataDaysWindow is how far back the range picker marks days with reservations.
	DataDaysWindow = 2 * 365
)

type Service struct {
	repo  ports.StatsRepository
	spots ports.SpotRepository
}

func New(repo ports.StatsRepository, spots ports.SpotRepository) *Service {
	return &Service{repo: repo, spots: spots}
}

// Midnight is the start of t's day in t's location.
func Midnight(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// ResolveRange turns the picked days into a contiguous span: from the first to the last picked day,
// at most MaxDays (the newest ones). No pick means the last DefaultDays days through today.
func ResolveRange(selected []time.Time, now time.Time) stats.Range {
	var first, last time.Time
	if len(selected) == 0 {
		last = Midnight(now)
		first = last.AddDate(0, 0, -(DefaultDays - 1))
	} else {
		first, last = Midnight(selected[0]), Midnight(selected[0])
		for _, d := range selected[1:] {
			d = Midnight(d)
			if d.Before(first) {
				first = d
			}
			if d.After(last) {
				last = d
			}
		}
	}
	if earliest := last.AddDate(0, 0, -(MaxDays - 1)); first.Before(earliest) {
		first = earliest
	}
	var days []time.Time
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		days = append(days, d)
	}
	return stats.Range{From: first, To: last.AddDate(0, 0, 1), Days: days}
}

// FillDaily returns one row per day of days, with zero totals for the days without reservations.
func FillDaily(rows []stats.Day, days []time.Time) []stats.Day {
	byDay := make(map[string]stats.Totals, len(rows))
	for _, r := range rows {
		byDay[r.Day.Format(time.DateOnly)] = r.Totals
	}
	out := make([]stats.Day, len(days))
	for i, d := range days {
		out[i] = stats.Day{Day: d, Totals: byDay[d.Format(time.DateOnly)]}
	}
	return out
}

// Sum adds the totals of every day.
func Sum(days []stats.Day) stats.Totals {
	var t stats.Totals
	for _, d := range days {
		t = t.Add(d.Totals)
	}
	return t
}

func filter(guildID string, rng stats.Range) stats.Filter {
	return stats.Filter{GuildID: guildID, From: rng.From, To: rng.To}
}

func byHours(f stats.Filter, limit int) stats.Query {
	return stats.Query{Filter: f, Sort: stats.Sort{Key: stats.SortHours}, Limit: limit}
}

func (s *Service) Overview(ctx context.Context, guildID string, rng stats.Range) (*stats.Overview, error) {
	f := filter(guildID, rng)
	daily, err := s.repo.Daily(ctx, f)
	if err != nil {
		return nil, err
	}
	spots, err := s.repo.SpotTotals(ctx, byHours(f, LeaderboardSize))
	if err != nil {
		return nil, err
	}
	players, err := s.repo.PlayerTotals(ctx, byHours(f, LeaderboardSize))
	if err != nil {
		return nil, err
	}
	characters, err := s.repo.CharacterLeaderboards(ctx, f, LeaderboardSize, int64(MinExpPerHourHours*3600))
	if err != nil {
		return nil, err
	}
	return &stats.Overview{
		Range:      rng,
		Totals:     Sum(daily),
		Spots:      spots.Total,
		Players:    players.Total,
		Characters: characters.Characters,
		Daily:      FillDaily(daily, rng.Days),
		TopSpots:   spots.Rows,
		Leaderboards: stats.Leaderboards{
			PlayersByHours:         players.Rows,
			CharactersByExp:        characters.ByExp,
			CharactersByExpPerHour: characters.ByExpPerHour,
		},
	}, nil
}

func (s *Service) Spots(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort, limit int) (stats.Page[stats.SpotRow], error) {
	return s.repo.SpotTotals(ctx, stats.Query{Filter: filter(guildID, rng), Sort: sort, Limit: limit})
}

func (s *Service) Players(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort, limit int) (stats.Page[stats.PlayerRow], error) {
	return s.repo.PlayerTotals(ctx, stats.Query{Filter: filter(guildID, rng), Sort: sort, Limit: limit})
}

func (s *Service) Characters(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort, limit int) (stats.Page[stats.CharacterRow], error) {
	return s.repo.CharacterTotals(ctx, stats.Query{Filter: filter(guildID, rng), Sort: sort, Limit: limit})
}

func (s *Service) Spot(ctx context.Context, guildID string, spotID int64, rng stats.Range) (*stats.SpotDetail, error) {
	sp, err := s.spots.SelectGuildSpotByID(ctx, guildID, spotID)
	if err != nil {
		return nil, err
	}
	f := filter(guildID, rng)
	f.SpotID = spotID
	daily, err := s.repo.Daily(ctx, f)
	if err != nil {
		return nil, err
	}
	players, err := s.repo.PlayerTotals(ctx, byHours(f, BreakdownSize))
	if err != nil {
		return nil, err
	}
	characters, err := s.repo.CharacterTotals(ctx, byHours(f, BreakdownSize))
	if err != nil {
		return nil, err
	}
	return &stats.SpotDetail{
		Spot:       sp,
		Range:      rng,
		Totals:     Sum(daily),
		Daily:      FillDaily(daily, rng.Days),
		Players:    players,
		Characters: characters,
	}, nil
}

func (s *Service) Player(ctx context.Context, guildID, userID string, rng stats.Range) (*stats.PlayerDetail, error) {
	if userID == "" {
		return nil, ports.ErrNotFound
	}
	name, err := s.repo.LatestPlayerName(ctx, guildID, userID)
	if err != nil {
		return nil, err
	}
	f := filter(guildID, rng)
	f.UserID = userID
	daily, err := s.repo.Daily(ctx, f)
	if err != nil {
		return nil, err
	}
	spots, err := s.repo.SpotTotals(ctx, byHours(f, BreakdownSize))
	if err != nil {
		return nil, err
	}
	characters, err := s.repo.CharacterTotals(ctx, byHours(f, BreakdownSize))
	if err != nil {
		return nil, err
	}
	return &stats.PlayerDetail{
		UserID:     userID,
		Name:       name,
		Range:      rng,
		Totals:     Sum(daily),
		Daily:      FillDaily(daily, rng.Days),
		Spots:      spots,
		Characters: characters,
	}, nil
}

func (s *Service) DataDays(ctx context.Context, guildID string, now time.Time) ([]time.Time, error) {
	to := Midnight(now).AddDate(0, 0, 1)
	return s.repo.ReservationDays(ctx, guildID, to.AddDate(0, 0, -DataDaysWindow), to)
}
