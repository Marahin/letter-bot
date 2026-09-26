package reservationshttp

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/ports"
)

type Handlers struct {
	D   *web.Deps
	now func() time.Time
}

func New(d *web.Deps) *Handlers { return &Handlers{D: d, now: time.Now} }

type flash struct {
	Text string
	Warn bool
}

func (h *Handlers) HandleList(w http.ResponseWriter, r *http.Request) {
	h.list(w, r, nil)
}

// HandleSpot is a respawn's own page: its reservations and counts.
func (h *Handlers) HandleSpot(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "spot")
	if !ok {
		return
	}
	overview, err := h.D.Reservations.SpotOverview(r.Context(), r.PathValue("id"), id)
	if errors.Is(err, ports.ErrNotFound) {
		h.D.NotFound(w, r)
		return
	}
	if err != nil {
		h.D.ServerError(w, r, "load spot overview", err)
		return
	}
	h.list(w, r, overview)
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request, sp *spot.Listed) {
	ctx := r.Context()
	current, ok := h.access(w, r)
	if !ok {
		return
	}
	guildID := current.Config.GuildID
	actor := h.actor(ctx, current, false)
	f := filterFromQuery(r.URL.Query())
	if sp != nil {
		f.Spot = ""
	}
	search := f.search(guildID, actor.UserID)
	if sp != nil {
		search.SpotID = &sp.ID
	}
	page, err := h.D.Reservations.Search(ctx, search, f.Page)
	if err != nil {
		h.D.ServerError(w, r, "search reservations", err)
		return
	}
	f.Page = page.Page
	v := listView{GuildID: guildID, GuildName: current.Config.Name, Actor: actor, Filter: f, Page: page, Now: h.now(), Spot: sp}

	if isHTMX(r) && r.Header.Get("HX-Target") == regionID {
		// A reload after a change keeps the address; a filter or page change updates it.
		if r.Header.Get("HX-Trigger") != regionID {
			w.Header().Set("HX-Push-Url", v.currentURL())
		}
		h.D.Render(w, r, Region(v))
		return
	}

	spots, err := h.D.Reservations.Spots(ctx, guildID)
	if err != nil {
		h.D.ServerError(w, r, "list spots", err)
		return
	}
	v.FilterSpots = spotOptions(ctx, spots, i18n.T(ctx, "reservations.filter.all_spots"), true, 0)
	if actor.Caps.Reserve {
		v.Actor = h.actor(ctx, current, true)
		preselect := ""
		if sp != nil && !sp.IsArchived() {
			preselect = strconv.FormatInt(sp.ID, 10)
		}
		v.Create = h.newForm(ctx, current, v.Actor, spots, preselect)
	}
	nav := h.D.Nav(r, guildID)
	nav.Active = "reservations"
	if sp != nil {
		nav.Active = "spots"
	}
	nav.ReturnTo = v.currentURL()
	h.D.Render(w, r, Page(h.D.Cfg.BaseURL, v, nav))
}

func (h *Handlers) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if !web.ParseForm(w, r) {
		return
	}
	ctx := r.Context()
	current, ok := h.access(w, r)
	if !ok {
		return
	}
	guildID := current.Config.GuildID
	actor := h.actor(ctx, current, true)
	spots, err := h.D.Reservations.Spots(ctx, guildID)
	if err != nil {
		h.D.ServerError(w, r, "list spots", err)
		return
	}
	f := h.newForm(ctx, current, actor, spots, "")
	draft, valid := readDraft(r, f)
	if !valid {
		h.renderForm(w, r, current, f)
		return
	}
	res, err := h.D.Reservations.Create(ctx, guildID, actor, draft)
	if err != nil {
		conflicts := make([]*reservation.Reservation, 0, len(res))
		for _, c := range res {
			conflicts = append(conflicts, c.Original)
		}
		if !h.refusal(ctx, f, err, conflicts) {
			h.D.ServerError(w, r, "create reservation", err)
			return
		}
		h.renderForm(w, r, current, f)
		return
	}
	msg := i18n.T(ctx, "reservations.flash.created", spotName(spots, draft.SpotID), dayAndHours(ctx, draft.StartAt, draft.EndAt))
	if len(res) > 0 {
		msg += " " + i18n.N(ctx, "reservations.flash.overbooked", len(res))
	}
	h.saved(w, r, guildID, h.newForm(ctx, current, actor, spots, ""), flash{Text: msg})
}

func (h *Handlers) HandleEditForm(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "reservation")
	if !ok {
		return
	}
	ctx := r.Context()
	current, ok := h.access(w, r)
	if !ok {
		return
	}
	existing, err := h.D.Reservations.Get(ctx, current.Config.GuildID, id)
	if errors.Is(err, reservations.ErrNotFound) {
		h.D.NotFound(w, r)
		return
	}
	if err != nil {
		h.D.ServerError(w, r, "load reservation", err)
		return
	}
	actor := h.actor(ctx, current, false)
	if !reservations.CanEdit(actor, existing.Reservation, h.now()) {
		h.D.Forbidden(w, r)
		return
	}
	f, ok := h.editForm(w, r, current, actor, existing)
	if !ok {
		return
	}
	h.renderForm(w, r, current, f)
}

func (h *Handlers) HandleEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "reservation")
	if !ok || !web.ParseForm(w, r) {
		return
	}
	ctx := r.Context()
	current, ok := h.access(w, r)
	if !ok {
		return
	}
	guildID := current.Config.GuildID
	actor := h.actor(ctx, current, false)
	existing, err := h.D.Reservations.Get(ctx, guildID, id)
	if errors.Is(err, reservations.ErrNotFound) {
		h.saved(w, r, guildID, nil, flash{Text: i18n.T(ctx, "reservations.flash.not_found"), Warn: true})
		return
	}
	if err != nil {
		h.D.ServerError(w, r, "load reservation", err)
		return
	}
	f, ok := h.editForm(w, r, current, actor, existing)
	if !ok {
		return
	}
	draft, valid := readDraft(r, f)
	if !valid {
		h.renderForm(w, r, current, f)
		return
	}
	conflicts, err := h.D.Reservations.Edit(ctx, guildID, actor, id, draft)
	if err != nil {
		if !h.refusal(ctx, f, err, conflicts) {
			h.D.ServerError(w, r, "edit reservation", err)
			return
		}
		h.renderForm(w, r, current, f)
		return
	}
	h.saved(w, r, guildID, nil, flash{Text: i18n.T(ctx, "reservations.flash.saved", dayAndHours(ctx, draft.StartAt, draft.EndAt))})
}

func (h *Handlers) HandleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "reservation")
	if !ok {
		return
	}
	ctx := r.Context()
	current, ok := h.access(w, r)
	if !ok {
		return
	}
	guildID := current.Config.GuildID
	err := h.D.Reservations.Delete(ctx, guildID, h.actor(ctx, current, false), id)
	var f flash
	switch {
	case errors.Is(err, reservations.ErrNotFound):
		f = flash{Text: i18n.T(ctx, "reservations.flash.not_found"), Warn: true}
	case errors.Is(err, reservations.ErrForbidden):
		f = flash{Text: i18n.T(ctx, "reservations.flash.delete_forbidden"), Warn: true}
	case err != nil:
		h.D.ServerError(w, r, "delete reservation", err)
		return
	default:
		f = flash{Text: i18n.T(ctx, "reservations.flash.deleted")}
	}
	if !isHTMX(r) {
		http.Redirect(w, r, web.GuildPath(guildID, "/reservations"), http.StatusSeeOther)
		return
	}
	w.Header().Set("HX-Trigger", savedEvent)
	h.D.Render(w, r, Status(f, false))
}

// HandleAuthors suggests past authors while a manager types an author.
func (h *Handlers) HandleAuthors(w http.ResponseWriter, r *http.Request) {
	authors, err := h.D.Reservations.KnownAuthors(r.Context(), r.PathValue("id"), r.URL.Query().Get("author"))
	if err != nil {
		h.D.ServerError(w, r, "known authors", err)
		return
	}
	h.D.Render(w, r, AuthorSuggestions(authors))
}

// saved answers a finished change: htmx gets the next form and the status line,
// and the event that closes the dialog and reloads the list; a plain form post a
// redirect back to the list.
func (h *Handlers) saved(w http.ResponseWriter, r *http.Request, guildID string, next *formView, f flash) {
	if !isHTMX(r) {
		http.Redirect(w, r, web.GuildPath(guildID, "/reservations"), http.StatusSeeOther)
		return
	}
	w.Header().Set("HX-Trigger", savedEvent)
	h.D.Render(w, r, Saved(next, f))
}

// renderForm answers a form: the fragment for htmx (a refusal also answers 200,
// as htmx does not swap a 4xx body), the form page otherwise.
func (h *Handlers) renderForm(w http.ResponseWriter, r *http.Request, current access.GuildAccess, f *formView) {
	if isHTMX(r) {
		h.D.Render(w, r, Form(f))
		return
	}
	nav := h.D.Nav(r, current.Config.GuildID)
	nav.Active = "reservations"
	h.D.Render(w, r, FormPage(h.D.Cfg.BaseURL, current.Config.Name, f, nav))
}

func (h *Handlers) access(w http.ResponseWriter, r *http.Request) (access.GuildAccess, bool) {
	current, ok := web.CurrentAccessFrom(r.Context())
	if !ok {
		h.D.ServerError(w, r, "reservations without a guild guard", errors.New("no guild access in context"))
	}
	return current, ok
}

// actor is the signed-in user in this guild. withName also resolves the author
// text of their own bookings, which costs a (cached) Discord member lookup.
func (h *Handlers) actor(ctx context.Context, current access.GuildAccess, withName bool) reservation.Actor {
	a := reservation.Actor{UserID: h.D.SessionUserID(ctx), Caps: current.Caps}
	if !withName {
		return a
	}
	if m, err := h.D.Access.Member(ctx, a.UserID, current.Config.GuildID); err == nil && m.DisplayName() != "" {
		a.Name = m.DisplayName()
		return a
	}
	if u, err := h.D.CurrentUser(ctx); err == nil {
		a.Name = u.DisplayName()
	}
	return a
}

func (h *Handlers) newForm(ctx context.Context, current access.GuildAccess, actor reservation.Actor, spots []*spot.Spot, spotID string) *formView {
	start, end := defaultWindow(h.now())
	return &formView{
		GuildID:       current.Config.GuildID,
		SpotID:        spotID,
		Start:         formatInput(start),
		End:           formatInput(end),
		Author:        actor.Name,
		AuthorID:      actor.UserID,
		SelfName:      actor.Name,
		CanPickAuthor: actor.Caps.Manage,
		CanOverbook:   actor.Caps.Overbook,
		Spots:         spotOptions(ctx, spots, i18n.T(ctx, "reservations.form.spot_placeholder"), false, 0),
	}
}

func (h *Handlers) editForm(w http.ResponseWriter, r *http.Request, current access.GuildAccess, actor reservation.Actor, existing *reservation.ReservationWithSpot) (*formView, bool) {
	ctx := r.Context()
	spots, err := h.D.Reservations.Spots(ctx, current.Config.GuildID)
	if err != nil {
		h.D.ServerError(w, r, "list spots", err)
		return nil, false
	}
	return &formView{
		GuildID:       current.Config.GuildID,
		ID:            existing.Reservation.ID,
		SpotID:        strconv.FormatInt(existing.SpotID, 10),
		Start:         formatInput(existing.StartAt),
		End:           formatInput(existing.EndAt),
		Author:        existing.Author,
		AuthorID:      existing.AuthorDiscordID,
		SelfName:      existing.Author,
		CanPickAuthor: actor.Caps.Manage,
		Spots:         spotOptions(ctx, spots, i18n.T(ctx, "reservations.form.spot_placeholder"), false, existing.SpotID),
	}, true
}

// readDraft copies the posted fields into the form and parses them. It reports
// false with the field errors set when a field cannot be read.
func readDraft(r *http.Request, f *formView) (reservation.Draft, bool) {
	ctx := r.Context()
	f.SpotID = r.PostFormValue("spot_id")
	f.Start, f.End = r.PostFormValue("start"), r.PostFormValue("end")
	f.Overbook = r.PostFormValue("overbook") == "1"
	if f.CanPickAuthor {
		f.Author, f.AuthorID = r.PostFormValue("author"), r.PostFormValue("author_discord_id")
	}
	d := reservation.Draft{Author: f.Author, AuthorDiscordID: f.AuthorID, Overbook: f.Overbook}
	id, err := strconv.ParseInt(f.SpotID, 10, 64)
	if err != nil || id <= 0 {
		f.setErr("spot", i18n.T(ctx, "reservations.form.error_spot"))
	}
	d.SpotID = id
	var ok bool
	if d.StartAt, ok = parseInput(f.Start); !ok {
		f.setErr("start", i18n.T(ctx, "reservations.form.error_time"))
	}
	if d.EndAt, ok = parseInput(f.End); !ok {
		f.setErr("end", i18n.T(ctx, "reservations.form.error_time"))
	}
	return d, len(f.Errors) == 0
}

// refusal puts a refused change on the form. It reports false for an error that
// is not a refusal (a failure to report as a server error).
func (h *Handlers) refusal(ctx context.Context, f *formView, err error, conflicts []*reservation.Reservation) bool {
	switch {
	case errors.Is(err, booking.ErrInvalidRange):
		f.setErr("end", i18n.T(ctx, "reservations.error.range"))
	case errors.Is(err, booking.ErrStartInPast):
		f.setErr("start", i18n.T(ctx, "reservations.error.past"))
	case errors.Is(err, booking.ErrReservationTooLong):
		f.setErr("end", i18n.T(ctx, "reservations.error.too_long", int(booking.MaximumReservationLength.Hours())))
	case errors.Is(err, booking.ErrSpotNotFound), errors.Is(err, booking.ErrSpotArchived):
		f.setErr("spot", i18n.T(ctx, "reservations.error.spot"))
	case errors.Is(err, reservations.ErrAuthorTooLong):
		f.setErr("author", i18n.T(ctx, "reservations.error.author_too_long", reservations.MaxAuthorLength))
	case errors.Is(err, booking.ErrQuotaExceeded):
		f.General = i18n.T(ctx, "reservations.error.quota", int(booking.MaximumReservationLength.Hours()))
	case errors.Is(err, booking.ErrSelfOverbook):
		f.General = i18n.T(ctx, "reservations.error.self_overbook")
	case errors.Is(err, booking.ErrInsufficientPermissions):
		f.General, f.Conflicts = i18n.T(ctx, "reservations.error.taken_ask"), conflicts
		if f.CanOverbook {
			f.General = i18n.T(ctx, "reservations.error.taken_overbook")
		}
	case errors.Is(err, booking.ErrConflict):
		f.General, f.Conflicts = i18n.T(ctx, "reservations.error.conflict"), conflicts
	case errors.Is(err, booking.ErrReservationEnded):
		f.General = i18n.T(ctx, "reservations.error.ended")
	case errors.Is(err, reservations.ErrForbidden):
		f.General = i18n.T(ctx, "reservations.error.forbidden")
	case errors.Is(err, reservations.ErrNotFound):
		f.General = i18n.T(ctx, "reservations.flash.not_found")
	default:
		return false
	}
	return true
}

func spotName(spots []*spot.Spot, id int64) string {
	for _, s := range spots {
		if s.ID == id {
			return s.Name
		}
	}
	return ""
}

func dayAndHours(ctx context.Context, start, end time.Time) string {
	day, hours := when(ctx, &reservation.ReservationWithSpot{Reservation: reservation.Reservation{StartAt: start, EndAt: end}})
	return day + " " + hours
}

func pageTitle(ctx context.Context, v listView) string {
	if v.Spot != nil {
		return i18n.T(ctx, "reservations.spot.title", v.Spot.Name, v.GuildName)
	}
	return i18n.T(ctx, "reservations.page.title", v.GuildName)
}

func deleteConfirm(ctx context.Context, r *reservation.ReservationWithSpot) string {
	return i18n.T(ctx, "reservations.row.delete_confirm", r.Author, r.Spot.Name, dayAndHours(ctx, r.StartAt, r.EndAt))
}

// zoneName is the abbreviation of the process time zone (D31), for the form hint.
func zoneName() string { return time.Now().Format("MST") }

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }
