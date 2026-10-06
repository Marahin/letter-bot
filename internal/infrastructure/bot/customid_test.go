package bot

import (
	"math"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/reservation"
)

func TestFormAction_RoundTrip(t *testing.T) {
	// given
	start := time.Unix(1790000000, 0)
	end := start.Add(2 * time.Hour)
	auto := reservation.AutoWindow
	actions := []formAction{
		{Kind: actionBook},
		{Kind: actionMine},
		{Kind: actionList},
		{Kind: actionEdit, ReservationID: 5},
		{Kind: actionCancel, ReservationID: 5},
		{Kind: actionCancelConfirm, ReservationID: 5},
		{Kind: actionOverbook, SpotID: 7, StartAt: start, EndAt: end},
		{Kind: actionRespawnPage, Page: 2},
		{Kind: actionRespawnPick, Index: 3},
		{Kind: actionSearch},
		{Kind: actionSearchSubmit},
		{Kind: actionStartPick, SpotID: 7, Window: 1, Length: 90 * time.Minute},
		{Kind: actionLengthPick, SpotID: 7, Window: auto, Now: true},
		{Kind: actionWindow, SpotID: 7, Window: 3, StartAt: start},
		{Kind: actionSubmit, SpotID: 7, StartAt: start, Length: time.Hour},
		{Kind: actionEditRespawnPage, ReservationID: 5, StartAt: start, Length: time.Hour, Page: 1},
		{Kind: actionEditRespawnPick, ReservationID: 5, Now: true, Length: time.Hour, Index: 4},
		{Kind: actionEditSearch, ReservationID: 5, StartAt: start, Length: 70 * time.Minute},
		{Kind: actionEditSearchSubmit, ReservationID: 5, StartAt: start, Length: 70 * time.Minute},
		{Kind: actionEditStartPick, ReservationID: 5, SpotID: 7, Window: auto, Length: 2 * time.Hour},
		{Kind: actionEditLengthPick, ReservationID: 5, SpotID: 7, Window: 0, StartAt: start},
		{Kind: actionEditWindow, ReservationID: 5, SpotID: 7, Window: 2, Now: true, Length: 30 * time.Minute},
		{Kind: actionEditSubmit, ReservationID: 5, SpotID: 7, StartAt: start, Length: 3 * time.Hour},
	}
	require.Len(t, actions, len(actionLayouts), "every action is covered")
	for _, action := range actions {
		t.Run(actionLayouts[action.Kind].name, func(t *testing.T) {
			// when
			parsed, err := parseFormAction(action.customID())

			// then
			require.NoError(t, err)
			assert.Equal(t, action, parsed)
		})
	}
}

func TestFormAction_FitsDiscordsLimit(t *testing.T) {
	// given
	far := time.Unix(math.MaxInt64/2, 0)
	for kind := range actionLayouts {
		action := formAction{
			Kind:          kind,
			ReservationID: math.MaxInt64,
			SpotID:        math.MaxInt64,
			StartAt:       far,
			EndAt:         far,
			Length:        maxLengthMin * time.Minute,
			Window:        maxWindow,
			Page:          maxPage,
			Index:         maxIndex,
		}

		// when
		id := action.customID()

		// then
		assert.LessOrEqual(t, utf8.RuneCountInString(id), maxCustomIDLength, id)
		_, err := parseFormAction(id)
		assert.NoError(t, err, id)
	}
}

func TestFormAction_As(t *testing.T) {
	// given
	booking := formAction{SpotID: 7}
	edit := formAction{ReservationID: 5, SpotID: 7}

	// when
	bookPick := booking.as(actionEditStartPick)
	editPick := edit.as(actionStartPick)
	list := edit.as(actionList)

	// then
	assert.Equal(t, actionStartPick, bookPick.Kind)
	assert.Equal(t, actionEditStartPick, editPick.Kind)
	assert.Equal(t, actionList, list.Kind)
}

func TestParseFormAction_RejectsMalformedIDs(t *testing.T) {
	// given
	for _, id := range []string{
		"",
		"book",
		"lf0:book",
		"lf1:",
		"lf1:nope",
		"lf1:book:extra",
		"lf1:edit",
		"lf1:edit:",
		"lf1:edit:abc",
		"lf1:edit:-4",
		"lf1:ob:7:1:",
		"lf1:ob:7:x:2",
		"lf1:rp:",
		"lf1:rp:-1",
		"lf1:rp:1000",
		"lf1:rs:10",
		"lf1:rq:1",
		"lf1:ws:7:10:60",
		"lf1:ws:7:x:60",
		"lf1:ws:7:0:1441",
		"lf1:ws:7:0:60:1",
		"lf1:wl:7:0:soon",
		"lf1:wl:7:0:+5",
		"lf1:wb:0:now:60",
		"lf1:wb:7:now",
		"lf1:eb:0:7:now:60",
		"lf1:erp:5:now:60",
		"lf1:pick:1:2",
		"lf1:retry:1830:2030:x",
		"lf1:mbook",
	} {
		t.Run(id, func(t *testing.T) {
			// when
			_, err := parseFormAction(id)

			// then
			assert.ErrorIs(t, err, errBadCustomID)
		})
	}
}
