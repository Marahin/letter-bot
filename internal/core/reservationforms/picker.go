package reservationforms

import (
	"cmp"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
)

const (
	// SelectOptions is Discord's limit of options in one select.
	SelectOptions = 25
	// GroupsPerPage leaves two of Discord's five rows: the usual respawns and the buttons.
	GroupsPerPage = 3
	PageSize      = SelectOptions * GroupsPerPage

	SlotStep    = 30 * time.Minute
	WindowSlots = 24
	// Windows is the number of start windows: 4 x 12 hours.
	Windows = 4

	UsualPeriod   = 90 * 24 * time.Hour
	PopularPeriod = 30 * 24 * time.Hour

	maxPrefix = 3
)

// ErrChoiceIncomplete means the member pressed Book before choosing a start and a length.
var ErrChoiceIncomplete = errors.New("choose a start and a length")

// PageRespawns sorts the respawns by name and returns one page of them, cut into
// up to GroupsPerPage groups of balanced size. page is clamped.
func PageRespawns(spots []*spot.Spot, page int) reservation.RespawnPicker {
	pages := groupPages(spots)
	page = min(max(page, 0), len(pages)-1)
	return reservation.RespawnPicker{Groups: pages[page], Page: page, Pages: len(pages)}
}

// groupPages returns at least one page, so an empty list is one empty page.
func groupPages(spots []*spot.Spot) [][]reservation.RespawnGroup {
	sorted := slices.Clone(spots)
	slices.SortStableFunc(sorted, func(a, b *spot.Spot) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	if len(sorted) == 0 {
		return [][]reservation.RespawnGroup{{}}
	}
	pages := [][]reservation.RespawnGroup{}
	var all []*reservation.RespawnGroup
	for start := 0; start < len(sorted); start += PageSize {
		groups := balance(sorted[start:min(start+PageSize, len(sorted))])
		for i := range groups {
			all = append(all, &groups[i])
		}
		pages = append(pages, groups)
	}
	labelGroups(all)
	return pages
}

func balance(spots []*spot.Spot) []reservation.RespawnGroup {
	n := (len(spots) + SelectOptions - 1) / SelectOptions
	size, extra := len(spots)/n, len(spots)%n
	groups := make([]reservation.RespawnGroup, 0, n)
	at := 0
	for i := range n {
		end := at + size
		if i < extra {
			end++
		}
		group := reservation.RespawnGroup{Spots: make([]reservation.Spot, 0, end-at)}
		for _, sp := range spots[at:end] {
			group.Spots = append(group.Spots, reservation.Spot{ID: sp.ID, Name: sp.Name})
		}
		groups = append(groups, group)
		at = end
	}
	return groups
}

// labelGroups names each group by the prefixes of its first and last names. A
// prefix grows until it differs from the neighbour across the boundary, so two
// groups never seem to overlap ("A–Ka", "Ko–Z").
func labelGroups(groups []*reservation.RespawnGroup) {
	for i, g := range groups {
		first, last := g.Spots[0].Name, g.Spots[len(g.Spots)-1].Name
		fromLength, toLength := 1, 1
		if i > 0 {
			prev := groups[i-1].Spots
			fromLength = distinctLength(prev[len(prev)-1].Name, first)
		}
		if i < len(groups)-1 {
			toLength = distinctLength(last, groups[i+1].Spots[0].Name)
		}
		g.From, g.To = prefix(first, fromLength), prefix(last, toLength)
	}
}

func distinctLength(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	for n := 1; n < maxPrefix; n++ {
		if n > len(ra) || n > len(rb) || string(ra[:n]) != string(rb[:n]) {
			return n
		}
	}
	return maxPrefix
}

func prefix(name string, n int) string {
	runes := []rune(strings.ToLower(strings.TrimSpace(name)))
	if len(runes) > n {
		runes = runes[:n]
	}
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return strings.TrimSpace(string(runes))
}

// UsualRespawns lists the member's most booked respawns, then the server's most
// popular ones, without duplicates and archived ones, up to limit.
func UsualRespawns(mine, popular []spot.Ranked, limit int) []reservation.UsualRespawn {
	byRank := func(a, b spot.Ranked) int {
		if c := cmp.Compare(b.Bookings, a.Bookings); c != 0 {
			return c
		}
		if c := b.LastStartAt.Compare(a.LastStartAt); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	}
	mine, popular = slices.Clone(mine), slices.Clone(popular)
	slices.SortStableFunc(mine, byRank)
	slices.SortStableFunc(popular, byRank)

	out := make([]reservation.UsualRespawn, 0, limit)
	seen := map[int64]bool{}
	add := func(r spot.Ranked, bookings int64) {
		if len(out) == limit || seen[r.ID] || r.IsArchived() {
			return
		}
		seen[r.ID] = true
		out = append(out, reservation.UsualRespawn{Spot: reservation.Spot{ID: r.ID, Name: r.Name}, Bookings: bookings})
	}
	for _, r := range mine {
		add(r, r.Bookings)
	}
	for _, r := range popular {
		add(r, 0)
	}
	return out
}

// firstSlot is the first :00 or :30 after now.
func firstSlot(now time.Time) time.Time {
	minute := now.Truncate(time.Minute)
	return minute.Add(time.Duration(30-now.Minute()%30) * time.Minute)
}

// StartSlots are the half-hour starts of one 12-hour window. They step in
// elapsed time, so a DST change shows a repeated or a skipped hour.
func StartSlots(now time.Time, window int) []time.Time {
	first := firstSlot(now).Add(time.Duration(window*WindowSlots) * SlotStep)
	slots := make([]time.Time, 0, WindowSlots)
	for i := range WindowSlots {
		slots = append(slots, first.Add(time.Duration(i)*SlotStep))
	}
	return slots
}

// WindowOf is the start window that shows the start, clamped to the windows.
func WindowOf(startAt, now time.Time) int {
	elapsed := startAt.Sub(firstSlot(now))
	if elapsed < 0 {
		return 0
	}
	return min(int(elapsed/(WindowSlots*SlotStep)), Windows-1)
}

// Lengths are the half-hour lengths up to the longest reservation.
func Lengths() []time.Duration {
	lengths := []time.Duration{}
	for d := SlotStep; d <= booking.MaximumReservationLength; d += SlotStep {
		lengths = append(lengths, d)
	}
	return lengths
}

// ResolveStart turns the choice into a start. A slot that passed less than a
// slot ago (an old message) starts now.
func ResolveStart(choice reservation.TimeChoice, now time.Time) (time.Time, error) {
	current := now.Truncate(time.Minute)
	switch {
	case choice.Now:
		return current, nil
	case choice.StartAt.IsZero():
		return time.Time{}, ErrChoiceIncomplete
	case !choice.StartAt.Before(current):
		return choice.StartAt, nil
	case current.Sub(choice.StartAt) < SlotStep:
		return current, nil
	}
	return time.Time{}, booking.ErrStartInPast
}

// startOptions builds the start list of a window. current is the start of the
// edited reservation, or zero.
func startOptions(now time.Time, window int, choice reservation.TimeChoice, current time.Time, booked []*reservation.Reservation) []reservation.StartOption {
	options := []reservation.StartOption{}
	slots := StartSlots(now, window)
	offGrid := !current.IsZero() && WindowOf(current, now) == window && !slices.ContainsFunc(slots, current.Equal)
	if offGrid {
		options = append(options, reservation.StartOption{StartAt: current, Current: true})
	}
	// A select holds 25 options. The current start takes the place of Now, so no slot is lost.
	if window == 0 && !offGrid {
		options = append(options, reservation.StartOption{StartAt: now.Truncate(time.Minute), Now: true, Selected: choice.Now})
	}
	for _, slot := range slots {
		options = append(options, reservation.StartOption{StartAt: slot})
	}
	for i := range options {
		o := &options[i]
		o.BookedBy = holder(o.StartAt, booked)
		if !o.Now && !choice.Now {
			o.Selected = o.StartAt.Equal(choice.StartAt)
		}
	}
	return options
}

func holder(at time.Time, booked []*reservation.Reservation) *reservation.Reservation {
	for _, r := range booked {
		if !at.Before(r.StartAt) && at.Before(r.EndAt) {
			return r
		}
	}
	return nil
}

// lengthOptions lists the lengths. current is the length of the edited
// reservation, or 0. notAfter drops the lengths that end by then (an ongoing
// reservation).
func lengthOptions(chosen, current time.Duration, startAt, notAfter time.Time) []reservation.LengthOption {
	options := []reservation.LengthOption{}
	lengths := Lengths()
	if current > 0 && !slices.Contains(lengths, current) {
		options = append(options, reservation.LengthOption{Length: current, Current: true})
	}
	for _, d := range lengths {
		if !notAfter.IsZero() && !startAt.Add(d).After(notAfter) {
			continue
		}
		options = append(options, reservation.LengthOption{Length: d})
	}
	for i := range options {
		options[i].Selected = options[i].Length == chosen
	}
	return options
}
