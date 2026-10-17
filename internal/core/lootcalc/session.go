package lootcalc

import (
	"strings"
	"time"

	"spot-assistant/internal/core/dto/loot"
)

// stampLayout is how the analyser prints a session time ("2026-09-25, 20:14:39").
const stampLayout = "2006-01-02, 15:04:05"

// parseStamp reads an analyser time in loc; the analyser prints the client's local time
// without a zone.
func parseStamp(s string, loc *time.Location) (time.Time, bool) {
	t, err := time.ParseInLocation(stampLayout, strings.TrimSpace(s), loc)
	return t, err == nil
}

// Interval reads two analyser stamps in loc. ok is false without both or when to is not after from.
func Interval(fromStamp, toStamp string, loc *time.Location) (from, to time.Time, ok bool) {
	from, okFrom := parseStamp(fromStamp, loc)
	to, okTo := parseStamp(toStamp, loc)
	if !okFrom || !okTo || !to.After(from) {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
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
