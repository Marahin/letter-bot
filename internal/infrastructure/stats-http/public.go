package statshttp

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/a-h/templ"

	"spot-assistant/internal/core/dto/stats"
	corestats "spot-assistant/internal/core/stats"
	"spot-assistant/internal/core/worldstats"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/ports"
)

// HandleWorlds lists every world of a public server. Like the three pages below, it needs no
// sign-in and no premium of the visitor: it shows what each server's public stats already show.
func (h *Handlers) HandleWorlds(w http.ResponseWriter, r *http.Request) {
	p := h.crossPage(w, r)
	rows, err := h.D.Worlds.Worlds(r.Context(), p.Range)
	if err != nil {
		h.D.ServerError(w, r, "worlds", err)
		return
	}
	h.D.Render(w, r, WorldsPage(h.D.Cfg.BaseURL, worldsView{crossPage: p, Rows: rows}, h.crossNav(r, "worlds")))
}

func (h *Handlers) HandleWorld(w http.ResponseWriter, r *http.Request) {
	h.crossDetail(w, r, "world", func(p crossPage) (templ.Component, error) {
		d, err := h.D.Worlds.World(r.Context(), r.PathValue("world"), p.Range)
		if err != nil {
			return nil, err
		}
		return WorldPage(h.D.Cfg.BaseURL, worldView{crossPage: p, D: d}, h.crossNav(r, "worlds")), nil
	})
}

// HandleSpawns is the spawn directory: every respawn name, sortable, narrowed to one world.
func (h *Handlers) HandleSpawns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query()
	sort := parseSort(stats.SpawnSortKeys, q.Get("sort"), q.Get("dir"))
	world := worldstats.Key(q.Get("world"))
	hidden := []web.RangePickerHidden{{Name: "sort", Value: string(sort.Key)}, {Name: "dir", Value: sortDir(sort)}}
	if world != "" {
		hidden = append(hidden, web.RangePickerHidden{Name: "world", Value: world})
	}
	p := h.crossPage(w, r, hidden...)
	csv := q.Get("format") == "csv"
	limit := maxTableRows
	if csv {
		limit = maxCSVRows
	}
	page, err := h.D.Worlds.Spawns(ctx, p.Range, world, sort, limit)
	if err != nil {
		h.D.ServerError(w, r, "spawns", err)
		return
	}
	rows := spawnRows(page.Rows)
	if csv {
		name := "spawns" + rangeSuffix(p.Range)
		if world != "" {
			name += "-" + url.PathEscape(world)
		}
		h.writeCSV(w, name, nameHeader(ctx, kindSpots), spawnColumns(ctx), rows)
		return
	}
	worlds, err := h.D.Worlds.PublicWorlds(ctx)
	if err != nil {
		// The filter is a convenience; the table still shows.
		h.D.Log.Warnw("public worlds", "error", err)
	}
	v := spawnsView{crossPage: p, Worlds: worlds, World: world, Sort: sort, Rows: rows, Total: page.Total}
	h.D.Render(w, r, SpawnsPage(h.D.Cfg.BaseURL, v, h.crossNav(r, "spawns")))
}

func (h *Handlers) HandleSpawn(w http.ResponseWriter, r *http.Request) {
	h.crossDetail(w, r, "spawn", func(p crossPage) (templ.Component, error) {
		d, err := h.D.Worlds.Spawn(r.Context(), r.PathValue("key"), p.Range)
		if err != nil {
			return nil, err
		}
		return SpawnPage(h.D.Cfg.BaseURL, spawnView{crossPage: p, D: d}, h.crossNav(r, "spawns")), nil
	})
}

// crossDetail renders a cross-server detail page that build loads; ErrNotFound is a 404.
func (h *Handlers) crossDetail(w http.ResponseWriter, r *http.Request, what string, build func(crossPage) (templ.Component, error)) {
	page, err := build(h.crossPage(w, r))
	if errors.Is(err, ports.ErrNotFound) {
		h.D.NotFound(w, r)
		return
	}
	if err != nil {
		h.D.ServerError(w, r, what, err)
		return
	}
	h.D.Render(w, r, page)
}

// crossPage resolves the day range the cross-server pages share (their own cookie), and
// persists a range the URL pinned. The picker marks no days with data: that would take a
// query over every server.
func (h *Handlers) crossPage(w http.ResponseWriter, r *http.Request, hidden ...web.RangePickerHidden) crossPage {
	ctx := r.Context()
	sel := web.RangeSelectionFrom(ctx)
	rng := corestats.ResolveRange(parseDays(sel.Query), h.now())
	h.D.PersistCrossRangeSelection(w, r, spanEnds(rng.Days))
	return crossPage{
		Range: rng,
		Picker: web.RangePickerFor(ctx, web.RangePickerProps{
			Action:       r.URL.Path,
			Label:        rangeLabel(ctx, rng),
			SelectedDays: dayKeys(rng.Days),
			Explicit:     sel.Explicit,
			Hidden:       hidden,
		}),
	}
}

func (h *Handlers) crossNav(r *http.Request, active string) web.Nav {
	n := h.D.Nav(r, "")
	n.Active = active
	return n
}
