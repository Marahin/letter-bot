// Package stats builds the reservation and experience statistics of a guild.
package stats

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/ports"
)

const (
	DefaultDays = 30
	MaxDays     = 400
	// LeaderboardSize is the number of rows in each overview leaderboard.
	LeaderboardSize = 10
	// MinExpPerHourHours keeps a single short lucky hunt off the experience-per-hour leaderboard.
	MinExpPerHourHours = 3.0
	// DataDaysWindow is how far back the range picker marks days with reservations.
	DataDaysWindow = 2 * 365
)

var ErrNotFound = ports.ErrNotFound

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

// ParseSort reads a sort key and a direction ("asc"/"desc"). An unknown key sorts by hours; no
// direction means ascending for the name and descending for figures.
func ParseSort(key, dir string) stats.Sort {
	k := stats.SortKey(key)
	if !slices.Contains(stats.SortKeys, k) {
		k = stats.SortHours
	}
	asc := k == stats.SortName
	switch dir {
	case "asc":
		asc = true
	case "desc":
		asc = false
	}
	return stats.Sort{Key: k, Asc: asc}
}

// metric returns the row figure for key; ok is false when the row has no experience data for it.
func metric(t stats.Totals, key stats.SortKey) (float64, bool) {
	switch key {
	case stats.SortReservations:
		return float64(t.Reservations), true
	case stats.SortExp:
		if v := t.ExpTotal(); v != nil {
			return float64(*v), true
		}
		return 0, false
	case stats.SortExpPerHour:
		if v := t.ExpPerHour(); v != nil {
			return *v, true
		}
		return 0, false
	default:
		return float64(t.Seconds), true
	}
}

// SortRows sorts in place. Rows without the figure come last in both directions; ties go by name.
func SortRows[T stats.StatsRow](rows []T, s stats.Sort) {
	byName := func(a, b T) int {
		return cmp.Compare(strings.ToLower(a.Label()), strings.ToLower(b.Label()))
	}
	slices.SortStableFunc(rows, func(a, b T) int {
		if s.Key == stats.SortName {
			if s.Asc {
				return byName(a, b)
			}
			return byName(b, a)
		}
		va, oka := metric(a.Stats(), s.Key)
		vb, okb := metric(b.Stats(), s.Key)
		switch {
		case oka != okb:
			if oka {
				return -1
			}
			return 1
		case va != vb:
			if s.Asc {
				return cmp.Compare(va, vb)
			}
			return cmp.Compare(vb, va)
		}
		return byName(a, b)
	})
}

// Top returns the n best rows by key, leaving out rows without the figure. For experience per
// hour a row also needs minHours of reservations with data.
func Top[T stats.StatsRow](rows []T, key stats.SortKey, n int, minHours float64) []T {
	out := make([]T, 0, len(rows))
	for _, r := range rows {
		t := r.Stats()
		if _, ok := metric(t, key); !ok {
			continue
		}
		if key == stats.SortExpPerHour && t.ExpHours() < minHours {
			continue
		}
		out = append(out, r)
	}
	SortRows(out, stats.Sort{Key: key})
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// Sum adds the totals of every row.
func Sum[T stats.StatsRow](rows []T) stats.Totals {
	var t stats.Totals
	for _, r := range rows {
		t = t.Add(r.Stats())
	}
	return t
}

func filter(guildID string, rng stats.Range) stats.Filter {
	return stats.Filter{GuildID: guildID, From: rng.From, To: rng.To}
}

func (s *Service) Overview(ctx context.Context, guildID string, rng stats.Range) (*stats.Overview, error) {
	f := filter(guildID, rng)
	spots, err := s.repo.SpotTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	players, err := s.repo.PlayerTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	characters, err := s.repo.CharacterTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.Daily(ctx, f)
	if err != nil {
		return nil, err
	}
	return &stats.Overview{
		Range:      rng,
		Totals:     Sum(spots),
		Spots:      len(spots),
		Players:    len(players),
		Characters: len(characters),
		Daily:      FillDaily(daily, rng.Days),
		TopSpots:   Top(spots, stats.SortHours, LeaderboardSize, 0),
		Leaderboards: stats.Leaderboards{
			PlayersByHours:         Top(players, stats.SortHours, LeaderboardSize, 0),
			CharactersByExp:        Top(characters, stats.SortExp, LeaderboardSize, 0),
			CharactersByExpPerHour: Top(characters, stats.SortExpPerHour, LeaderboardSize, MinExpPerHourHours),
		},
	}, nil
}

func (s *Service) Spots(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort) ([]stats.SpotRow, error) {
	rows, err := s.repo.SpotTotals(ctx, filter(guildID, rng))
	if err != nil {
		return nil, err
	}
	SortRows(rows, sort)
	return rows, nil
}

func (s *Service) Players(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort) ([]stats.PlayerRow, error) {
	rows, err := s.repo.PlayerTotals(ctx, filter(guildID, rng))
	if err != nil {
		return nil, err
	}
	SortRows(rows, sort)
	return rows, nil
}

func (s *Service) Characters(ctx context.Context, guildID string, rng stats.Range, sort stats.Sort) ([]stats.CharacterRow, error) {
	rows, err := s.repo.CharacterTotals(ctx, filter(guildID, rng))
	if err != nil {
		return nil, err
	}
	SortRows(rows, sort)
	return rows, nil
}

func (s *Service) Spot(ctx context.Context, guildID string, spotID int64, rng stats.Range) (*stats.SpotDetail, error) {
	sp, err := s.spots.SelectGuildSpotByID(ctx, guildID, spotID)
	if err != nil {
		return nil, err
	}
	f := filter(guildID, rng)
	f.SpotID = spotID
	spots, err := s.repo.SpotTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.Daily(ctx, f)
	if err != nil {
		return nil, err
	}
	players, err := s.repo.PlayerTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	characters, err := s.repo.CharacterTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	SortRows(players, stats.Sort{Key: stats.SortHours})
	SortRows(characters, stats.Sort{Key: stats.SortHours})
	return &stats.SpotDetail{
		Spot:       sp,
		Range:      rng,
		Totals:     Sum(spots),
		Daily:      FillDaily(daily, rng.Days),
		Players:    players,
		Characters: characters,
	}, nil
}

func (s *Service) Player(ctx context.Context, guildID, userID string, rng stats.Range) (*stats.PlayerDetail, error) {
	if userID == "" {
		return nil, ErrNotFound
	}
	name, err := s.repo.LatestPlayerName(ctx, guildID, userID)
	if err != nil {
		return nil, err
	}
	f := filter(guildID, rng)
	f.UserID = userID
	spots, err := s.repo.SpotTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	daily, err := s.repo.Daily(ctx, f)
	if err != nil {
		return nil, err
	}
	characters, err := s.repo.CharacterTotals(ctx, f)
	if err != nil {
		return nil, err
	}
	SortRows(spots, stats.Sort{Key: stats.SortHours})
	SortRows(characters, stats.Sort{Key: stats.SortHours})
	return &stats.PlayerDetail{
		UserID:     userID,
		Name:       name,
		Range:      rng,
		Totals:     Sum(spots),
		Daily:      FillDaily(daily, rng.Days),
		Spots:      spots,
		Characters: characters,
	}, nil
}

func (s *Service) DataDays(ctx context.Context, guildID string, now time.Time) ([]time.Time, error) {
	to := Midnight(now).AddDate(0, 0, 1)
	return s.repo.ReservationDays(ctx, guildID, to.AddDate(0, 0, -DataDaysWindow), to)
}
