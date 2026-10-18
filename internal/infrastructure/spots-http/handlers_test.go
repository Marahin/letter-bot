package spotshttp

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/core/spots"
	"spot-assistant/internal/infrastructure/web/webtest"
)

const guildID = "g1"

func managerAccess() *access.GuildAccess {
	return &access.GuildAccess{
		Config: guildconfig.Config{GuildID: guildID, Name: "Celesta Community", Premium: true},
		Caps:   permission.Capabilities{Manage: true, View: true, Reserve: true, Overbook: true},
	}
}

func viewerAccess() *access.GuildAccess {
	return &access.GuildAccess{
		Config: guildconfig.Config{GuildID: guildID, Name: "Celesta Community", Premium: true},
		Caps:   permission.Capabilities{View: true},
	}
}

func signedIn(t *testing.T, a *access.GuildAccess) (http.Handler, webtest.Mocks, *http.Cookie) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(a, nil)
	return h, m, cookie
}

func htmx(r *http.Request) *http.Request {
	r.Header.Set("HX-Request", "true")
	return r
}

func activeList() *spot.List {
	created := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	return &spot.List{
		Spots: []spot.Listed{
			{Spot: spot.Spot{ID: 1, Name: "Dragon Lords", CreatedAt: created, Region: "darama", ImageURL: "https://img.test/dl.gif"}, Reservations: spot.ReservationCounts{Total: 12, Upcoming: 2}},
			{Spot: spot.Spot{ID: 7, Name: "New Spot", CreatedAt: created}},
		},
		ActiveCount:   2,
		ArchivedCount: 1,
	}
}

func TestHandleList_ManagerSeesControls(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"Respawns", `id="spots-region"`, `id="spots-search"`,
		"Dragon Lords", "New Spot", "02 Jan 2026",
		`hx-post="/servers/g1/spots"`,
		`hx-get="/servers/g1/spots/1/edit" hx-trigger="toggle once" hx-target="find [data-spot-editor]"`,
		`hx-post="/servers/g1/spots/1/remove"`,
		"Archive Dragon Lords? It has 12 reservations, so it is archived, not deleted",
		"Its 2 upcoming reservations stay.",
		"Delete New Spot? It has no reservations",
		`class="letter-btn-danger`, `class="letter-btn-quiet`,
		`href="/servers/g1/spots?tab=archived"`,
		`<img src="https://img.test/dl.gif" alt="" loading="lazy" referrerpolicy="no-referrer" class="h-8 w-8 shrink-0 rounded bg-zone-950 object-contain p-0.5">`,
		"Darama &amp; Krailos", "Other", `id="spot-new-region"`, `name="image_url"`,
		"/assets/alpine.js",
	} {
		assert.Contains(t, body, want)
	}
	assert.NotContains(t, body, "Import the default list")
	assert.NotContains(t, body, `id="spot-1-region"`, "a closed editor loads its form on open")
	assert.Equal(t, 1, strings.Count(body, `name="image_url"`), "only the add form carries the image field")
}

func TestHandleList_ViewerSeesNoControls(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerAccess())
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Dragon Lords")
	assert.NotContains(t, body, "/edit")
	assert.NotContains(t, body, "/remove")
	assert.NotContains(t, body, "New respawn")
}

func TestHandleList_ArchivedTabWithQuery(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	archivedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{Archived: true, Query: "emp"}).Return(&spot.List{
		Spots: []spot.Listed{
			{Spot: spot.Spot{ID: 3, Name: "Empty (old)", ArchivedAt: &archivedAt}, Reservations: spot.ReservationCounts{Total: 1}},
			{Spot: spot.Spot{ID: 5, Name: "Empty (unused)", ArchivedAt: &archivedAt}},
		},
		ActiveCount: 4, ArchivedCount: 2,
	}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots?tab=archived&q=emp", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `hx-post="/servers/g1/spots/3/restore"`)
	assert.NotContains(t, body, `hx-post="/servers/g1/spots/3/remove"`, "a spot with reservations cannot be deleted")
	assert.Contains(t, body, `hx-post="/servers/g1/spots/5/remove"`)
	assert.NotContains(t, body, `/spots/3/rename`)
	assert.Contains(t, body, "01 Sep 2026")
	assert.Contains(t, body, `name="tab" value="archived"`)
	assert.Contains(t, body, `name="q" value="emp"`)
}

func TestHandleList_EmptyGuildOffersImport(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(&spot.List{Spots: []spot.Listed{}}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Import the default list")
	assert.Contains(t, rec.Body.String(), `hx-post="/servers/g1/spots/import"`)
}

func TestHandleList_EmptyStates(t *testing.T) {
	cases := map[string]struct {
		target string
		filter spot.ListFilter
		want   string
	}{
		"no match":    {target: "/servers/g1/spots?q=zzz", filter: spot.ListFilter{Query: "zzz"}, want: "No respawns match &#34;zzz&#34;."},
		"no archived": {target: "/servers/g1/spots?tab=archived", filter: spot.ListFilter{Archived: true}, want: "No archived respawns."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, viewerAccess())
			m.Spots.EXPECT().List(mock.Anything, guildID, tc.filter).Return(&spot.List{Spots: []spot.Listed{}, ActiveCount: 3}, nil)

			// when
			rec := webtest.Serve(h, webtest.Get(tc.target, cookie))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestHandleList_ViewerOfEmptyGuildGetsNoImport(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerAccess())
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(&spot.List{Spots: []spot.Listed{}}, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "Import the default list")
	assert.Contains(t, rec.Body.String(), "No active respawns.")
}

func TestHandleList_SearchAnswersTheRegionOnly(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerAccess())
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{Query: "dra"}).Return(activeList(), nil)
	r := htmx(webtest.Get("/servers/g1/spots?q=dra", cookie))
	r.Header.Set("HX-Target", "spots-region")

	// when
	rec := webtest.Serve(h, r)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.True(t, strings.HasPrefix(body, `<div id="spots-region">`), body)
	assert.NotContains(t, body, "<html")
}

func TestHandleList_Errors(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerAccess())
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(nil, errors.New("db down"))

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots", cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleList_PremiumRequired(t *testing.T) {
	// given
	a := viewerAccess()
	a.Config.Premium = false
	h, _, cookie := signedIn(t, a)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots", cookie))

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleEditForm(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Get(mock.Anything, guildID, int64(1)).Return(&activeList().Spots[0].Spot, nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/spots/1/edit?tab=archived&q=dra", cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		`hx-post="/servers/g1/spots/1/edit"`, `hx-target="#spots-region"`,
		`id="spot-1-name"`, `value="Dragon Lords"`,
		`id="spot-1-region"`, `<input type="hidden" name="region" value="darama"`,
		`id="spot-1-image"`, `value="https://img.test/dl.gif"`,
		`<input type="hidden" name="tab" value="archived">`, `<input type="hidden" name="q" value="dra">`,
	} {
		assert.Contains(t, body, want)
	}
	assert.NotContains(t, body, "<html")
	assert.NotContains(t, body, "<details")
}

func TestHandleEditForm_Refusals(t *testing.T) {
	t.Run("unknown spot", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, managerAccess())
		m.Spots.EXPECT().Get(mock.Anything, guildID, int64(99)).Return(nil, spots.ErrNotFound)

		// when
		rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/spots/99/edit", cookie)))

		// then
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
	t.Run("bad id", func(t *testing.T) {
		// given
		h, _, cookie := signedIn(t, managerAccess())

		// when
		rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/spots/abc/edit", cookie)))

		// then
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
	t.Run("viewer", func(t *testing.T) {
		// given
		h, _, cookie := signedIn(t, viewerAccess())

		// when
		rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/spots/1/edit", cookie)))

		// then
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
	t.Run("service", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, managerAccess())
		m.Spots.EXPECT().Get(mock.Anything, guildID, int64(1)).Return(nil, errors.New("db down"))

		// when
		rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/spots/1/edit", cookie)))

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
	t.Run("without htmx", func(t *testing.T) {
		// given
		h, _, cookie := signedIn(t, managerAccess())

		// when
		rec := webtest.Serve(h, webtest.Get("/servers/g1/spots/1/edit?q=dra", cookie))

		// then
		assert.Equal(t, http.StatusSeeOther, rec.Code)
		assert.Equal(t, "/servers/g1/spots?q=dra", rec.Header().Get("Location"))
	})
}

func TestMutations_NeedManageTier(t *testing.T) {
	for _, target := range []string{
		"/servers/g1/spots", "/servers/g1/spots/import",
		"/servers/g1/spots/1/edit", "/servers/g1/spots/1/remove", "/servers/g1/spots/1/restore",
	} {
		t.Run(target, func(t *testing.T) {
			// given
			h, _, cookie := signedIn(t, viewerAccess())

			// when
			rec := webtest.Serve(h, webtest.Post(target, url.Values{"name": {"X"}}, cookie))

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
		})
	}
}

func TestHandleCreate_HTMXAnswersRegionWithFlash(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Create(mock.Anything, guildID, spot.Fields{Name: " Hero Cave ", Region: "edron", ImageURL: "https://img.test/hc.png"}).Return(&spot.Spot{ID: 12, Name: "Hero Cave"}, nil)
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{Query: "h"}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots", url.Values{"name": {" Hero Cave "}, "region": {"edron"}, "image_url": {"https://img.test/hc.png"}, "q": {"h"}}, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.True(t, strings.HasPrefix(body, `<div id="spots-region">`))
	assert.Contains(t, body, "Hero Cave added.")
	assert.Contains(t, body, "autofocus")
}

func TestHandleCreate_PlainPostRedirects(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Create(mock.Anything, guildID, spot.Fields{Name: "Hero Cave"}).Return(&spot.Spot{ID: 12, Name: "Hero Cave"}, nil)

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/spots", url.Values{"name": {"Hero Cave"}, "tab": {"archived"}}, cookie))

	// then
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/servers/g1/spots?tab=archived", rec.Header().Get("Location"))
}

func TestHandleCreate_RefusedFieldRendersInlineError(t *testing.T) {
	cases := map[string]struct {
		err     error
		want    string
		errorID string
	}{
		"empty":          {err: spots.ErrNameEmpty, want: "Enter a name.", errorID: "spot-new-name-error"},
		"too long":       {err: spots.ErrNameTooLong, want: "Use 120 characters or fewer.", errorID: "spot-new-name-error"},
		"duplicate":      {err: spots.ErrDuplicateName, want: "An active respawn already has the name Hero Cave.", errorID: "spot-new-name-error"},
		"region":         {err: spots.ErrUnknownRegion, want: "Select a region from the list.", errorID: "spot-new-region-error"},
		"image":          {err: spots.ErrImageURLInvalid, want: "Enter a link that starts with http:// or https://.", errorID: "spot-new-image-error"},
		"image too long": {err: spots.ErrImageURLTooLong, want: "Use 2,048 characters or fewer.", errorID: "spot-new-image-error"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, managerAccess())
			posted := spot.Fields{Name: "Hero Cave", Region: "zao", ImageURL: "ftp://img.test/a.png"}
			m.Spots.EXPECT().Create(mock.Anything, guildID, posted).Return(nil, tc.err)
			m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots", url.Values{
				"name": {"Hero Cave"}, "region": {"zao"}, "image_url": {"ftp://img.test/a.png"},
			}, cookie)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			body := rec.Body.String()
			assert.Contains(t, body, tc.want)
			assert.Contains(t, body, `id="`+tc.errorID+`"`)
			assert.Contains(t, body, `role="alert"`)
			assert.Contains(t, body, `value="Hero Cave"`)
			assert.Contains(t, body, `value="ftp://img.test/a.png"`)
			assert.Contains(t, body, `<input type="hidden" name="region" value="zao"`)
		})
	}
}

func TestHandleCreate_ServiceError(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Create(mock.Anything, guildID, spot.Fields{Name: "Hero Cave"}).Return(nil, errors.New("db down"))

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/spots", url.Values{"name": {"Hero Cave"}}, cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleUpdate(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Update(mock.Anything, guildID, int64(1), spot.Fields{Name: " Dragon Lords (north) ", Region: "darama", ImageURL: "https://img.test/dl.gif"}).Return(nil)
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots/1/edit", url.Values{
		"name": {" Dragon Lords (north) "}, "region": {"darama"}, "image_url": {"https://img.test/dl.gif"},
	}, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Dragon Lords (north) saved.")
}

func TestHandleUpdate_RefusedFieldOpensTheEditor(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Update(mock.Anything, guildID, int64(1), spot.Fields{Name: "New Spot", Region: "edron"}).Return(spots.ErrDuplicateName)
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots/1/edit", url.Values{"name": {"New Spot"}, "region": {"edron"}}, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "An active respawn already has the name New Spot.")
	assert.Contains(t, body, `<details class="group" open>`)
	assert.Contains(t, body, `aria-describedby="spot-1-name-error"`)
	assert.Contains(t, body, `<input type="hidden" name="region" value="edron"`)
	assert.Equal(t, 1, strings.Count(body, " open>"), "only the refused row opens")
	assert.Contains(t, body, `id="spot-1-region"`, "the refused editor renders its form inline")
	assert.Contains(t, body, `hx-get="/servers/g1/spots/7/edit"`, "the other rows stay lazy")
}

func TestHandleUpdate_NotFound(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Update(mock.Anything, guildID, int64(99), spot.Fields{Name: "X"}).Return(spots.ErrNotFound)
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots/99/edit", url.Values{"name": {"X"}}, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "This respawn does not exist any more.")
}

func TestHandleUpdate_Errors(t *testing.T) {
	t.Run("bad id", func(t *testing.T) {
		// given
		h, _, cookie := signedIn(t, managerAccess())

		// when
		rec := webtest.Serve(h, webtest.Post("/servers/g1/spots/abc/edit", url.Values{"name": {"X"}}, cookie))

		// then
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
	t.Run("service", func(t *testing.T) {
		// given
		h, m, cookie := signedIn(t, managerAccess())
		m.Spots.EXPECT().Update(mock.Anything, guildID, int64(1), spot.Fields{Name: "X"}).Return(errors.New("db down"))

		// when
		rec := webtest.Serve(h, webtest.Post("/servers/g1/spots/1/edit", url.Values{"name": {"X"}}, cookie))

		// then
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleRemove(t *testing.T) {
	cases := map[string]struct {
		outcome spot.RemoveOutcome
		err     error
		want    string
	}{
		"deleted":   {outcome: spot.RemoveDeleted, want: "Respawn deleted."},
		"archived":  {outcome: spot.RemoveArchived, want: "Respawn archived, because it has reservations."},
		"not found": {err: spots.ErrNotFound, want: "This respawn does not exist any more."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, managerAccess())
			m.Spots.EXPECT().Remove(mock.Anything, guildID, int64(7)).Return(tc.outcome, tc.err)
			m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots/7/remove", nil, cookie)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestHandleRemove_ServiceError(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Remove(mock.Anything, guildID, int64(7)).Return("", errors.New("db down"))

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/spots/7/remove", nil, cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleRestore(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"restored":  {want: "Respawn restored."},
		"clash":     {err: spots.ErrDuplicateName, want: "an active respawn has the same name"},
		"not found": {err: spots.ErrNotFound, want: "This respawn does not exist any more."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, managerAccess())
			m.Spots.EXPECT().Restore(mock.Anything, guildID, int64(3)).Return(tc.err)
			m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{Archived: true}).Return(activeList(), nil)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots/3/restore", url.Values{"tab": {"archived"}}, cookie)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.want)
		})
	}
}

func TestHandleRestore_ClashWithoutHTMXRendersThePage(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Restore(mock.Anything, guildID, int64(3)).Return(spots.ErrDuplicateName)
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{Archived: true}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/spots/3/restore", url.Values{"tab": {"archived"}}, cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html")
	assert.Contains(t, rec.Body.String(), "an active respawn has the same name")
}

func TestHandleRestore_ServiceError(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().Restore(mock.Anything, guildID, int64(3)).Return(errors.New("db down"))

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/spots/3/restore", nil, cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleImport(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().ImportDefaults(mock.Anything, guildID).Return(int64(208), nil)
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/spots/import", nil, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Imported 208 respawns.")
}

func TestHandleImport_ServiceError(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().ImportDefaults(mock.Anything, guildID).Return(int64(0), errors.New("db down"))

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/spots/import", nil, cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleList_PolishCopy(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerAccess())
	m.Spots.EXPECT().List(mock.Anything, guildID, spot.ListFilter{}).Return(activeList(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots?lang=pl", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Respy")
	assert.Contains(t, body, "Ma 12 rezerwacji")
	assert.Contains(t, body, "Jego 2 nadchodzące rezerwacje zostają.")
}
