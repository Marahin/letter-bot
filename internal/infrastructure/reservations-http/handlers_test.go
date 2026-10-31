package reservationshttp

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

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/core/reservations"
	"spot-assistant/internal/infrastructure/web/webtest"
	"spot-assistant/internal/ports"
)

const guildID = "g1"

func accessWith(caps permission.Capabilities) *access.GuildAccess {
	return &access.GuildAccess{
		Config: guildconfig.Config{GuildID: guildID, Name: "Celesta Community", Premium: true},
		Caps:   caps,
	}
}

var (
	managerCaps = permission.Capabilities{Manage: true, View: true, Reserve: true, Overbook: true}
	memberCaps  = permission.Capabilities{View: true, Reserve: true}
	viewerCaps  = permission.Capabilities{View: true}
)

func signedIn(t *testing.T, caps permission.Capabilities) (http.Handler, webtest.Mocks, *http.Cookie) {
	t.Helper()
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(accessWith(caps), nil)
	m.Access.EXPECT().Member(mock.Anything, "u1", guildID).Return(&access.GuildMember{Nick: "Quiet Nyx"}, nil).Maybe()
	return h, m, cookie
}

func htmx(r *http.Request) *http.Request {
	r.Header.Set("HX-Request", "true")
	return r
}

func guildSpots() []*spot.Spot {
	archivedAt := time.Now().Add(-time.Hour)
	return []*spot.Spot{
		{ID: 1, Name: "Dragon Lords"},
		{ID: 3, Name: "Empty (old)", ArchivedAt: &archivedAt},
		{ID: 4, Name: "Hero Cave"},
	}
}

func item(id int64, authorID string, start time.Time) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: id, SpotID: 4, Author: "Author " + authorID, AuthorDiscordID: authorID, StartAt: start, EndAt: start.Add(2 * time.Hour)},
		Spot:        reservation.Spot{ID: 4, Name: "Hero Cave"},
	}
}

func onePage(items ...*reservation.ReservationWithSpot) *reservation.Page {
	return &reservation.Page{Items: items, Total: int64(len(items)), Page: 1, Pages: 1, PerPage: 50}
}

func expectList(m webtest.Mocks, page *reservation.Page) {
	m.Reservations.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything).Return(page, nil)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil).Maybe()
}

func TestHandleList_ManagerSeesEveryControl(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerCaps)
	soon := time.Now().Add(time.Hour)
	expectList(m, onePage(item(7, "u2", soon), item(8, "u1", soon.Add(3*time.Hour))))

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/reservations", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"Reservations", `id="reservations-region"`, `id="reservations-filter"`, `id="filter-spot"`,
		"All respawns", "Hero Cave", "Author u2",
		`hx-get="/servers/g1/reservations/7/edit"`, `hx-post="/servers/g1/reservations/7/delete"`,
		`data-modal-open="reservation-new-modal"`, `name="overbook"`, `data-author-input`,
		`value="Quiet Nyx"`, "Only mine", `/assets/reservations.js`,
		`hx-trigger="reservation-saved from:body"`,
	} {
		assert.Contains(t, body, want)
	}
}

func TestHandleList_MemberEditsOnlyOwnAndCannotOverbook(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	soon := time.Now().Add(time.Hour)
	expectList(m, onePage(item(7, "u2", soon), item(8, "u1", soon)))

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/reservations", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `hx-get="/servers/g1/reservations/8/edit"`)
	assert.NotContains(t, body, "/reservations/7/edit")
	assert.NotContains(t, body, "/reservations/7/delete")
	assert.NotContains(t, body, `name="overbook"`)
	assert.NotContains(t, body, "data-author-input")
	assert.Contains(t, body, "Reserved for")
	assert.Contains(t, body, "Quiet Nyx")
}

func TestHandleList_ViewerHasNoControls(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerCaps)
	expectList(m, onePage(item(7, "u1", time.Now().Add(time.Hour))))

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/reservations", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Hero Cave")
	assert.NotContains(t, body, "/edit")
	assert.NotContains(t, body, "/delete")
	assert.NotContains(t, body, "reservation-new-modal\"")
	assert.NotContains(t, body, "Only mine")
}

func TestHandleList_FilterToSearch(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	m.Reservations.EXPECT().Search(mock.Anything, mock.MatchedBy(func(f reservation.SearchFilter) bool {
		return f.GuildID == guildID && f.SpotID != nil && *f.SpotID == 4 && f.Author == "nyx" &&
			f.Scope == reservation.ScopePast && f.AuthorDiscordID == "u1" &&
			f.From != nil && f.From.Format(inputDate) == "2026-09-01" &&
			f.To != nil && f.To.Hour() == 23
	}), 2).Return(&reservation.Page{Items: []*reservation.ReservationWithSpot{}, Page: 1}, nil)

	// when
	req := htmx(webtest.Get("/servers/g1/reservations?spot=4&author=+nyx+&from=2026-09-01&to=2026-09-02&scope=past&mine=1&page=2", cookie))
	req.Header.Set("HX-Target", regionID)
	req.Header.Set("HX-Trigger", "reservations-filter")
	rec := webtest.Serve(h, req)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "No reservations match these filters.")
	assert.NotContains(t, body, "<html")
	assert.Equal(t, "/servers/g1/reservations?author=nyx&from=2026-09-01&mine=1&scope=past&spot=4&to=2026-09-02", rec.Header().Get("HX-Push-Url"))
}

func TestHandleList_ReloadKeepsTheAddress(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	expectList(m, onePage())

	// when
	req := htmx(webtest.Get("/servers/g1/reservations", cookie))
	req.Header.Set("HX-Target", regionID)
	req.Header.Set("HX-Trigger", regionID)
	rec := webtest.Serve(h, req)

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("HX-Push-Url"))
	assert.Contains(t, rec.Body.String(), "No upcoming reservations.")
}

func TestHandleList_Pagination(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerCaps)
	page := &reservation.Page{Items: []*reservation.ReservationWithSpot{item(7, "u2", time.Now().Add(time.Hour))}, Total: 120, Page: 2, Pages: 3, PerPage: 50}
	expectList(m, page)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/reservations?scope=all&page=2", cookie))

	// then
	body := rec.Body.String()
	assert.Contains(t, body, "Page 2 of 3")
	assert.Contains(t, body, "51–51 of 120 reservations")
	assert.Contains(t, body, `href="/servers/g1/reservations?scope=all"`)
	assert.Contains(t, body, `href="/servers/g1/reservations?page=3&amp;scope=all"`)
}

func TestHandleList_Errors(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerCaps)
	m.Reservations.EXPECT().Search(mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("down"))

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/reservations", cookie))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleSpot_FixesTheRespawn(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	m.Reservations.EXPECT().SpotOverview(mock.Anything, guildID, int64(4)).Return(&spot.Listed{
		Spot: spot.Spot{ID: 4, Name: "Hero Cave"}, Reservations: spot.ReservationCounts{Total: 12, Upcoming: 2},
	}, nil)
	m.Reservations.EXPECT().Search(mock.Anything, mock.MatchedBy(func(f reservation.SearchFilter) bool {
		return f.SpotID != nil && *f.SpotID == 4
	}), 0).Return(onePage(item(8, "u1", time.Now().Add(time.Hour))), nil)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots/4?spot=1", cookie))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "<h1 class=\"mt-1 font-display text-4xl font-bold -tracking-[0.02em] text-zone-50\">Hero Cave</h1>")
	assert.Contains(t, body, ">12</dd>")
	assert.Contains(t, body, `action="/servers/g1/spots/4"`)
	assert.NotContains(t, body, `id="filter-spot"`)
	assert.Contains(t, body, `{&#34;id&#34;:&#34;reservation-new-spot&#34;,&#34;selected&#34;:&#34;4&#34;}`)
}

func TestHandleSpot_NotFound(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, viewerCaps)
	m.Reservations.EXPECT().SpotOverview(mock.Anything, guildID, int64(99)).Return(nil, ports.ErrNotFound)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/spots/99", cookie))

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func createForm(start, end time.Time) url.Values {
	return url.Values{
		"spot_id":           {"4"},
		"start":             {start.Format(inputTime)},
		"end":               {end.Format(inputTime)},
		"author":            {"Storm Quiet"},
		"author_discord_id": {"u2"},
		"overbook":          {"1"},
	}
}

func TestHandleCreate_ManagerBooksForAnotherAuthor(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerCaps)
	start := time.Now().Add(2 * time.Hour).Truncate(time.Minute)
	end := start.Add(90 * time.Minute)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	m.Reservations.EXPECT().Create(mock.Anything, guildID, reservation.Actor{UserID: "u1", Name: "Quiet Nyx", Caps: managerCaps}, reservation.Draft{
		SpotID: 4, StartAt: start, EndAt: end, Author: "Storm Quiet", AuthorDiscordID: "u2", Overbook: true,
	}).Return([]*reservation.ClippedOrRemovedReservation{{Original: &reservation.Reservation{ID: 3}}}, nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations", createForm(start, end), cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, savedEvent, rec.Header().Get("HX-Trigger"))
	body := rec.Body.String()
	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, "Booked Hero Cave")
	assert.Contains(t, body, "1 reservation was overbooked.")
	assert.Contains(t, body, `id="reservation-new-form"`)
}

func TestHandleCreate_MemberCannotChooseTheAuthor(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	start := time.Now().Add(2 * time.Hour).Truncate(time.Minute)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	m.Reservations.EXPECT().Create(mock.Anything, guildID, mock.Anything, mock.MatchedBy(func(d reservation.Draft) bool {
		return d.Author == "Quiet Nyx" && d.AuthorDiscordID == "u1"
	})).Return(nil, nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations", createForm(start, start.Add(time.Hour)), cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "overbooked")
}

func TestHandleCreate_ConflictListsTheBlockingReservations(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	start := time.Now().Add(2 * time.Hour).Truncate(time.Minute)
	blocking := &reservation.Reservation{ID: 3, Author: "Storm Quiet", StartAt: start, EndAt: start.Add(time.Hour)}
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	m.Reservations.EXPECT().Create(mock.Anything, guildID, mock.Anything, mock.Anything).
		Return([]*reservation.ClippedOrRemovedReservation{{Original: blocking}}, booking.ErrInsufficientPermissions)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations", createForm(start, start.Add(time.Hour)), cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("HX-Trigger"))
	body := rec.Body.String()
	assert.Contains(t, body, `role="alert"`)
	assert.Contains(t, body, "Ask a manager to overbook")
	assert.Contains(t, body, "Storm Quiet · ")
}

func TestHandleCreate_FieldErrors(t *testing.T) {
	cases := map[string]struct {
		err      error
		expected string
	}{
		"too long":   {booking.ErrReservationTooLong, "A reservation can take up to 3 hours."},
		"past start": {booking.ErrStartInPast, "The start cannot be in the past."},
		"quota":      {booking.ErrQuotaExceeded, "One author can book 3 hours within 24 hours."},
		"archived":   {booking.ErrSpotArchived, "Choose an active respawn."},
		"started":    {booking.ErrSpotLocked, "It cannot move to another respawn."},
		"author id":  {reservations.ErrAuthorIDInvalid, "This is not a Discord user."},
		"no author":  {reservations.ErrAuthorUnknown, "We could not read your Discord name."},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, managerCaps)
			start := time.Now().Add(2 * time.Hour).Truncate(time.Minute)
			m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
			m.Reservations.EXPECT().Create(mock.Anything, guildID, mock.Anything, mock.Anything).Return(nil, tc.err)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations", createForm(start, start.Add(time.Hour)), cookie)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.expected)
		})
	}
}

func TestHandleCreate_UnreadableFieldsNeverReachTheService(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	form := url.Values{"spot_id": {""}, "start": {"tomorrow"}, "end": {"26/09/2026 14:00"}}

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations", form, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Choose a respawn.")
	assert.Contains(t, body, `id="reservation-new-start-error"`)
	assert.NotContains(t, body, `id="reservation-new-end-error"`)
	assert.Contains(t, body, `aria-invalid="true"`)
}

func TestHandleCreate_ServerError(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	start := time.Now().Add(2 * time.Hour)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	m.Reservations.EXPECT().Create(mock.Anything, guildID, mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations", createForm(start, start.Add(time.Hour)), cookie)))

	// then
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestHandleCreate_PlainPostRedirects(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	start := time.Now().Add(2 * time.Hour)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	m.Reservations.EXPECT().Create(mock.Anything, guildID, mock.Anything, mock.Anything).Return(nil, nil)

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/reservations", createForm(start, start.Add(time.Hour)), cookie))

	// then
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/servers/g1/reservations", rec.Header().Get("Location"))
}

func TestHandleCreate_ViewerIsForbidden(t *testing.T) {
	// given
	h, _, cookie := signedIn(t, viewerCaps)
	start := time.Now().Add(2 * time.Hour)

	// when
	rec := webtest.Serve(h, webtest.Post("/servers/g1/reservations", createForm(start, start.Add(time.Hour)), cookie))

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleCreate_CrossOriginIsRefused(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	start := time.Now().Add(2 * time.Hour)
	req := webtest.Post("/servers/g1/reservations", createForm(start, start.Add(time.Hour)), cookie)
	req.Header.Set("Sec-Fetch-Site", "cross-site")

	// when
	rec := webtest.Serve(h, req)

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleEditForm(t *testing.T) {
	cases := map[string]struct {
		caps     permission.Capabilities
		authorID string
		ended    bool
		expected int
	}{
		"owner":             {memberCaps, "u1", false, http.StatusOK},
		"manager of other":  {managerCaps, "u2", false, http.StatusOK},
		"member of other":   {memberCaps, "u2", false, http.StatusForbidden},
		"owner after ended": {memberCaps, "u1", true, http.StatusForbidden},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, tc.caps)
			start := time.Now().Add(time.Hour)
			if tc.ended {
				start = time.Now().Add(-5 * time.Hour)
			}
			m.Reservations.EXPECT().Get(mock.Anything, guildID, int64(7)).Return(item(7, tc.authorID, start), nil)
			m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil).Maybe()

			// when
			rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/reservations/7/edit", cookie)))

			// then
			require.Equal(t, tc.expected, rec.Code)
			if tc.expected == http.StatusOK {
				body := rec.Body.String()
				assert.Contains(t, body, `hx-post="/servers/g1/reservations/7/edit"`)
				assert.Contains(t, body, start.Format(inputTime))
				assert.NotContains(t, body, `name="overbook"`)
				assert.NotContains(t, body, "<html")
			}
		})
	}
}

func TestHandleEditForm_PlainGetIsAPageAndMissingIs404(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerCaps)
	m.Reservations.EXPECT().Get(mock.Anything, guildID, int64(7)).Return(item(7, "u2", time.Now().Add(time.Hour)), nil)
	m.Reservations.EXPECT().Get(mock.Anything, guildID, int64(8)).Return(nil, ports.ErrNotFound)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)

	// when
	page := webtest.Serve(h, webtest.Get("/servers/g1/reservations/7/edit", cookie))
	missing := webtest.Serve(h, webtest.Get("/servers/g1/reservations/8/edit", cookie))

	// then
	assert.Equal(t, http.StatusOK, page.Code)
	assert.Contains(t, page.Body.String(), "<html")
	assert.Contains(t, page.Body.String(), "Edit reservation")
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func TestHandleEdit_Saves(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, memberCaps)
	start := time.Now().Add(time.Hour).Truncate(time.Minute)
	m.Reservations.EXPECT().Get(mock.Anything, guildID, int64(8)).Return(item(8, "u1", start), nil)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	m.Reservations.EXPECT().Edit(mock.Anything, guildID, reservation.Actor{UserID: "u1", Caps: memberCaps}, int64(8), mock.MatchedBy(func(d reservation.Draft) bool {
		return d.SpotID == 1 && d.StartAt.Equal(start) && d.Author == "Author u1" && d.AuthorDiscordID == "u1"
	})).Return(nil, nil)
	form := url.Values{"spot_id": {"1"}, "start": {start.Format(inputTime)}, "end": {start.Add(time.Hour).Format(inputTime)}, "author": {"Hijack"}}

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations/8/edit", form, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, savedEvent, rec.Header().Get("HX-Trigger"))
	body := rec.Body.String()
	assert.Contains(t, body, "Reservation saved")
	assert.NotContains(t, body, "<form")
}

func TestHandleEdit_ConflictStaysInTheDialog(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerCaps)
	start := time.Now().Add(time.Hour).Truncate(time.Minute)
	m.Reservations.EXPECT().Get(mock.Anything, guildID, int64(8)).Return(item(8, "u2", start), nil)
	m.Reservations.EXPECT().Spots(mock.Anything, guildID).Return(guildSpots(), nil)
	m.Reservations.EXPECT().Edit(mock.Anything, guildID, mock.Anything, int64(8), mock.Anything).
		Return([]*reservation.Reservation{{ID: 9, Author: "Dark Quiet", StartAt: start, EndAt: start.Add(time.Hour)}}, booking.ErrConflict)
	form := url.Values{"spot_id": {"4"}, "start": {start.Format(inputTime)}, "end": {start.Add(time.Hour).Format(inputTime)}}

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations/8/edit", form, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "overlaps these times")
	assert.Contains(t, body, "Dark Quiet · ")
	assert.Contains(t, body, `id="reservation-edit-form"`)
}

func TestHandleEdit_GoneReservation(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerCaps)
	m.Reservations.EXPECT().Get(mock.Anything, guildID, int64(8)).Return(nil, ports.ErrNotFound)

	// when
	rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations/8/edit", url.Values{}, cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "does not exist any more")
}

func TestHandleDelete(t *testing.T) {
	cases := map[string]struct {
		err      error
		expected string
	}{
		"deleted":   {nil, "Reservation deleted."},
		"forbidden": {reservations.ErrForbidden, "You cannot delete this reservation."},
		"gone":      {ports.ErrNotFound, "does not exist any more"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// given
			h, m, cookie := signedIn(t, memberCaps)
			m.Reservations.EXPECT().Delete(mock.Anything, guildID, reservation.Actor{UserID: "u1", Caps: memberCaps}, int64(7)).Return(tc.err)

			// when
			rec := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations/7/delete", url.Values{}, cookie)))

			// then
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, savedEvent, rec.Header().Get("HX-Trigger"))
			assert.Contains(t, rec.Body.String(), tc.expected)
			assert.Contains(t, rec.Body.String(), `id="reservations-status"`)
		})
	}
}

func TestHandleDelete_PlainPostRedirectsAndFailureIs500(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerCaps)
	m.Reservations.EXPECT().Delete(mock.Anything, guildID, mock.Anything, int64(7)).Return(nil)
	m.Reservations.EXPECT().Delete(mock.Anything, guildID, mock.Anything, int64(8)).Return(errors.New("down"))

	// when
	plain := webtest.Serve(h, webtest.Post("/servers/g1/reservations/7/delete", url.Values{}, cookie))
	broken := webtest.Serve(h, htmx(webtest.Post("/servers/g1/reservations/8/delete", url.Values{}, cookie)))

	// then
	assert.Equal(t, http.StatusSeeOther, plain.Code)
	assert.Equal(t, http.StatusInternalServerError, broken.Code)
}

func TestHandleAuthors(t *testing.T) {
	// given
	h, m, cookie := signedIn(t, managerCaps)
	m.Reservations.EXPECT().KnownAuthors(mock.Anything, guildID, "sto").Return([]*reservation.KnownAuthor{{AuthorDiscordID: "u2", Author: "Storm Quiet"}}, nil)

	// when
	rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/reservations/authors?author=sto", cookie)))

	// then
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, `data-author="Storm Quiet"`)
	assert.Contains(t, body, `data-id="u2"`)
}

func TestHandleAuthors_MemberIsForbidden(t *testing.T) {
	// given
	h, _, cookie := signedIn(t, memberCaps)

	// when
	rec := webtest.Serve(h, htmx(webtest.Get("/servers/g1/reservations/authors?author=sto", cookie)))

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHandleList_NonPremiumShowsThePremiumPage(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, Register)
	cookie := webtest.SignIn(t, d, m, "u1")
	a := accessWith(memberCaps)
	a.Config.Premium = false
	m.Access.EXPECT().Access(mock.Anything, "u1", guildID).Return(a, nil)

	// when
	rec := webtest.Serve(h, webtest.Get("/servers/g1/reservations", cookie))

	// then
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.True(t, strings.Contains(rec.Body.String(), "premium") || strings.Contains(rec.Body.String(), "Premium"))
}
