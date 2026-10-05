package bot

import (
	"math"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormAction_RoundTrip(t *testing.T) {
	// given
	start := time.Unix(1790000000, 0)
	end := start.Add(2 * time.Hour)
	actions := []formAction{
		{Kind: actionBookForm},
		{Kind: actionMine},
		{Kind: actionList},
		{Kind: actionEditForm, ReservationID: 5},
		{Kind: actionCancel, ReservationID: 5},
		{Kind: actionCancelConfirm, ReservationID: 5},
		{Kind: actionOverbook, SpotID: 7, StartAt: start, EndAt: end},
		{Kind: actionBookPick, StartAt: start, EndAt: end},
		{Kind: actionEditPick, ReservationID: 5, StartAt: start, EndAt: end},
		{Kind: actionBookRetry, StartText: "1830", EndText: "", SpotText: "Banuta: -1"},
		{Kind: actionEditRetry, ReservationID: 5, StartText: "0900", EndText: "1100", SpotText: ""},
		{Kind: actionBookSubmit},
		{Kind: actionEditSubmit, ReservationID: 5},
	}
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
	far := time.Unix(math.MaxInt32, 0)
	actions := []formAction{
		{Kind: actionEditPick, ReservationID: math.MaxInt64, StartAt: far, EndAt: far},
		{Kind: actionOverbook, SpotID: math.MaxInt64, StartAt: far, EndAt: far},
		{Kind: actionEditRetry, ReservationID: math.MaxInt64, StartText: "2359", EndText: "2359", SpotText: strings.Repeat("Ż", 100)},
	}
	for _, action := range actions {
		// when
		id := action.customID()

		// then
		assert.LessOrEqual(t, utf8.RuneCountInString(id), maxCustomIDLength)
		assert.True(t, utf8.ValidString(id))
		_, err := parseFormAction(id)
		assert.NoError(t, err)
	}
}

func TestFormAction_CutsALongSpotText(t *testing.T) {
	// given
	action := formAction{Kind: actionBookRetry, StartText: "1830", EndText: "2030", SpotText: strings.Repeat("a", 120)}

	// when
	parsed, err := parseFormAction(action.customID())

	// then
	require.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 80), parsed.SpotText)
	assert.Equal(t, "1830", parsed.StartText)
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
		"lf1:pick:1",
		"lf1:retry:18:2030:x",
		"lf1:retry:18ab:2030:x",
		"lf1:retry:1830:2030",
	} {
		t.Run(id, func(t *testing.T) {
			// when
			_, err := parseFormAction(id)

			// then
			assert.ErrorIs(t, err, errBadCustomID)
		})
	}
}

func TestClockInput(t *testing.T) {
	// when
	filled := clockInput("1830")
	empty := clockInput("")

	// then
	assert.Equal(t, "18:30", filled)
	assert.Equal(t, "", empty)
}
