package spotshttp

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
)

const tabArchived = "archived"

// pageView is everything the page and its region render.
type pageView struct {
	GuildID   string
	GuildName string
	CanManage bool
	Filter    spot.ListFilter
	List      *spot.List
	Flash     flash

	CreateName  string
	CreateError string
	// FocusCreate puts the cursor back in the add field after an add.
	FocusCreate bool

	// RenameID is the spot whose rename was refused; its editor renders open.
	RenameID    int64
	RenameValue string
	RenameError string
}

type flash struct {
	Text string
	Warn bool
}

func okFlash(text string) flash   { return flash{Text: text} }
func warnFlash(text string) flash { return flash{Text: text, Warn: true} }

func filterFrom(tab, query string) spot.ListFilter {
	return spot.ListFilter{Archived: tab == tabArchived, Query: query}
}

// viewFromForm keeps the tab and search of the page a change came from, so the
// refreshed list shows the same slice.
func viewFromForm(r *http.Request) pageView {
	return pageView{Filter: filterFrom(r.PostFormValue("tab"), r.PostFormValue("q"))}
}

func tabValue(f spot.ListFilter) string {
	if f.Archived {
		return tabArchived
	}
	return ""
}

func listURL(guildID string, f spot.ListFilter) string {
	return tabURL(guildID, f.Archived, f.Query)
}

func tabURL(guildID string, archived bool, query string) string {
	v := url.Values{}
	if archived {
		v.Set("tab", tabArchived)
	}
	if query != "" {
		v.Set("q", query)
	}
	path := web.GuildPath(guildID, "/spots")
	if len(v) == 0 {
		return path
	}
	return path + "?" + v.Encode()
}

func spotPath(guildID string, id int64, action string) string {
	return web.GuildPath(guildID, "/spots/"+strconv.FormatInt(id, 10)+"/"+action)
}

func spotDOMID(id int64) string { return "spot-" + strconv.FormatInt(id, 10) }

func formatCount(n int64) string { return strconv.FormatInt(n, 10) }

// removeConfirm is the question before a remove. It states the outcome, because
// a spot with reservations is archived, not deleted.
func removeConfirm(ctx context.Context, s spot.Listed) string {
	if s.Reservations.Total == 0 {
		return i18n.T(ctx, "spots.remove.confirm_delete", s.Name)
	}
	msg := i18n.N(ctx, "spots.remove.confirm_archive", int(s.Reservations.Total), s.Name)
	if s.Reservations.Upcoming > 0 {
		msg += " " + i18n.N(ctx, "spots.remove.confirm_upcoming", int(s.Reservations.Upcoming))
	}
	return msg
}

func (v pageView) emptyText(ctx context.Context) string {
	switch {
	case v.Filter.Query != "":
		return i18n.T(ctx, "spots.list.no_match", v.Filter.Query)
	case v.Filter.Archived:
		return i18n.T(ctx, "spots.list.no_archived")
	}
	return i18n.T(ctx, "spots.list.no_active")
}

// showImport offers the default list only to a guild without a single spot.
func (v pageView) showImport() bool {
	return v.CanManage && v.List != nil && v.List.Total() == 0
}

func (v pageView) renameOpen(id int64) bool { return v.RenameID == id && v.RenameError != "" }

func (v pageView) renameValue(s spot.Listed) string {
	if v.renameOpen(s.ID) {
		return v.RenameValue
	}
	return s.Name
}
