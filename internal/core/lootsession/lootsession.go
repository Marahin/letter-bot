// Package lootsession counts the Loot Calculator sessions in the stats: it stores a session
// and attributes it to the reservation it was hunted in.
package lootsession

import (
	"context"
	"crypto/sha256"
	"slices"
	"strconv"
	"strings"
	"time"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/loot"
	"spot-assistant/internal/core/lootcalc"
	"spot-assistant/internal/ports"
)

const (
	// FutureSlack tolerates a client clock a little ahead of ours.
	FutureSlack = 5 * time.Minute
	// MaxFigure bounds each member figure (gp, damage, healing): far above any hunt, and the party
	// sum of MaxPartySize members stays far from the int64 limit.
	MaxFigure int64 = 1e12
	// MaxPartySize is the most pasted lines a counted session may have.
	MaxPartySize = 10
	// MinDuration drops a session too short to say anything per hour.
	MinDuration = time.Minute
)

type Service struct {
	repo ports.LootSessionRepository
	// loc reads the analyser's zone-less stamps (the process zone, D31).
	loc *time.Location
	now func() time.Time
}

func New(repo ports.LootSessionRepository, loc *time.Location) *Service {
	return &Service{repo: repo, loc: loc, now: time.Now}
}

func (s *Service) Record(ctx context.Context, sub loot.Submission, count bool) (loot.Outcome, error) {
	if !count {
		return loot.Outcome{Status: loot.StatusUnticked}, nil
	}
	from, to, ok := lootcalc.Interval(sub.From, sub.To, s.loc)
	if !ok {
		return loot.Outcome{Status: loot.StatusNoTime}, nil
	}
	if !inRange(sub, to.Sub(from)) {
		return loot.Outcome{Status: loot.StatusOutOfRange}, nil
	}
	if to.After(s.now().Add(FutureSlack)) {
		return loot.Outcome{Status: loot.StatusFuture}, nil
	}
	session := NewSession(sub, from, to)
	if len(session.Members) == 0 {
		return loot.Outcome{Status: loot.StatusOutOfRange}, nil
	}

	id, err := s.repo.SaveSession(ctx, session)
	if err != nil {
		return loot.Outcome{}, err
	}
	keys := make([]string, len(session.Members))
	for i, m := range session.Members {
		keys[i] = m.Key
	}
	candidates, err := s.repo.MatchCandidates(ctx, from, to, keys, booking.MaximumReservationLength)
	if err != nil {
		return loot.Outcome{}, err
	}
	match, overlap := pick(candidates, from, to)
	if match == nil {
		return loot.Outcome{Status: loot.StatusUnmatched}, nil
	}
	counted, err := s.repo.Pick(ctx, loot.Attribution{
		SessionID:      id,
		ReservationID:  match.ReservationID,
		OverlapSeconds: int64(overlap / time.Second),
		ByAuthor:       sub.SubmittedBy != "" && sub.SubmittedBy == match.AuthorDiscordID,
	})
	if err != nil {
		return loot.Outcome{}, err
	}
	if !counted {
		return loot.Outcome{Status: loot.StatusOutmatched, Match: match}, nil
	}
	return loot.Outcome{Status: loot.StatusCounted, Match: match}, nil
}

// inRange bounds what a public form may store: the length of a reservation at most,
// a Party Hunt's size, and figures no hunt reaches.
func inRange(sub loot.Submission, duration time.Duration) bool {
	if duration < MinDuration || duration > booking.MaximumReservationLength {
		return false
	}
	if len(sub.Members) == 0 || len(sub.Members) > MaxPartySize {
		return false
	}
	for _, m := range sub.Members {
		for _, v := range []int64{m.Loot, m.Supplies, m.Balance, m.Damage, m.Healing} {
			if v > MaxFigure || v < -MaxFigure {
				return false
			}
		}
	}
	return true
}

// NewSession builds the stored session. A character named twice keeps its first line; the
// party figures and the member count still cover every line, as the split does.
func NewSession(sub loot.Submission, from, to time.Time) loot.Session {
	s := loot.Session{SubmittedBy: sub.SubmittedBy, StartedAt: from, EndedAt: to, Duration: to.Sub(from), PartySize: len(sub.Members)}
	seen := map[string]bool{}
	for _, m := range sub.Members {
		s.Loot += m.Loot
		s.Supplies += m.Supplies
		s.Balance += m.Balance
		s.Damage += m.Damage
		s.Healing += m.Healing
		key := experience.CharacterKey(m.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		m.Name = strings.TrimSpace(m.Name)
		s.Members = append(s.Members, loot.SessionMember{Member: m, Key: key, Position: len(s.Members)})
	}
	s.Hash = Hash(from, to, s.Members)
	return s
}

// Hash identifies a session by its time and figures, not by the paste: the same hunt pasted
// again, in any member order, has the same hash.
func Hash(from, to time.Time, members []loot.SessionMember) []byte {
	sorted := slices.Clone(members)
	slices.SortFunc(sorted, func(a, b loot.SessionMember) int { return strings.Compare(a.Key, b.Key) })
	h := sha256.New()
	h.Write([]byte("v1\x00" + strconv.FormatInt(from.Unix(), 10) + "\x00" + strconv.FormatInt(to.Unix(), 10) + "\x00"))
	for _, m := range sorted {
		figures := []int64{m.Loot, m.Supplies, m.Balance, m.Damage, m.Healing}
		fields := make([]string, 0, 2+len(figures))
		fields = append(fields, m.Key, strconv.FormatBool(m.Leader))
		for _, v := range figures {
			fields = append(fields, strconv.FormatInt(v, 10))
		}
		h.Write([]byte(strings.Join(fields, "\x00") + "\n"))
	}
	return h.Sum(nil)
}

// pick is the candidate that overlaps the session by at least half its length: the largest
// overlap, then the latest start, then the larger id.
func pick(candidates []loot.Candidate, from, to time.Time) (*loot.Candidate, time.Duration) {
	duration := to.Sub(from)
	var best *loot.Candidate
	var bestOverlap time.Duration
	for i := range candidates {
		c := &candidates[i]
		overlap := minTime(to, c.EndAt).Sub(maxTime(from, c.StartAt))
		if overlap < time.Second || 2*overlap < duration {
			continue
		}
		if best == nil || better(c, overlap, best, bestOverlap) {
			best, bestOverlap = c, overlap
		}
	}
	return best, bestOverlap
}

func better(c *loot.Candidate, overlap time.Duration, best *loot.Candidate, bestOverlap time.Duration) bool {
	switch {
	case overlap != bestOverlap:
		return overlap > bestOverlap
	case !c.StartAt.Equal(best.StartAt):
		return c.StartAt.After(best.StartAt)
	default:
		return c.ReservationID > best.ReservationID
	}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
