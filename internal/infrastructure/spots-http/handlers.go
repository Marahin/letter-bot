package spotshttp

import (
	"errors"
	"net/http"
	"strings"

	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/spots"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
)

// regionID is the element every htmx request of the page swaps.
const regionID = "spots-region"

type Handlers struct {
	D *web.Deps
}

func New(d *web.Deps) *Handlers { return &Handlers{D: d} }

func (h *Handlers) HandleList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	h.respond(w, r, pageView{Filter: filterFrom(q.Get("tab"), q.Get("q"))})
}

func (h *Handlers) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if !web.ParseForm(w, r) {
		return
	}
	v := viewFromForm(r)
	created, err := h.D.Spots.Create(r.Context(), r.PathValue("id"), r.PostFormValue("name"))
	if msg := nameError(r, err); msg != "" {
		v.CreateName, v.CreateError = r.PostFormValue("name"), msg
		h.respond(w, r, v)
		return
	}
	if err != nil {
		h.D.ServerError(w, r, "create spot", err)
		return
	}
	v.Flash = okFlash(i18n.T(r.Context(), "spots.flash.created", created.Name))
	v.FocusCreate = true
	h.done(w, r, v)
}

func (h *Handlers) HandleRename(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "spot")
	if !ok || !web.ParseForm(w, r) {
		return
	}
	v := viewFromForm(r)
	name := strings.TrimSpace(r.PostFormValue("name"))
	err := h.D.Spots.Rename(r.Context(), r.PathValue("id"), id, name)
	if msg := nameError(r, err); msg != "" {
		v.RenameID, v.RenameValue, v.RenameError = id, r.PostFormValue("name"), msg
		h.respond(w, r, v)
		return
	}
	switch {
	case errors.Is(err, spots.ErrNotFound):
		v.Flash = warnFlash(i18n.T(r.Context(), "spots.flash.not_found"))
	case err != nil:
		h.D.ServerError(w, r, "rename spot", err)
		return
	default:
		v.Flash = okFlash(i18n.T(r.Context(), "spots.flash.renamed", name))
	}
	h.done(w, r, v)
}

func (h *Handlers) HandleRemove(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "spot")
	if !ok || !web.ParseForm(w, r) {
		return
	}
	v := viewFromForm(r)
	outcome, err := h.D.Spots.Remove(r.Context(), r.PathValue("id"), id)
	switch {
	case errors.Is(err, spots.ErrNotFound):
		v.Flash = warnFlash(i18n.T(r.Context(), "spots.flash.not_found"))
	case err != nil:
		h.D.ServerError(w, r, "remove spot", err)
		return
	case outcome == spot.RemoveArchived:
		v.Flash = okFlash(i18n.T(r.Context(), "spots.flash.archived"))
	default:
		v.Flash = okFlash(i18n.T(r.Context(), "spots.flash.deleted"))
	}
	h.done(w, r, v)
}

func (h *Handlers) HandleRestore(w http.ResponseWriter, r *http.Request) {
	id, ok := h.D.PathInt64(w, r, "spot")
	if !ok || !web.ParseForm(w, r) {
		return
	}
	v := viewFromForm(r)
	err := h.D.Spots.Restore(r.Context(), r.PathValue("id"), id)
	switch {
	case errors.Is(err, spots.ErrDuplicateName):
		v.Flash = warnFlash(i18n.T(r.Context(), "spots.flash.restore_clash"))
		h.respond(w, r, v)
		return
	case errors.Is(err, spots.ErrNotFound):
		v.Flash = warnFlash(i18n.T(r.Context(), "spots.flash.not_found"))
	case err != nil:
		h.D.ServerError(w, r, "restore spot", err)
		return
	default:
		v.Flash = okFlash(i18n.T(r.Context(), "spots.flash.restored"))
	}
	h.done(w, r, v)
}

func (h *Handlers) HandleImport(w http.ResponseWriter, r *http.Request) {
	if !web.ParseForm(w, r) {
		return
	}
	v := viewFromForm(r)
	added, err := h.D.Spots.ImportDefaults(r.Context(), r.PathValue("id"))
	if err != nil {
		h.D.ServerError(w, r, "import default spots", err)
		return
	}
	v.Flash = okFlash(i18n.N(r.Context(), "spots.flash.imported", int(added)))
	h.done(w, r, v)
}

// done answers a finished change: htmx gets the refreshed region with the flash,
// a plain form post a redirect back to the list it came from.
func (h *Handlers) done(w http.ResponseWriter, r *http.Request, v pageView) {
	if isHTMX(r) {
		h.respond(w, r, v)
		return
	}
	http.Redirect(w, r, listURL(r.PathValue("id"), v.Filter), http.StatusSeeOther)
}

// respond renders the region for htmx and the full page otherwise. A refusal
// also answers 200: htmx does not swap a 4xx body.
func (h *Handlers) respond(w http.ResponseWriter, r *http.Request, v pageView) {
	current, ok := web.CurrentAccessFrom(r.Context())
	if !ok {
		h.D.ServerError(w, r, "spots without a guild guard", errors.New("no guild access in context"))
		return
	}
	guildID := current.Config.GuildID
	list, err := h.D.Spots.List(r.Context(), guildID, v.Filter)
	if err != nil {
		h.D.ServerError(w, r, "list spots", err)
		return
	}
	v.GuildID, v.GuildName = guildID, current.Config.Name
	v.CanManage = current.Caps.Manage
	v.List = list

	if isHTMX(r) && (r.Method == http.MethodPost || r.Header.Get("HX-Target") == regionID) {
		h.D.Render(w, r, Region(v))
		return
	}
	nav := h.D.Nav(r, guildID)
	nav.Active = "spots"
	nav.ReturnTo = listURL(guildID, v.Filter)
	h.D.Render(w, r, Page(h.D.Cfg.BaseURL, v, nav))
}

// nameError is the inline message for a refused name, "" for any other outcome.
func nameError(r *http.Request, err error) string {
	ctx := r.Context()
	switch {
	case errors.Is(err, spots.ErrNameEmpty):
		return i18n.T(ctx, "spots.form.error_empty")
	case errors.Is(err, spots.ErrNameTooLong):
		return i18n.T(ctx, "spots.form.error_too_long", spots.MaxNameLength)
	case errors.Is(err, spots.ErrDuplicateName):
		return i18n.T(ctx, "spots.form.error_duplicate", strings.TrimSpace(r.PostFormValue("name")))
	}
	return ""
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }
