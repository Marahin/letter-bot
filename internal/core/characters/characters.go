// Package characters builds the character page: the TibiaData profile, the experience history and
// the reservation statistics of one character.
package characters

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/stats"
	corestats "spot-assistant/internal/core/stats"
	"spot-assistant/internal/ports"
)

// RecentLimit is the number of reservations the character page lists.
const RecentLimit = 20

type Service struct {
	characters ports.CharacterAPI
	worlds     ports.WorldNameRepository
	experience ports.ExperienceRepository
	stats      ports.StatsRepository
	log        *zap.SugaredLogger
}

func New(characters ports.CharacterAPI, worlds ports.WorldNameRepository, exp ports.ExperienceRepository, st ports.StatsRepository, log *zap.SugaredLogger) *Service {
	if log == nil {
		log = zap.NewNop().Sugar()
	}
	return &Service{characters: characters, worlds: worlds, experience: exp, stats: st, log: log}
}

// Profile returns ErrNotFound when the guild does not know the character, before asking TibiaData:
// the page is public, and must not relay lookups of any name.
func (s *Service) Profile(ctx context.Context, guildID, name string, rng stats.Range) (*stats.CharacterProfile, error) {
	name = strings.TrimSpace(name)
	key := experience.CharacterKey(name)
	if key == "" {
		return nil, ports.ErrNotFound
	}
	p := &stats.CharacterProfile{Key: key, Name: name, Range: rng}

	gw, err := s.worlds.SelectGuildWorld(ctx, guildID)
	switch {
	case errors.Is(err, ports.ErrNotFound):
	case err != nil:
		return nil, err
	default:
		p.World = gw.WorldName
		history, err := s.experience.SnapshotHistory(ctx, p.World, key, rng.From, rng.To)
		if err != nil {
			return nil, err
		}
		p.History = DailyHistory(history)
	}

	f := stats.Filter{GuildID: guildID, From: rng.From, To: rng.To, CharacterKey: key}
	daily, err := s.stats.Daily(ctx, f)
	if err != nil {
		return nil, err
	}
	if p.Spots, err = s.stats.SpotTotals(ctx, stats.Query{Filter: f, Sort: stats.Sort{Key: stats.SortHours}, Limit: corestats.BreakdownSize}); err != nil {
		return nil, err
	}
	if p.Recent, err = s.stats.CharacterReservations(ctx, f, RecentLimit); err != nil {
		return nil, err
	}
	p.Totals = corestats.Sum(daily)
	p.Daily = corestats.FillDaily(daily, rng.Days)

	known, err := s.knownInGuild(ctx, guildID, p)
	if err != nil {
		return nil, err
	}
	if !known {
		return nil, ports.ErrNotFound
	}
	s.loadCharacter(ctx, p)
	return p, nil
}

// knownInGuild also looks outside the range, so a member who picks an empty range still gets the page.
func (s *Service) knownInGuild(ctx context.Context, guildID string, p *stats.CharacterProfile) (bool, error) {
	if p.Totals.Reservations > 0 || len(p.History) > 0 {
		return true, nil
	}
	found, err := s.stats.CharacterReservations(ctx, allTime(guildID, p.Key), 1)
	if err != nil {
		return false, err
	}
	return len(found) > 0, nil
}

// allTime filters every reservation of the character in the guild.
func allTime(guildID, key string) stats.Filter {
	return stats.Filter{
		GuildID:      guildID,
		From:         time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		To:           time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC),
		CharacterKey: key,
	}
}

// loadCharacter degrades to no profile when TibiaData fails: the statistics still show.
func (s *Service) loadCharacter(ctx context.Context, p *stats.CharacterProfile) {
	ch, err := s.characters.GetCharacter(ctx, p.Name)
	switch {
	case errors.Is(err, ports.ErrCharacterNotFound):
		p.Source = stats.ProfileNotFound
	case err != nil:
		s.log.Warnw("character profile unavailable", "character", p.Name, "error", err)
		p.Source = stats.ProfileUnavailable
	default:
		p.Source = stats.ProfileFound
		p.Character = ch
		if ch.Name != "" {
			p.Name = ch.Name
		}
	}
}

// DailyHistory keeps the last snapshot of each local day, oldest first. Snapshots must be oldest first.
func DailyHistory(snapshots []experience.Snapshot) []stats.HistoryPoint {
	var out []stats.HistoryPoint
	for _, sn := range snapshots {
		day := corestats.Midnight(sn.ObservedAt.Local())
		point := stats.HistoryPoint{Day: day, Level: sn.Level, Experience: sn.Experience}
		if n := len(out); n > 0 && out[n-1].Day.Equal(day) {
			out[n-1] = point
			continue
		}
		out = append(out, point)
	}
	return out
}
