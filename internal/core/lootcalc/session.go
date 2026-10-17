package lootcalc

import (
	"strings"
	"time"

	"spot-assistant/internal/core/dto/loot"
)

// StampLayout is how the analyser prints a session time ("2026-09-25, 20:14:39").
const StampLayout = "2006-01-02, 15:04:05"

// ParseStamp reads an analyser time in loc; the analyser prints the client's local time
// without a zone.
func ParseStamp(s string, loc *time.Location) (time.Time, bool) {
	t, err := time.ParseInLocation(StampLayout, strings.TrimSpace(s), loc)
	return t, err == nil
}

// Interval reads two analyser stamps in loc. ok is false without both or when to is not after from.
func Interval(fromStamp, toStamp string, loc *time.Location) (from, to time.Time, ok bool) {
	from, okFrom := ParseStamp(fromStamp, loc)
	to, okTo := ParseStamp(toStamp, loc)
	if !okFrom || !okTo || !to.After(from) {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

// Interval is the session's From and To in loc.
func (s Session) Interval(loc *time.Location) (from, to time.Time, ok bool) {
	return Interval(s.From, s.To, loc)
}

// Submission is the session as the loot session service takes it.
func (s Session) Submission() loot.Submission {
	members := make([]loot.Member, len(s.Players))
	for i, p := range s.Players {
		members[i] = loot.Member{
			Name: p.Name, Leader: p.Leader,
			Loot: p.Loot, Supplies: p.Supplies, Balance: p.Balance, Damage: p.Damage, Healing: p.Healing,
		}
	}
	return loot.Submission{From: s.From, To: s.To, Members: members}
}
