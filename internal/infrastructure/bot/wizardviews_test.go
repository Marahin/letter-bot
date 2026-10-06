package bot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/reservation"
	"spot-assistant/internal/core/dto/spot"
	"spot-assistant/internal/core/reservationforms"
)

type sentOption struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

type sentItem struct {
	Type        int          `json:"type"`
	CustomID    string       `json:"custom_id"`
	Label       string       `json:"label"`
	Placeholder string       `json:"placeholder"`
	Disabled    bool         `json:"disabled"`
	Options     []sentOption `json:"options"`
}

type sentMessageRow struct {
	Components []sentItem `json:"components"`
}

func sentRows(t *testing.T, view formView) []sentMessageRow {
	t.Helper()
	raw, err := json.Marshal(view.components)
	require.NoError(t, err)
	var rows []sentMessageRow
	require.NoError(t, json.Unmarshal(raw, &rows))
	return rows
}

func customIDs(t *testing.T, view formView) []string {
	t.Helper()
	var ids []string
	for _, r := range sentRows(t, view) {
		for _, c := range r.Components {
			if c.CustomID != "" {
				ids = append(ids, c.CustomID)
			}
		}
	}
	return ids
}

// assertDiscordLimits checks the message limits that Discord refuses a message for.
func assertDiscordLimits(t *testing.T, view formView) {
	t.Helper()
	assert.LessOrEqual(t, utf8.RuneCountInString(view.content), 2000)
	rows := sentRows(t, view)
	assert.LessOrEqual(t, len(rows), maxRows)
	seen := map[string]bool{}
	for _, r := range rows {
		require.NotEmpty(t, r.Components)
		assert.LessOrEqual(t, len(r.Components), maxButtonsPerRow)
		for _, c := range r.Components {
			if c.CustomID != "" {
				assert.LessOrEqual(t, utf8.RuneCountInString(c.CustomID), maxCustomIDLength)
				assert.False(t, seen[c.CustomID], "custom id %q is not unique", c.CustomID)
				seen[c.CustomID] = true
				_, err := parseFormAction(c.CustomID)
				assert.NoError(t, err, c.CustomID)
			}
			if c.Type != 3 {
				assert.LessOrEqual(t, utf8.RuneCountInString(c.Label), maxLabelLength)
				continue
			}
			assert.Len(t, r.Components, 1, "a select takes a whole row")
			assert.LessOrEqual(t, utf8.RuneCountInString(c.Placeholder), maxPlaceholderLength)
			assert.NotEmpty(t, c.Options)
			assert.LessOrEqual(t, len(c.Options), maxSelectOptions)
			values := map[string]bool{}
			defaults := 0
			for _, o := range c.Options {
				assert.LessOrEqual(t, utf8.RuneCountInString(o.Label), maxOptionLength)
				assert.LessOrEqual(t, utf8.RuneCountInString(o.Description), maxOptionLength)
				assert.LessOrEqual(t, utf8.RuneCountInString(o.Value), maxOptionLength)
				assert.False(t, values[o.Value], "value %q is not unique", o.Value)
				values[o.Value] = true
				if o.Default {
					defaults++
				}
			}
			assert.LessOrEqual(t, defaults, 1)
		}
	}
}

func spotsNamed(n int) []*spot.Spot {
	spots := make([]*spot.Spot, 0, n)
	for i := range n {
		spots = append(spots, &spot.Spot{ID: int64(i + 1), Name: fmt.Sprintf("%c respawn %03d", 'A'+i*26/max(n, 1), i)})
	}
	return spots
}

func usual(n int) []reservation.UsualRespawn {
	out := make([]reservation.UsualRespawn, 0, n)
	for i := range n {
		out = append(out, reservation.UsualRespawn{Spot: reservation.Spot{ID: int64(500 + i), Name: fmt.Sprintf("Usual %d", i)}, Bookings: int64(n - i - 1)})
	}
	return out
}

func TestRespawnStepView(t *testing.T) {
	editState := formAction{ReservationID: 9, StartAt: time.Unix(1791226800, 0), Length: 2 * time.Hour}
	tests := []struct {
		name     string
		spots    int
		page     int
		usual    int
		state    formAction
		status   string
		wantRows int
	}{
		{"three groups, paged, usual", 200, 1, 25, formAction{}, "", 5},
		{"one group, no usual", 3, 0, 0, formAction{}, "", 2},
		{"no respawns", 0, 0, 0, formAction{}, "", 1},
		{"edit, last page", 160, 2, 2, editState, "", 3},
		{"with a status", 30, 0, 1, formAction{}, "No respawn has this name.", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			picker := reservationforms.PageRespawns(spotsNamed(tt.spots), tt.page)
			picker.Usual = usual(tt.usual)

			// when
			view := viewBot().respawnStepView(&picker, tt.state, tt.status)

			// then
			assertDiscordLimits(t, view)
			assert.Equal(t, tt.wantRows, rowCount(view))
			matchJSON(t, viewJSON(view))
		})
	}
}

func TestRespawnStepView_CutsLongNames(t *testing.T) {
	// given
	picker := reservationforms.PageRespawns([]*spot.Spot{{ID: 1, Name: strings.Repeat("Ż", 120)}}, 0)

	// when
	view := viewBot().respawnStepView(&picker, formAction{}, "")

	// then
	assertDiscordLimits(t, view)
	option := sentRows(t, view)[0].Components[0].Options[0]
	assert.Equal(t, 100, utf8.RuneCountInString(option.Label))
	assert.Equal(t, "1", option.Value)
}

func TestMatchesView(t *testing.T) {
	for _, tt := range []struct {
		name  string
		count int
		state formAction
	}{
		{"two", 2, formAction{}},
		{"too many, edit", 30, formAction{ReservationID: 9, Now: true, Length: time.Hour}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// when
			view := viewBot().matchesView("respawn", spotsNamed(tt.count), tt.state)

			// then
			assertDiscordLimits(t, view)
			assert.Equal(t, 2, rowCount(view))
			matchJSON(t, viewJSON(view))
		})
	}
}

// timePicker builds step 2 the way the core does, around viewNow (18:00 UTC).
func timePicker(choice reservation.TimeChoice, booked ...*reservation.Reservation) *reservation.TimePicker {
	if choice.SpotID == 0 {
		choice.SpotID = 7
	}
	p := &reservation.TimePicker{Spot: reservation.Spot{ID: choice.SpotID, Name: "Library -1"}, Choice: choice, Windows: reservationforms.Windows, Booked: booked}
	if choice.Window == 0 {
		p.Starts = append(p.Starts, reservation.StartOption{StartAt: viewNow, Now: true, Selected: choice.Now})
	}
	for _, slot := range reservationforms.StartSlots(viewNow, choice.Window) {
		if len(p.Starts) == reservationforms.SelectOptions {
			break
		}
		option := reservation.StartOption{StartAt: slot, Selected: !choice.Now && slot.Equal(choice.StartAt)}
		for _, r := range booked {
			if !slot.Before(r.StartAt) && slot.Before(r.EndAt) {
				option.BookedBy = r
			}
		}
		p.Starts = append(p.Starts, option)
	}
	for _, d := range reservationforms.Lengths() {
		p.Lengths = append(p.Lengths, reservation.LengthOption{Length: d, Selected: d == choice.Length})
	}
	return p
}

func TestTimeStepView(t *testing.T) {
	booked := &reservation.Reservation{Author: "Druid", StartAt: viewNow.Add(time.Hour), EndAt: viewNow.Add(3 * time.Hour)}
	editing := listed(9, "Library -1", viewNow.Add(70*time.Minute))
	ongoing := listed(9, "Library -1", viewNow.Add(-time.Hour))

	withCurrent := timePicker(reservation.TimeChoice{ReservationID: 9, StartAt: editing.StartAt, Length: 2 * time.Hour})
	withCurrent.Editing = editing
	withCurrent.Starts = append([]reservation.StartOption{{StartAt: editing.StartAt, Current: true, Selected: true}}, withCurrent.Starts[:reservationforms.SelectOptions-1]...)
	withCurrent.Lengths = append([]reservation.LengthOption{{Length: 70 * time.Minute, Current: true}}, withCurrent.Lengths...)

	running := timePicker(reservation.TimeChoice{ReservationID: 9, StartAt: ongoing.StartAt, Length: 2 * time.Hour})
	running.Editing, running.Ongoing = ongoing, true
	running.Starts = []reservation.StartOption{{StartAt: ongoing.StartAt, Keep: true, Selected: true}}
	running.Lengths = running.Lengths[2:]

	tests := []struct {
		name       string
		picker     *reservation.TimePicker
		wantRows   int
		wantSubmit string
	}{
		{"fresh", timePicker(reservation.TimeChoice{}), 4, ""},
		{"chosen, with booked slots", timePicker(reservation.TimeChoice{StartAt: viewNow.Add(90 * time.Minute), Length: 2 * time.Hour}, booked), 4, "lf1:wb:7:1791228600:120"},
		{"now chosen", timePicker(reservation.TimeChoice{Now: true, Length: 30 * time.Minute}), 4, "lf1:wb:7:now:30"},
		{"the last window", timePicker(reservation.TimeChoice{Window: 3, Length: time.Hour}), 4, ""},
		{"edit, current start off the grid", withCurrent, 4, "lf1:eb:9:7:1791227400:120"},
		{"edit, ongoing", running, 3, "lf1:eb:9:7:1791219600:120"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// when
			view := viewBot().timeStepView(tt.picker, "", viewNow)

			// then
			assertDiscordLimits(t, view)
			assert.Equal(t, tt.wantRows, rowCount(view))
			submit := sentRows(t, view)[tt.wantRows-1].Components[0]
			assert.Equal(t, tt.wantSubmit == "", submit.Disabled, "the submit waits for a start and a length")
			if tt.wantSubmit != "" {
				assert.Equal(t, tt.wantSubmit, submit.CustomID)
			}
			matchJSON(t, viewJSON(view))
		})
	}
}

func TestTimeStepView_WindowButtons(t *testing.T) {
	// when
	first := viewBot().timeStepView(timePicker(reservation.TimeChoice{}), "", viewNow)
	last := viewBot().timeStepView(timePicker(reservation.TimeChoice{Window: 3}), "", viewNow)

	// then
	firstRow, lastRow := sentRows(t, first)[2].Components, sentRows(t, last)[2].Components
	assert.True(t, firstRow[0].Disabled)
	assert.False(t, firstRow[1].Disabled)
	assert.Equal(t, "lf1:ww:7:1::0", firstRow[1].CustomID)
	assert.False(t, lastRow[0].Disabled)
	assert.Equal(t, "lf1:ww:7:2::0", lastRow[0].CustomID)
	assert.True(t, lastRow[1].Disabled)
}

func TestWizardErrorView(t *testing.T) {
	// when
	booking := viewBot().wizardErrorView(errors.New("boom"), formAction{SpotID: 7})
	edit := viewBot().wizardErrorView(errors.New("boom"), formAction{ReservationID: 9})

	// then
	assert.Equal(t, []string{"lf1:rp:0"}, customIDs(t, booking))
	assert.Equal(t, []string{"lf1:list"}, customIDs(t, edit))
}
