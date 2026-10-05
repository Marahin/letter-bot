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
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/reservationforms"
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
	matchJSON(t, summaryComponents("https://tibialoot.test"))
	plain, ok := summaryComponents("")[0].(discordgo.ActionsRow)
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
			// when
			view := viewBot().listView(pageOf(tt.total, tt.items), "", viewNow)

			// then
			assert.LessOrEqual(t, rowCount(view), 5)
			matchJSON(t, viewJSON(view))
		})
	}
}

func TestListView_StatusAndExtraButtons(t *testing.T) {
	// given
	retry := button("Try again", discordgo.SecondaryButton, formAction{Kind: actionEditRetry, ReservationID: 1})

	// when
	view := viewBot().listView(pageOf(4, 4), "Not changed.", viewNow, retry)

	// then
	assert.Equal(t, 5, rowCount(view))
	matchJSON(t, viewJSON(view))
}

func TestConfirmCancelView(t *testing.T) {
	matchJSON(t, viewJSON(viewBot().confirmCancelView(listed(3, "Library -1", viewNow.Add(time.Hour)))))
}

func TestModals(t *testing.T) {
	for name, modal := range map[string]*discordgo.InteractionResponseData{
		"book":  bookModal(reservation.Form{}),
		"retry": bookModal(retryForm(formAction{Kind: actionRetry, StartText: "1830", SpotText: "Library"})),
		"edit":  editModal(3, existingForm(listed(3, "Library -1", viewNow.Add(time.Hour)))),
	} {
		t.Run(name, func(t *testing.T) {
			assert.LessOrEqual(t, len(modal.Components), 5)
			assert.LessOrEqual(t, len(modal.Title), 45)
			matchJSON(t, modal)
		})
	}
}

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
	view := viewBot().bookOutcomeView(outcome, nil, reservation.Form{})

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
	form := reservation.Form{Spot: "Library", StartAt: "19:00", EndAt: "21.00"}
	for name, canOverbook := range map[string]bool{"can overbook": true, "cannot overbook": false} {
		t.Run(name, func(t *testing.T) {
			// when
			view := viewBot().bookOutcomeView(conflictOutcome(canOverbook), booking.ErrInsufficientPermissions, form)

			// then
			matchJSON(t, viewJSON(view))
		})
	}
}

func candidates(n int) []*spot.Spot {
	spots := make([]*spot.Spot, 0, n)
	for i := range n {
		spots = append(spots, &spot.Spot{ID: int64(i + 1), Name: fmt.Sprintf("Library -%d", i+1)})
	}
	return spots
}

func TestBookOutcomeView_Ambiguous(t *testing.T) {
	// given
	outcome := &reservation.FormOutcome{Draft: reservation.Draft{StartAt: viewNow.Add(time.Hour), EndAt: viewNow.Add(3 * time.Hour)}}
	err := &reservationforms.AmbiguousSpotError{Query: "Library", Candidates: candidates(15)}

	// when
	view := viewBot().bookOutcomeView(outcome, err, reservation.Form{Spot: "Library", StartAt: "19:00", EndAt: "21:00"})

	// then
	selectRow, ok := view.components[0].(discordgo.ActionsRow)
	require.True(t, ok)
	menu, ok := selectRow.Components[0].(plainSelect)
	require.True(t, ok)
	assert.Len(t, menu.Options, 15)
	for _, option := range menu.Options {
		assert.NotEmpty(t, option.Description)
	}
	matchJSON(t, viewJSON(view))
}

func TestBookOutcomeView_Error(t *testing.T) {
	// when
	view := viewBot().bookOutcomeView(nil, reservationforms.ErrTimeFormat, reservation.Form{Spot: "Library", StartAt: "late", EndAt: "21:00"})

	// then
	matchJSON(t, viewJSON(view))
}

func TestEditOutcomeView(t *testing.T) {
	form := reservation.Form{Spot: "Library -1", StartAt: "19:00", EndAt: "21:00"}
	tests := map[string]struct {
		outcome *reservation.FormOutcome
		err     error
	}{
		"success":   {conflictOutcome(false), nil},
		"conflict":  {conflictOutcome(false), booking.ErrConflict},
		"error":     {nil, errors.New("boom")},
		"ambiguous": {conflictOutcome(false), &reservationforms.AmbiguousSpotError{Query: "Lib", Candidates: candidates(2)}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// when
			view := viewBot().editOutcomeView(1, tt.outcome, tt.err, form, pageOf(1, 1), viewNow)

			// then
			assert.LessOrEqual(t, rowCount(view), 5)
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

func TestCut(t *testing.T) {
	assert.Equal(t, "abc", cut("abc", 3))
	assert.Equal(t, "ab…", cut("abcd", 3))
}

func TestTypedClock(t *testing.T) {
	assert.Equal(t, "0930", typedClock("9.30"))
	assert.Equal(t, "", typedClock("late"))
}
