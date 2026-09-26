package reservationshttp

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"time"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
)

const (
	regionID   = "reservations-region"
	statusID   = "reservations-status"
	editBodyID = "reservation-edit-body"
	createID   = "reservation-new"
	editID     = "reservation-edit"
	// savedEvent closes the dialog (modal.js) and reloads the region.
	savedEvent = "reservation-saved"

	// inputTime is the value format of the datetime fields (flatpickr's dateFormat).
	inputTime = "2006-01-02T15:04"
	// altInputTime is what a reader types into the visible field without JavaScript.
	altInputTime = "02/01/2006 15:04"
	inputDate    = "2006-01-02"
)

// filterForm is the filter bar as the query string carries it, so the page and
// its links can re-render it verbatim.
type filterForm struct {
	Spot   string
	Author string
	From   string
	To     string
	Scope  string
	Mine   bool
	Page   int
}

func filterFromQuery(q url.Values) filterForm {
	page, _ := strconv.Atoi(q.Get("page"))
	scope := q.Get("scope")
	switch reservation.SearchScope(scope) {
	case reservation.ScopeAll, reservation.ScopePast:
	default:
		scope = string(reservation.ScopeUpcoming)
	}
	return filterForm{
		Spot:   q.Get("spot"),
		Author: strings.TrimSpace(q.Get("author")),
		From:   validDate(q.Get("from")),
		To:     validDate(q.Get("to")),
		Scope:  scope,
		Mine:   q.Get("mine") == "1",
		Page:   page,
	}
}

func validDate(v string) string {
	if _, err := time.ParseInLocation(inputDate, v, time.Local); err != nil {
		return ""
	}
	return v
}

// search turns the filter bar into a core filter. From and To are whole local days.
func (f filterForm) search(guildID, userID string) reservation.SearchFilter {
	out := reservation.SearchFilter{GuildID: guildID, Author: f.Author, Scope: reservation.SearchScope(f.Scope)}
	if id, err := strconv.ParseInt(f.Spot, 10, 64); err == nil {
		out.SpotID = &id
	}
	if f.Mine {
		out.AuthorDiscordID = userID
	}
	if d, err := time.ParseInLocation(inputDate, f.From, time.Local); err == nil {
		out.From = &d
	}
	if d, err := time.ParseInLocation(inputDate, f.To, time.Local); err == nil {
		end := d.AddDate(0, 0, 1).Add(-time.Nanosecond)
		out.To = &end
	}
	return out
}

func (f filterForm) values(withPage bool) url.Values {
	v := url.Values{}
	set := func(k, val string) {
		if val != "" {
			v.Set(k, val)
		}
	}
	set("spot", f.Spot)
	set("author", f.Author)
	set("from", f.From)
	set("to", f.To)
	if f.Scope != string(reservation.ScopeUpcoming) {
		set("scope", f.Scope)
	}
	if f.Mine {
		v.Set("mine", "1")
	}
	if withPage && f.Page > 1 {
		v.Set("page", strconv.Itoa(f.Page))
	}
	return v
}

func (f filterForm) active() bool {
	return f.Spot != "" || f.Author != "" || f.From != "" || f.To != "" || f.Mine || f.Scope != string(reservation.ScopeUpcoming)
}

// listView is what the page and its region render.
type listView struct {
	GuildID   string
	GuildName string
	Actor     reservation.Actor
	Filter    filterForm
	Page      *reservation.Page
	Now       time.Time
	// Spot is set on a respawn's own page, which fixes the respawn filter.
	Spot *spot.Listed
	// FilterSpots are the respawn filter options (active, then archived).
	FilterSpots []web.ComboboxOption
	Create      *formView
}

func (v listView) basePath() string {
	if v.Spot != nil {
		return web.GuildPath(v.GuildID, "/spots/"+strconv.FormatInt(v.Spot.ID, 10))
	}
	return web.GuildPath(v.GuildID, "/reservations")
}

func (v listView) url(f filterForm, withPage bool) string {
	q := f.values(withPage)
	if v.Spot != nil {
		q.Del("spot")
	}
	if len(q) == 0 {
		return v.basePath()
	}
	return v.basePath() + "?" + q.Encode()
}

func (v listView) currentURL() string { return v.url(v.Filter, true) }

func (v listView) pageURL(page int) string {
	f := v.Filter
	f.Page = page
	return v.url(f, true)
}

func (v listView) clearURL() string { return v.basePath() }

func (v listView) canEdit(r *reservation.ReservationWithSpot) bool {
	return reservations.CanEdit(v.Actor, r.Reservation, v.Now)
}

func (v listView) canDelete(r *reservation.ReservationWithSpot) bool {
	return reservations.CanDelete(v.Actor, r.Reservation, v.Now)
}

func (v listView) isMine(r *reservation.ReservationWithSpot) bool {
	return v.Actor.UserID != "" && r.AuthorDiscordID == v.Actor.UserID
}

func (v listView) hasActions() bool { return v.Actor.Caps.Reserve || v.Actor.Caps.Manage }

// state is the time status chip of a row.
func (v listView) state(r *reservation.ReservationWithSpot) string {
	switch {
	case !r.EndAt.After(v.Now):
		return "past"
	case !r.StartAt.After(v.Now):
		return "ongoing"
	}
	return "upcoming"
}

func (v listView) emptyText(ctx context.Context) string {
	if v.Filter.active() {
		return i18n.T(ctx, "reservations.list.no_match")
	}
	return i18n.T(ctx, "reservations.list.none_upcoming")
}

func (v listView) pageInfo(ctx context.Context) string {
	if v.Page.Total == 0 {
		return ""
	}
	first := int64((v.Page.Page-1)*v.Page.PerPage) + 1
	last := first + int64(len(v.Page.Items)) - 1
	return i18n.T(ctx, "reservations.list.range", first, last, v.Page.Total)
}

// formView is the create or edit dialog form.
type formView struct {
	GuildID string
	// ID is the edited reservation, 0 for a new one.
	ID       int64
	SpotID   string
	Start    string
	End      string
	Author   string
	AuthorID string
	Overbook bool
	// SelfName is who a member without the manage right books as.
	SelfName      string
	CanPickAuthor bool
	CanOverbook   bool
	Spots         []web.ComboboxOption
	Errors        map[string]string
	General       string
	Conflicts     []*reservation.Reservation
}

func (f *formView) prefix() string {
	if f.ID == 0 {
		return createID
	}
	return editID
}

func (f *formView) action() string {
	if f.ID == 0 {
		return web.GuildPath(f.GuildID, "/reservations")
	}
	return web.GuildPath(f.GuildID, "/reservations/"+strconv.FormatInt(f.ID, 10)+"/edit")
}

// zoneName is the abbreviation of the process time zone (D31) at the form's start,
// so a reservation past a DST change names the offset it is booked in.
func (f *formView) zoneName(now time.Time) string {
	t, ok := parseInput(f.Start)
	if !ok {
		t = now
	}
	return t.In(time.Local).Format("MST")
}

func (f *formView) fieldID(name string) string { return f.prefix() + "-" + name }

func (f *formView) err(name string) string { return f.Errors[name] }

func (f *formView) setErr(name, msg string) {
	if f.Errors == nil {
		f.Errors = map[string]string{}
	}
	f.Errors[name] = msg
}

// describedBy points a field at its hint and, when refused, its error.
func (f *formView) describedBy(name string, hint bool) string {
	ids := []string{}
	if hint {
		ids = append(ids, f.fieldID(name)+"-hint")
	}
	if f.err(name) != "" {
		ids = append(ids, f.fieldID(name)+"-error")
	}
	return strings.Join(ids, " ")
}

func reservationPath(guildID string, id int64, action string) string {
	return web.GuildPath(guildID, "/reservations/"+strconv.FormatInt(id, 10)+"/"+action)
}

func rowID(id int64) string { return "reservation-" + strconv.FormatInt(id, 10) }

func formatInput(t time.Time) string { return t.In(time.Local).Format(inputTime) }

// parseInput reads a datetime field in the process time zone (D31).
func parseInput(v string) (time.Time, bool) {
	v = strings.TrimSpace(v)
	for _, layout := range []string{inputTime, altInputTime} {
		if t, err := time.ParseInLocation(layout, v, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// defaultWindow is the next half hour, for two hours.
func defaultWindow(now time.Time) (time.Time, time.Time) {
	start := now.In(time.Local).Truncate(30 * time.Minute).Add(30 * time.Minute)
	return start, start.Add(2 * time.Hour)
}

func clock(t time.Time) string { return t.In(time.Local).Format("15:04") }

// when is the row's time range: the start day, then the clock range, with a +1
// when the reservation ends the next day.
func when(ctx context.Context, r *reservation.ReservationWithSpot) (day, hours string) {
	start, end := r.StartAt.In(time.Local), r.EndAt.In(time.Local)
	hours = clock(start) + "–" + clock(end)
	if end.YearDay() != start.YearDay() || end.Year() != start.Year() {
		hours += " +1"
	}
	return i18n.Date(ctx, i18n.WeekdayDayMonth, start), hours
}

func duration(r *reservation.ReservationWithSpot) string {
	d := r.EndAt.Sub(r.StartAt).Round(time.Minute)
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case h == 0:
		return strconv.Itoa(m) + " min"
	case m == 0:
		return strconv.Itoa(h) + " h"
	}
	return strconv.Itoa(h) + " h " + strconv.Itoa(m) + " min"
}

func conflictLine(ctx context.Context, r *reservation.Reservation) string {
	return r.Author + " · " + i18n.Date(ctx, i18n.DayMonthTime, r.StartAt.In(time.Local)) + "–" + clock(r.EndAt)
}

func spotOptions(ctx context.Context, spots []*spot.Spot, placeholder string, includeArchived bool, keepID int64) []web.ComboboxOption {
	opts := []web.ComboboxOption{{Value: "", Label: placeholder}}
	var archived []web.ComboboxOption
	for _, s := range spots {
		o := web.ComboboxOption{Value: strconv.FormatInt(s.ID, 10), Label: s.Name}
		switch {
		case !s.IsArchived():
			opts = append(opts, o)
		case includeArchived || s.ID == keepID:
			o.Group = i18n.T(ctx, "reservations.filter.archived_group")
			archived = append(archived, o)
		}
	}
	return append(opts, archived...)
}
