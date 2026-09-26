package statshttp

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/stats"
	corestats "spot-assistant/internal/core/stats"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/ports"
)

// maxTableRows caps the HTML tables; the CSV export has every row.
const maxTableRows = 500

// maxBreakdownRows caps the tables on a detail page.
const maxBreakdownRows = 25

type Handlers struct {
	D   *web.Deps
	now func() time.Time
}

func New(d *web.Deps) *Handlers { return &Handlers{D: d, now: time.Now} }

func (h *Handlers) HandleOverview(w http.ResponseWriter, r *http.Request) {
	p, ok := h.page(w, r)
	if !ok {
		return
	}
	o, err := h.D.Stats.Overview(r.Context(), p.GuildID, p.Range)
	if err != nil {
		h.D.ServerError(w, r, "stats overview", err)
		return
	}
	h.D.Render(w, r, Overview(h.D.Cfg.BaseURL, overviewView{pageView: p, O: o}, h.nav(r, p, "stats")))
}

func (h *Handlers) HandleSpots(w http.ResponseWriter, r *http.Request) {
	h.table(w, r, kindSpots, func(ctx context.Context, p pageView, s stats.Sort) ([]tableRow, error) {
		rows, err := h.D.Stats.Spots(ctx, p.GuildID, p.Range, s)
		return spotRows(p.GuildID, rows), err
	})
}

func (h *Handlers) HandlePlayers(w http.ResponseWriter, r *http.Request) {
	h.table(w, r, kindPlayers, func(ctx context.Context, p pageView, s stats.Sort) ([]tableRow, error) {
		rows, err := h.D.Stats.Players(ctx, p.GuildID, p.Range, s)
		return playerRows(p.GuildID, rows), err
	})
}

func (h *Handlers) HandleCharacters(w http.ResponseWriter, r *http.Request) {
	h.table(w, r, kindCharacters, func(ctx context.Context, p pageView, s stats.Sort) ([]tableRow, error) {
		rows, err := h.D.Stats.Characters(ctx, p.GuildID, p.Range, s)
		return characterRows(p.GuildID, rows), err
	})
}

func (h *Handlers) table(w http.ResponseWriter, r *http.Request, kind tableKind, load func(context.Context, pageView, stats.Sort) ([]tableRow, error)) {
	q := r.URL.Query()
	sort := corestats.ParseSort(q.Get("sort"), q.Get("dir"))
	p, ok := h.page(w, r, web.RangePickerHidden{Name: "sort", Value: string(sort.Key)}, web.RangePickerHidden{Name: "dir", Value: sortDir(sort)})
	if !ok {
		return
	}
	rows, err := load(r.Context(), p, sort)
	if err != nil {
		h.D.ServerError(w, r, "stats table", err)
		return
	}
	if q.Get("format") == "csv" {
		h.csv(w, r, kind, p, rows)
		return
	}
	v := tableView{pageView: p, Kind: kind, Sort: sort, Total: len(rows), Rows: rows}
	if len(rows) > maxTableRows {
		v.Rows = rows[:maxTableRows]
	}
	active := "stats-" + string(kind)
	h.D.Render(w, r, Table(h.D.Cfg.BaseURL, v, h.nav(r, p, active)))
}

func (h *Handlers) HandleSpot(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "spot")
	if !ok {
		return
	}
	p, ok := h.page(w, r)
	if !ok {
		return
	}
	detail, err := h.D.Stats.Spot(r.Context(), p.GuildID, id, p.Range)
	if errors.Is(err, ports.ErrNotFound) {
		h.D.NotFound(w, r)
		return
	}
	if err != nil {
		h.D.ServerError(w, r, "stats spot", err)
		return
	}
	v := spotView{pageView: p, D: detail, Players: playerRows(p.GuildID, detail.Players), Characters: characterRows(p.GuildID, detail.Characters)}
	h.D.Render(w, r, SpotPage(h.D.Cfg.BaseURL, v, h.nav(r, p, "stats-spots")))
}

func (h *Handlers) HandlePlayer(w http.ResponseWriter, r *http.Request) {
	p, ok := h.page(w, r)
	if !ok {
		return
	}
	detail, err := h.D.Stats.Player(r.Context(), p.GuildID, r.PathValue("user"), p.Range)
	if errors.Is(err, ports.ErrNotFound) {
		h.D.NotFound(w, r)
		return
	}
	if err != nil {
		h.D.ServerError(w, r, "stats player", err)
		return
	}
	v := playerView{pageView: p, D: detail, Spots: spotRows(p.GuildID, detail.Spots), Characters: characterRows(p.GuildID, detail.Characters)}
	h.D.Render(w, r, PlayerPage(h.D.Cfg.BaseURL, v, h.nav(r, p, "stats-players")))
}

func (h *Handlers) HandleCharacter(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	p, ok := h.page(w, r)
	if !ok {
		return
	}
	profile, err := h.D.Characters.Profile(r.Context(), p.GuildID, name, p.Range)
	if errors.Is(err, ports.ErrNotFound) {
		h.D.NotFound(w, r)
		return
	}
	if err != nil {
		h.D.ServerError(w, r, "character profile", err)
		return
	}
	v := characterView{pageView: p, P: profile, Spots: spotRows(p.GuildID, profile.Spots), Now: h.now()}
	h.D.Render(w, r, CharacterPage(h.D.Cfg.BaseURL, v, h.nav(r, p, "stats-characters")))
}

// page resolves the guild and the day range every Stats page shares, and persists a range the
// URL pinned. hidden are the query values the picker must carry over.
func (h *Handlers) page(w http.ResponseWriter, r *http.Request, hidden ...web.RangePickerHidden) (pageView, bool) {
	ctx := r.Context()
	current, ok := web.CurrentAccessFrom(ctx)
	if !ok {
		h.D.ServerError(w, r, "stats without a guild guard", errors.New("no guild access in context"))
		return pageView{}, false
	}
	guildID := current.Config.GuildID
	rng := corestats.ResolveRange(parseDays(web.RangeSelectionFrom(ctx).Query), h.now())
	// Only the first and last day: a 400-day list would outgrow the 4 KB cookie limit, and
	// ResolveRange expands the two back to the span.
	h.D.PersistRangeSelection(w, r, guildID, spanEnds(rng.Days))
	return pageView{
		GuildID:   guildID,
		GuildName: current.Config.Name,
		Range:     rng,
		Picker:    h.picker(ctx, r, current, rng, hidden),
		Today:     corestats.Midnight(h.now()),
	}, true
}

func (h *Handlers) picker(ctx context.Context, r *http.Request, current access.GuildAccess, rng stats.Range, hidden []web.RangePickerHidden) web.RangePickerProps {
	dataDays, err := h.D.Stats.DataDays(ctx, current.Config.GuildID, h.now())
	if err != nil {
		// The dots are a hint; the page still works without them.
		h.D.Log.Warnw("stats data days", "guild", current.Config.GuildID, "error", err)
	}
	return web.RangePickerFor(ctx, web.RangePickerProps{
		Action:       r.URL.Path,
		Label:        rangeLabel(ctx, rng),
		SelectedDays: dayKeys(rng.Days),
		DataDays:     dayKeys(dataDays),
		Explicit:     web.RangeSelectionFrom(ctx).Explicit,
		DataLegend:   i18n.T(ctx, "stats.range.days_with_data"),
		Hidden:       hidden,
	})
}

func (h *Handlers) nav(r *http.Request, p pageView, active string) web.Nav {
	nav := h.D.Nav(r, p.GuildID)
	nav.Active = active
	return nav
}

// parseDays reads the picker's "days" list, or the noscript from/to span, as local midnights.
// Nothing valid means no selection (the page default).
func parseDays(q url.Values) []time.Time {
	var days []time.Time
	for _, part := range strings.Split(q.Get("days"), ",") {
		if d, ok := parseDay(part); ok {
			days = append(days, d)
		}
	}
	if len(days) > 0 {
		return days
	}
	for _, key := range []string{"from", "to"} {
		if d, ok := parseDay(q.Get(key)); ok {
			days = append(days, d)
		}
	}
	return days
}

func parseDay(s string) (time.Time, bool) {
	t, err := time.ParseInLocation(time.DateOnly, strings.TrimSpace(s), time.Local)
	return t, err == nil
}

func spanEnds(days []time.Time) []time.Time {
	if len(days) < 2 {
		return days
	}
	return []time.Time{days[0], days[len(days)-1]}
}

func dayKeys(days []time.Time) []string {
	out := make([]string, len(days))
	for i, d := range days {
		out[i] = d.Format(time.DateOnly)
	}
	return out
}

// rangeLabel names the span: one day, or "02 Jan – 08 Jan 2026".
func rangeLabel(ctx context.Context, rng stats.Range) string {
	if len(rng.Days) == 0 {
		return ""
	}
	first, last := rng.Days[0], rng.Days[len(rng.Days)-1]
	if len(rng.Days) == 1 {
		return i18n.Date(ctx, i18n.DayMonthYear, first)
	}
	if first.Year() == last.Year() {
		return i18n.Date(ctx, i18n.DayMonth, first) + " – " + i18n.Date(ctx, i18n.DayMonthYear, last)
	}
	return i18n.Date(ctx, i18n.DayMonthYear, first) + " – " + i18n.Date(ctx, i18n.DayMonthYear, last)
}
