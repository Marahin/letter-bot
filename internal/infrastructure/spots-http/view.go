package spotshttp

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/a-h/templ"

	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/regions"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
)

const tabArchived = "archived"

// spotNewID prefixes the element ids of the add form.
const spotNewID = "spot-new"

// pageView is everything the page and its region render.
type pageView struct {
	GuildID   string
	GuildName string
	CanManage bool
	Filter    spot.ListFilter
	List      *spot.List
	Flash     flash

	// Create holds the posted fields of a refused add.
	Create       spot.Fields
	CreateErrors fieldErrors
	// FocusCreate puts the cursor back in the add field after an add.
	FocusCreate bool

	// EditID is the spot whose edit was refused; its editor renders open with
	// the posted fields.
	EditID     int64
	Edit       spot.Fields
	EditErrors fieldErrors
}

// fieldErrors are the inline messages of a refused spot form.
type fieldErrors struct {
	Name   string
	Region string
	Image  string
}

func (e fieldErrors) any() bool { return e.Name != "" || e.Region != "" || e.Image != "" }

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

func (v pageView) editOpen(id int64) bool { return v.EditID == id && v.EditErrors.any() }

// editFields are what a spot's editor shows: the refused post, else the spot.
func (v pageView) editFields(s spot.Listed) spot.Fields {
	if v.editOpen(s.ID) {
		return v.Edit
	}
	return spot.Fields{Name: s.Name, Region: s.Region, ImageURL: s.ImageURL}
}

func (v pageView) editErrors(id int64) fieldErrors {
	if v.EditID == id {
		return v.EditErrors
	}
	return fieldErrors{}
}

// regionOptions are the catalog regions, after the choice of none (Other).
func regionOptions(ctx context.Context) []web.ComboboxOption {
	opts := make([]web.ComboboxOption, 0, len(regions.Catalog)+1)
	opts = append(opts, web.ComboboxOption{Value: "", Label: i18n.T(ctx, "spots.form.region_none")})
	for _, r := range regions.Catalog {
		opts = append(opts, web.ComboboxOption{Value: r.Key, Label: r.Name})
	}
	return opts
}

// regionName is the region column of a spot.
func regionName(ctx context.Context, key string) string {
	r := regions.Of(key)
	if r.Key == regions.Other.Key {
		return i18n.T(ctx, "spots.region.other")
	}
	return r.Name
}

// imageURL keeps a javascript: link out of the page, though the service accepts only http and https.
func imageURL(raw string) string { return string(templ.URL(raw)) }
