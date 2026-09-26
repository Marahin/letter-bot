package toolshttp

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"spot-assistant/internal/core/lootcalc"
	"spot-assistant/internal/infrastructure/i18n"
)

// entryScriptID is the JSON island of a result that loot-calculator.js saves.
const entryScriptID = "loot-entry"

var errTooLong = errors.New("session text is too long")

type lootView struct {
	Text   string
	Error  error
	Result *resultView
}

type resultView struct {
	Session lootcalc.Session
	Split   lootcalc.Result
	// Players are sorted by balance, highest first.
	Players   []lootcalc.PartyMember
	Discord   string
	TeamSpeak string
	// Entry is the history record loot-calculator.js keeps in localStorage.
	Entry historyEntry
}

// historyEntry is one saved session. Key identifies the same hunt, as the
// original did with createdAt + names (plus the total, for a paste without the
// header), so a recalculation is not saved twice.
type historyEntry struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Names []string `json:"names"`
	Total int64    `json:"total"`
	Text  string   `json:"text"`
}

func newResultView(text string, s lootcalc.Session, r lootcalc.Result) *resultView {
	players := append([]lootcalc.PartyMember(nil), s.Players...)
	sort.SliceStable(players, func(i, j int) bool { return players[i].Balance > players[j].Balance })

	names := make([]string, 0, len(s.Players))
	for _, p := range s.Players {
		names = append(names, p.Name)
	}
	entry := historyEntry{
		Key:   s.From + "|" + s.To + "|" + strings.Join(names, "|") + "|" + strconv.FormatInt(r.Total, 10),
		Label: sessionLabel(s),
		Names: names,
		Total: r.Total,
		Text:  text,
	}
	return &resultView{
		Session:   s,
		Split:     r,
		Players:   players,
		Discord:   lootcalc.DiscordText(r),
		TeamSpeak: lootcalc.TeamSpeakText(r),
		Entry:     entry,
	}
}

// sessionLabel shortens the analyser header to "2026-09-25, 20:14 → 21:27", or
// "" when the paste had no header (the script then uses the save time).
func sessionLabel(s lootcalc.Session) string {
	if s.From == "" {
		return ""
	}
	fromDate, fromTime := splitStamp(s.From)
	toDate, toTime := splitStamp(s.To)
	if toDate != fromDate {
		toTime = toDate + ", " + toTime
	}
	return fromDate + ", " + fromTime + " → " + toTime
}

func splitStamp(stamp string) (date, clock string) {
	date, clock, _ = strings.Cut(stamp, ", ")
	if len(clock) > 5 {
		clock = clock[:5]
	}
	return date, clock
}

// digits is an amount for display: dot groups as in the copied text, and a true
// minus sign.
func digits(n int64) string {
	return strings.Replace(lootcalc.GroupDigits(n), "-", "−", 1)
}

func amount(n int64) string {
	return digits(n) + " gp"
}

func duration(s lootcalc.Session) string {
	if s.Duration == "" {
		return "—"
	}
	return s.Duration
}

func errorText(ctx context.Context, err error) string {
	var pe *lootcalc.ParseError
	switch {
	case errors.Is(err, errTooLong):
		return i18n.T(ctx, "tools.loot.error.too_long")
	case errors.Is(err, lootcalc.ErrEmpty):
		return i18n.T(ctx, "tools.loot.error.empty")
	case errors.Is(err, lootcalc.ErrMissingFields) && errors.As(err, &pe):
		return i18n.T(ctx, "tools.loot.error.missing", pe.Player, joinAnd(ctx, pe.Missing))
	case errors.Is(err, lootcalc.ErrMalformedValue) && errors.As(err, &pe):
		return i18n.T(ctx, "tools.loot.error.malformed", pe.Line, pe.Text)
	default:
		return i18n.T(ctx, "tools.loot.error.no_players")
	}
}

// joinAnd lists field names: "Damage", "Damage and Healing", "Loot, Damage and Healing".
func joinAnd(ctx context.Context, items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " " + i18n.T(ctx, "tools.loot.error.and") + " " + items[len(items)-1]
}
