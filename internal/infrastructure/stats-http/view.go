package statshttp

import (
	"context"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/experience"
	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
)

// Chart colours: the signal orange for every series, as each chart shows one measure of one guild.
const (
	seriesColor = "#F97316"
	barColor    = "#FB923C"
)

type tableKind string

const (
	kindSpots      tableKind = "spots"
	kindPlayers    tableKind = "players"
	kindCharacters tableKind = "characters"
)

type pageView struct {
	GuildID   string
	GuildName string
	Range     stats.Range
	Picker    web.RangePickerProps
	// Today is the current local midnight: the daily charts mark it as not over yet.
	Today time.Time
}

type overviewView struct {
	pageView

	O *stats.Overview
}

// tableRow is one row of any stats table: a respawn, a player or a character.
type tableRow struct {
	stats.Totals

	Name     string
	Href     string
	Archived bool
}

type tableView struct {
	pageView

	Kind  tableKind
	Sort  stats.Sort
	Rows  []tableRow
	Total int
}

// breakdownView is a detail page's table: its first rows and the number of rows without the cap.
type breakdownView struct {
	Rows  []tableRow
	Total int
}

type spotView struct {
	pageView

	D          *stats.SpotDetail
	Players    breakdownView
	Characters breakdownView
}

type playerView struct {
	pageView

	D          *stats.PlayerDetail
	Spots      breakdownView
	Characters breakdownView
}

type characterView struct {
	pageView

	P     *stats.CharacterProfile
	Spots breakdownView
	Now   time.Time
}

func spotHref(guildID string, id int64) string {
	return web.GuildPath(guildID, "/stats/spots/"+strconv.FormatInt(id, 10))
}

func playerHref(guildID, userID string) string {
	return web.GuildPath(guildID, "/stats/players/"+url.PathEscape(userID))
}

func characterHref(guildID, name string) string {
	return web.GuildPath(guildID, "/characters/"+url.PathEscape(strings.TrimSpace(name)))
}

func spotRows(guildID string, rows []stats.SpotRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{Name: r.Name, Href: spotHref(guildID, r.SpotID), Archived: r.Archived, Totals: r.Totals}
	}
	return out
}

func playerRows(guildID string, rows []stats.PlayerRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{Name: r.Name, Href: playerHref(guildID, r.UserID), Totals: r.Totals}
	}
	return out
}

func characterRows(guildID string, rows []stats.CharacterRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{Name: r.Name, Href: characterHref(guildID, r.Name), Totals: r.Totals}
	}
	return out
}

func sortDir(s stats.Sort) string {
	if s.Asc {
		return "asc"
	}
	return "desc"
}

// sortHref is a column header link: the same column flips the direction, another column
// starts at its default.
func sortHref(path string, current stats.Sort, key stats.SortKey) string {
	q := url.Values{"sort": {string(key)}}
	if key == current.Key {
		next := stats.Sort{Key: key, Asc: !current.Asc}
		q.Set("dir", sortDir(next))
	}
	return path + "?" + q.Encode()
}

// ariaSort is the header's aria-sort value.
func ariaSort(current stats.Sort, key stats.SortKey) string {
	switch {
	case current.Key != key:
		return "none"
	case current.Asc:
		return "ascending"
	default:
		return "descending"
	}
}

func csvHref(v tableView) string {
	q := url.Values{"format": {"csv"}, "sort": {string(v.Sort.Key)}, "dir": {sortDir(v.Sort)}}
	return v.path() + "?" + q.Encode()
}

func (v tableView) path() string {
	return web.GuildPath(v.GuildID, "/stats/"+string(v.Kind))
}

func (v tableView) title(ctx context.Context) string {
	switch v.Kind {
	case kindPlayers:
		return i18n.T(ctx, "stats.players.heading")
	case kindCharacters:
		return i18n.T(ctx, "stats.characters.heading")
	default:
		return i18n.T(ctx, "stats.spots.heading")
	}
}

func (v tableView) lede(ctx context.Context) string {
	switch v.Kind {
	case kindPlayers:
		return i18n.T(ctx, "stats.players.lede")
	case kindCharacters:
		return i18n.T(ctx, "stats.characters.lede")
	default:
		return i18n.T(ctx, "stats.spots.lede")
	}
}

func nameHeader(ctx context.Context, kind tableKind) string {
	switch kind {
	case kindPlayers:
		return i18n.T(ctx, "stats.table.player")
	case kindCharacters:
		return i18n.T(ctx, "stats.table.character")
	default:
		return i18n.T(ctx, "stats.table.spot")
	}
}

type column struct {
	Key   stats.SortKey
	Label string
}

func figureColumns(ctx context.Context) []column {
	return []column{
		{stats.SortReservations, i18n.T(ctx, "stats.table.reservations")},
		{stats.SortHours, i18n.T(ctx, "stats.table.hours")},
		{stats.SortExp, i18n.T(ctx, "stats.table.exp")},
		{stats.SortExpPerHour, i18n.T(ctx, "stats.table.exp_per_hour")},
	}
}

func formatInt(ctx context.Context, n int64) string { return i18n.Int(ctx, n) }

func formatHours(ctx context.Context, t stats.Totals) string {
	return i18n.Decimal(ctx, t.Hours(), 1)
}

// formatExp is the experience figure; ok is false when there is no data, and the caller shows
// "no data", never 0.
func formatExp(ctx context.Context, t stats.Totals) (string, bool) {
	v := t.ExpTotal()
	if v == nil {
		return "", false
	}
	return i18n.Int(ctx, *v), true
}

func formatExpPerHour(ctx context.Context, t stats.Totals) (string, bool) {
	v := t.ExpPerHour()
	if v == nil {
		return "", false
	}
	return i18n.Int(ctx, int64(*v+0.5)), true
}

// coverage says how many reservations the experience figures cover.
func coverage(ctx context.Context, t stats.Totals) string {
	return i18n.T(ctx, "stats.exp.coverage", i18n.Int(ctx, t.ExpReservations), i18n.Int(ctx, t.Reservations))
}

// axisFormat formats the y ticks of a chart over vals: large figures shortened to 1.2K, 3.4M,
// 5.6B (localized suffixes), with enough decimals that neighbouring ticks differ.
func axisFormat(ctx context.Context, vals []float64) func(float64) string {
	lo, hi := 0.0, 0.0
	for i, v := range vals {
		if i == 0 || v < lo {
			lo = v
		}
		if i == 0 || v > hi {
			hi = v
		}
	}
	peak := math.Max(math.Abs(lo), math.Abs(hi))
	unit, suffix := 1.0, func(n string) string { return n }
	switch {
	case peak >= 1e9:
		unit, suffix = 1e9, func(n string) string { return i18n.T(ctx, "stats.number.billions", n) }
	case peak >= 1e6:
		unit, suffix = 1e6, func(n string) string { return i18n.T(ctx, "stats.number.millions", n) }
	case peak >= 1e4:
		unit, suffix = 1e3, func(n string) string { return i18n.T(ctx, "stats.number.thousands", n) }
	}
	// Four gridline intervals over the span (plus padding) must print as distinct labels.
	step := math.Max(hi-lo, math.Abs(hi)*0.1) / 4 / unit
	decimals := 0
	for decimals < 3 && step > 0 && step < math.Pow(10, float64(-decimals)) {
		decimals++
	}
	return func(v float64) string { return suffix(i18n.Decimal(ctx, v/unit, decimals)) }
}

func dayLabel(ctx context.Context, d time.Time) string { return i18n.Date(ctx, i18n.DayMonth, d) }

func dayLabels(ctx context.Context, days []stats.Day) []string {
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = dayLabel(ctx, d.Day)
	}
	return out
}

// dailyProps is a single-series line of one daily figure. A last point on today is partial.
func dailyProps(ctx context.Context, days []stats.Day, today time.Time, label string, value func(stats.Totals) float64, format func(stats.Totals) string) web.ChartProps {
	vals := make([]float64, len(days))
	tips := make([]string, len(days))
	partial := false
	for i, d := range days {
		vals[i] = value(d.Totals)
		tips[i] = dayLabel(ctx, d.Day) + " · " + format(d.Totals)
		if i == len(days)-1 && d.Day.Equal(today) {
			partial = true
			tips[i] = i18n.T(ctx, "stats.chart.today_tip", tips[i])
		}
	}
	return web.ChartProps{
		PartialLast: partial,
		Kind:        web.ChartLine,
		Series:      []web.ChartSeries{{Name: label, Color: seriesColor, Vals: vals, Tips: tips}},
		XLabels:     dayLabels(ctx, days),
		Height:      160,
		// The three daily charts share a row, so each is a third wide.
		MaxXLabels: 4,
		Area:       true,
		ZeroBase:   true,
		FormatY:    axisFormat(ctx, append([]float64{0}, vals...)),
		Label:      label,
	}
}

func reservationsChart(ctx context.Context, days []stats.Day, today time.Time) web.ChartProps {
	return dailyProps(ctx, days, today, i18n.T(ctx, "stats.chart.reservations"),
		func(t stats.Totals) float64 { return float64(t.Reservations) },
		func(t stats.Totals) string { return formatInt(ctx, t.Reservations) })
}

func hoursChart(ctx context.Context, days []stats.Day, today time.Time) web.ChartProps {
	return dailyProps(ctx, days, today, i18n.T(ctx, "stats.chart.hours"),
		func(t stats.Totals) float64 { return t.Hours() },
		func(t stats.Totals) string { return i18n.T(ctx, "stats.chart.hours_tip", formatHours(ctx, t)) })
}

// expDays keeps the days with experience data: a day without data is unknown, not 0, so the
// line skips it instead of dropping to the floor.
func expDays(days []stats.Day) []stats.Day {
	var out []stats.Day
	for _, d := range days {
		if d.HasExp() {
			out = append(out, d)
		}
	}
	return out
}

func expChart(ctx context.Context, days []stats.Day, today time.Time) web.ChartProps {
	return dailyProps(ctx, expDays(days), today, i18n.T(ctx, "stats.chart.exp"),
		func(t stats.Totals) float64 { return float64(t.Exp) },
		func(t stats.Totals) string {
			s, _ := formatExp(ctx, t)
			return s
		})
}

func topSpotsChart(ctx context.Context, guildID string, rows []stats.SpotRow) web.ChartProps {
	bars := make([]web.ChartBarItem, len(rows))
	for i, r := range rows {
		bars[i] = web.ChartBarItem{
			Name:  r.Name,
			Color: barColor,
			Val:   r.Hours(),
			Text:  i18n.T(ctx, "stats.chart.hours_tip", formatHours(ctx, r.Totals)),
			Href:  spotHref(guildID, r.SpotID),
		}
	}
	return web.ChartProps{Kind: web.ChartHBar, Bars: bars, Label: i18n.T(ctx, "stats.chart.top_spots")}
}

func historyChart(ctx context.Context, points []stats.HistoryPoint) web.ChartProps {
	vals := make([]float64, len(points))
	tips := make([]string, len(points))
	labels := make([]string, len(points))
	for i, p := range points {
		vals[i] = float64(p.Experience)
		labels[i] = dayLabel(ctx, p.Day)
		tips[i] = labels[i] + " · " + i18n.T(ctx, "stats.character.history_tip", strconv.Itoa(p.Level), i18n.Int(ctx, p.Experience))
	}
	label := i18n.T(ctx, "stats.character.history")
	return web.ChartProps{
		Kind:       web.ChartLine,
		Series:     []web.ChartSeries{{Name: label, Color: seriesColor, Vals: vals, Tips: tips}},
		XLabels:    labels,
		Height:     180,
		MaxXLabels: 6,
		Area:       true,
		FormatY:    axisFormat(ctx, vals),
		Label:      label,
	}
}

func dayAndHours(ctx context.Context, start, end time.Time) string {
	return i18n.Date(ctx, i18n.WeekdayDayMonth, start) + " " + start.Format("15:04") + "–" + end.Format("15:04")
}

// partyNames splits an author text into its characters, in order.
func partyNames(author string) []string {
	chars := experience.Characters(author)
	out := make([]string, len(chars))
	for i, c := range chars {
		out[i] = c.Name
	}
	return out
}

// reservationExp is the character's gain in one reservation: a figure, "no data", or pending
// while the reservation has not been attributed yet.
func reservationExp(ctx context.Context, r stats.CharacterReservation) (text string, state string) {
	switch {
	case r.Status == experience.StatusOK && r.Gain != nil:
		return i18n.Int(ctx, *r.Gain), "ok"
	case r.Status == experience.StatusNoData:
		return i18n.T(ctx, "stats.exp.no_data"), "no_data"
	default:
		return i18n.T(ctx, "stats.exp.pending"), "pending"
	}
}

func lastLogin(ctx context.Context, t *time.Time, now time.Time) string {
	if t == nil {
		return i18n.T(ctx, "stats.character.never")
	}
	return i18n.Ago(ctx, *t, now) + " · " + i18n.Date(ctx, i18n.DayMonthYearTime, t.Local())
}

func guildLine(ctx context.Context, name, rank string) string {
	switch {
	case name == "":
		return i18n.T(ctx, "stats.character.no_guild")
	case rank == "":
		return name
	default:
		return i18n.T(ctx, "stats.character.guild_rank", rank, name)
	}
}

func historyEmpty(ctx context.Context, world string) string {
	if world == "" {
		return i18n.T(ctx, "stats.character.history_no_world")
	}
	return i18n.T(ctx, "stats.character.history_empty", world)
}
