package statshttp

import (
	"context"
	"net/url"
	"strings"

	"spot-assistant/internal/core/dto/stats"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
)

// crossPage is what every cross-server page shares: the range and its picker.
type crossPage struct {
	Range  stats.Range
	Picker web.RangePickerProps
}

type worldsView struct {
	crossPage

	Rows []stats.WorldRow
}

type worldView struct {
	crossPage

	D *stats.WorldDetail
}

type spawnsView struct {
	crossPage

	Worlds []stats.World
	// World is the selected world key, "" for every world.
	World string
	Sort  stats.Sort
	Rows  []tableRow
	Total int
}

type spawnView struct {
	crossPage

	D *stats.SpawnDetail
}

func worldHref(name string) string {
	return web.WorldsPath + "/" + url.PathEscape(strings.TrimSpace(name))
}

func spawnHref(key string) string {
	return web.SpawnsPath + "/" + url.PathEscape(key)
}

func spawnRows(rows []stats.SpawnRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{Name: r.Name, Href: spawnHref(r.Key), Worlds: r.Worlds, Totals: r.Totals}
	}
	return out
}

func worldGuildRows(rows []stats.WorldGuildRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{Name: r.Name, Href: web.GuildPath(r.GuildID, "/stats"), Badge: true, IconURL: web.GuildIconURL(r.GuildID, r.Icon), Totals: r.Totals}
	}
	return out
}

func worldRows(rows []stats.WorldRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{Name: r.Name, Href: worldHref(r.Name), Totals: r.Totals}
	}
	return out
}

// spawnSpotRows are a spawn's respawns, each under its server and world, linking the server's
// respawn page.
func spawnSpotRows(rows []stats.SpawnSpotRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{
			Name: r.GuildName, Sub: r.World + " · " + strings.TrimSpace(r.SpotName), Href: spotHref(r.GuildID, r.SpotID),
			Archived: r.Archived, Totals: r.Totals,
		}
	}
	return out
}

func worldCharacterRows(rows []stats.WorldCharacterRow) []tableRow {
	out := make([]tableRow, len(rows))
	for i, r := range rows {
		out[i] = tableRow{Name: r.Name, Href: characterHref(r.GuildID, r.Name), Totals: r.Totals}
	}
	return out
}

func (v spawnsView) sortState() *sortState {
	s := &sortState{Path: web.SpawnsPath, Sort: v.Sort}
	if v.World != "" {
		s.Extra = url.Values{"world": {v.World}}
	}
	return s
}

func (v spawnsView) csvHref() string {
	q := url.Values{"format": {"csv"}, "sort": {string(v.Sort.Key)}, "dir": {sortDir(v.Sort)}}
	if v.World != "" {
		q.Set("world", v.World)
	}
	return web.SpawnsPath + "?" + q.Encode()
}

// spawnsMore is a world page's link to the spawn directory of that world, in board order.
func spawnsMore(world string, key stats.SortKey) string {
	return web.SpawnsPath + "?" + url.Values{"world": {world}, "sort": {string(key)}}.Encode()
}

func worldTitle(ctx context.Context, v spawnsView) string {
	for _, w := range v.Worlds {
		if w.Key == v.World {
			return i18n.T(ctx, "spawns.list.title_world", w.Name)
		}
	}
	return i18n.T(ctx, "spawns.list.title")
}
