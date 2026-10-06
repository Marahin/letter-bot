package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/core/booking"
	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/infrastructure/bot/formatter"
)

var viewNow = time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC)

func viewBot() *Bot {
	return &Bot{formatter: formatter.NewFormatter(), webBaseURL: "https://tibialoot.test", log: zap.NewNop().Sugar()}
}

func listed(id int64, name string, startAt time.Time) *reservation.ReservationWithSpot {
	return &reservation.ReservationWithSpot{
		Reservation: reservation.Reservation{ID: id, SpotID: id * 10, StartAt: startAt, EndAt: startAt.Add(2 * time.Hour), AuthorDiscordID: "u1", Author: "Knight"},
		Spot:        reservation.Spot{ID: id * 10, Name: name},
	}
}

func pageOf(total int64, n int) *reservation.Page {
	items := make([]*reservation.ReservationWithSpot, 0, n)
	for i := range n {
		items = append(items, listed(int64(i+1), fmt.Sprintf("Respawn %d", i+1), viewNow.Add(time.Duration(i-1)*3*time.Hour)))
	}
	return &reservation.Page{Items: items, Total: total, Page: 1, Pages: 1, PerPage: listLimit}
}

func matchJSON(t *testing.T, v any) {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	snaps.MatchSnapshot(t, string(raw))
}

func viewJSON(v formView) map[string]any {
	return map[string]any{"content": v.content, "components": v.components}
}

func rowCount(v formView) int {
	return len(v.components)
}

func TestSummaryComponents(t *testing.T) {
	// when
	withPanel := summaryComponents("https://tibialoot.test")
	withoutPanel := summaryComponents("")

	// then
	matchJSON(t, withPanel)
	plain, ok := withoutPanel[0].(discordgo.ActionsRow)
	require.True(t, ok)
	assert.Len(t, plain.Components, 2, "no panel link without a web URL")
}

func TestListView(t *testing.T) {
	for _, tt := range []struct {
		name  string
		total int64
		items int
	}{{"empty", 0, 0}, {"one", 1, 1}, {"four", 4, 4}, {"six", 6, 4}} {
		t.Run(tt.name, func(t *testing.T) {
			// given
			page := pageOf(tt.total, tt.items)

			// when
			view := viewBot().listView(page, "", viewNow)

			// then
			assert.LessOrEqual(t, rowCount(view), 5)
			matchJSON(t, viewJSON(view))
		})
	}
}

func TestListView_StatusAndExtraButtons(t *testing.T) {
	// given
	retry := button("Try again", discordgo.SecondaryButton, formAction{Kind: actionEditWindow, ReservationID: 1, SpotID: 7, Window: reservation.AutoWindow})

	// when
	view := viewBot().listView(pageOf(4, 4), "Not changed.", viewNow, retry)

	// then
	assert.Equal(t, 5, rowCount(view))
	matchJSON(t, viewJSON(view))
}

func TestListView_UnreadList(t *testing.T) {
	// when
	view := viewBot().listView(nil, "Changed.", viewNow)

	// then
	assert.Contains(t, view.content, "Changed.")
	assert.Contains(t, view.content, "Could not load your reservations")
	assert.Equal(t, 1, rowCount(view))
}

func TestConfirmCancelView(t *testing.T) {
	// given
	r := listed(3, "Library -1", viewNow.Add(time.Hour))

	// when
	view := viewBot().confirmCancelView(r)

	// then
	matchJSON(t, viewJSON(view))
}

func TestSearchModal(t *testing.T) {
	// when
	modal := searchModal(formAction{Kind: actionEditSearchSubmit, ReservationID: 3, Now: true, Length: time.Hour})

	// then
	assert.Len(t, modal.Components, 1)
	assert.LessOrEqual(t, len(modal.Title), 45)
	matchJSON(t, modal)
}

var retryChoice = formAction{SpotID: 7, StartAt: time.Unix(1791226800, 0), Length: 2 * time.Hour, Window: reservation.AutoWindow}

func TestBookOutcomeView_Success(t *testing.T) {
	// given
	outcome := &reservation.FormOutcome{
		Draft:    reservation.Draft{SpotID: 7, StartAt: viewNow.Add(time.Hour), EndAt: viewNow.Add(3 * time.Hour)},
		SpotName: "Library -1",
		Overbooked: []*reservation.ClippedOrRemovedReservation{
			{Original: &reservation.Reservation{Author: "Druid", StartAt: viewNow, EndAt: viewNow.Add(2 * time.Hour)}, New: []*reservation.Reservation{{StartAt: viewNow, EndAt: viewNow.Add(time.Hour)}}},
			{Original: &reservation.Reservation{Author: "Paladin", StartAt: viewNow.Add(time.Hour), EndAt: viewNow.Add(2 * time.Hour)}},
		},
	}

	// when
	view := viewBot().bookOutcomeView(outcome, nil, retryChoice)

	// then
	matchJSON(t, viewJSON(view))
}

func conflictOutcome(canOverbook bool) *reservation.FormOutcome {
	return &reservation.FormOutcome{
		Draft:       reservation.Draft{SpotID: 7, StartAt: viewNow.Add(time.Hour), EndAt: viewNow.Add(3 * time.Hour)},
		SpotName:    "Library -1",
		Conflicts:   []*reservation.Reservation{{Author: "Druid", StartAt: viewNow, EndAt: viewNow.Add(2 * time.Hour)}},
		CanOverbook: canOverbook,
	}
}

func TestBookOutcomeView_Conflicts(t *testing.T) {
	for name, canOverbook := range map[string]bool{"can overbook": true, "cannot overbook": false} {
		t.Run(name, func(t *testing.T) {
			// when
			view := viewBot().bookOutcomeView(conflictOutcome(canOverbook), booking.ErrInsufficientPermissions, retryChoice)

			// then
			assertDiscordLimits(t, view)
			matchJSON(t, viewJSON(view))
		})
	}
}

func TestBookOutcomeView_Error(t *testing.T) {
	// when
	view := viewBot().bookOutcomeView(nil, booking.ErrStartInPast, formAction{SpotID: 7, Now: true, Length: time.Hour, Window: reservation.AutoWindow})

	// then
	assert.Equal(t, []string{"lf1:ww:7::now:60"}, customIDs(t, view))
	matchJSON(t, viewJSON(view))
}

func TestEditOutcomeView(t *testing.T) {
	// given
	retry := formAction{ReservationID: 1, SpotID: 7, StartAt: time.Unix(1791226800, 0), Length: 2 * time.Hour, Window: reservation.AutoWindow}
	tests := map[string]struct {
		outcome *reservation.FormOutcome
		err     error
	}{
		"success":  {conflictOutcome(false), nil},
		"conflict": {conflictOutcome(false), booking.ErrConflict},
		"error":    {nil, errors.New("boom")},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			view := viewBot().editOutcomeView(tt.outcome, tt.err, retry, pageOf(1, 1), viewNow)

			// then
			assertDiscordLimits(t, view)
			matchJSON(t, viewJSON(view))
		})
	}
}

func TestFormView_EditClearsTheComponents(t *testing.T) {
	// when
	edit := formView{content: "x"}.edit()

	// then
	raw, err := json.Marshal(edit)
	require.NoError(t, err)
	assert.JSONEq(t, `{"content":"x","components":[],"allowed_mentions":{"parse":[],"replied_user":false}}`, string(raw))
}

func TestEditOutcomeView_UnreadList(t *testing.T) {
	// given
	outcome := conflictOutcome(false)

	// when
	view := viewBot().editOutcomeView(outcome, nil, formAction{ReservationID: 1}, nil, viewNow)

	// then
	assert.Contains(t, view.content, "Library -1")
	assert.Contains(t, view.content, "Could not load your reservations")
}

func TestCut(t *testing.T) {
	// when
	short := cut("abc", 3)
	long := cut("abcd", 3)

	// then
	assert.Equal(t, "abc", short)
	assert.Equal(t, "ab…", long)
}
