package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/infrastructure/i18n"
)

func rangePickerProps() RangePickerProps {
	return RangePickerProps{
		Action:       "/servers/g1/stats",
		Label:        "15 Jun – 21 Jun 2026",
		SelectedDays: []string{"2026-06-15", "2026-06-16"},
		DataDays:     []string{"2026-06-16"},
		Explicit:     true,
		DataLegend:   "Days with recorded data",
		Hidden:       []RangePickerHidden{{Name: "lineup", Value: "7"}},
	}
}

func renderRangePicker(ctx context.Context, t *testing.T, p RangePickerProps) string {
	t.Helper()
	var sb strings.Builder
	require.NoError(t, RangePicker(p).Render(ctx, &sb))
	return sb.String()
}

// attrValue reads one attribute off the rendered markup. templ escapes a JSON
// array's quotes, so the test unescapes before parsing.
func attrValue(t *testing.T, out, name string) string {
	t.Helper()
	marker := name + `="`
	i := strings.Index(out, marker)
	require.GreaterOrEqualf(t, i, 0, "%s is not in the markup", name)
	rest := out[i+len(marker):]
	j := strings.Index(rest, `"`)
	require.GreaterOrEqual(t, j, 0)
	return strings.ReplaceAll(rest[:j], "&#34;", `"`)
}

func TestRangePicker_RendersTheToggleThePanelAndTheNoScriptFallback(t *testing.T) {
	// given a picker with a selection, when it renders
	out := renderRangePicker(context.Background(), t, RangePickerFor(context.Background(), rangePickerProps()))

	// then the script-only control ships hidden, with the panel and the readout
	assert.Contains(t, out, "data-range-ui")
	assert.Contains(t, out, "data-range-toggle")
	assert.Contains(t, out, "data-range-popover")
	assert.Contains(t, out, "data-range-calendar")
	assert.Contains(t, out, `data-range-label>15 Jun – 21 Jun 2026<`)
	assert.Contains(t, out, `aria-haspopup="dialog"`)
	assert.Contains(t, out, `role="dialog"`)
	assert.Contains(t, out, "Days with recorded data")

	// and the sheet can be dismissed with no Escape key, while the live region
	// waits outside the panel the script rebuilds
	assert.Contains(t, out, "data-range-close")
	assert.Contains(t, out, "data-range-backdrop")
	assert.Contains(t, out, `data-range-live class="sr-only" aria-live="polite"`)

	// and the no-script reader still gets a from/to span with the page's own
	// lineup scope carried over
	assert.Contains(t, out, "<noscript>")
	assert.Contains(t, out, `name="from"`)
	assert.Contains(t, out, `name="to"`)
	assert.Contains(t, out, `name="lineup" value="7"`)

	// and the scripts get their copy through the strings island
	assert.Contains(t, out, "data-letter-strings")
	assert.Contains(t, out, "range.picker.hint_armed")
	assert.Contains(t, out, "range.picker.mode_days")
}

func TestRangePicker_DayAttributesAreJSONArrays(t *testing.T) {
	// given a picker, when it renders
	out := renderRangePicker(context.Background(), t, RangePickerFor(context.Background(), rangePickerProps()))

	// then the browser reads every day list as JSON
	for _, attr := range []string{"data-selected", "data-days", "data-months", "data-weekdays"} {
		var got []string
		require.NoErrorf(t, json.Unmarshal([]byte(attrValue(t, out, attr)), &got), "%s is not a JSON array", attr)
		assert.NotEmptyf(t, got, "%s is empty", attr)
	}
	assert.Equal(t, `["2026-06-15","2026-06-16"]`, attrValue(t, out, "data-selected"))
	assert.Equal(t, `["2026-06-16"]`, attrValue(t, out, "data-days"))
}

func TestRangePicker_EmptyDayListsStayValidJSON(t *testing.T) {
	// given nothing selected and no recorded data
	p := RangePickerProps{Action: "/servers/g1/attendance"}

	// when it renders, then the attributes are empty arrays, not empty strings
	out := renderRangePicker(context.Background(), t, p)
	assert.Equal(t, "[]", attrValue(t, out, "data-selected"))
	assert.Equal(t, "[]", attrValue(t, out, "data-days"))
}

func TestRangePicker_ExplicitOnlyWhenTheSelectionIsPinned(t *testing.T) {
	// given a pinned selection, then the picker writes it to the URL on load
	pinned := renderRangePicker(context.Background(), t, rangePickerProps())
	assert.Contains(t, pinned, "data-explicit")

	// given the page's implicit default, then it does not
	p := rangePickerProps()
	p.Explicit = false
	assert.NotContains(t, renderRangePicker(context.Background(), t, p), "data-explicit")
}

// TestRangePickerFor_CarriesTheLocalesOwnDatePattern: the panel formats its dates
// in the browser, so it needs the locale's own day/month order as data.
func TestRangePickerFor_CarriesTheLocalesOwnDatePattern(t *testing.T) {
	loc, ok := i18n.Lookup("pl")
	require.True(t, ok)
	ctx := i18n.WithLocale(context.Background(), loc)

	p := RangePickerFor(ctx, rangePickerProps())

	assert.Equal(t, "{DD} {Mon}", p.DatePattern)
	assert.Contains(t, renderRangePicker(ctx, t, p), `data-date-format="{DD} {Mon}"`)
}

func TestRangePickerFor_CarriesTheLocalesOwnCalendarNames(t *testing.T) {
	// given a Polish request
	loc, ok := i18n.Lookup("pl")
	require.True(t, ok)
	ctx := i18n.WithLocale(context.Background(), loc)

	// when the picker is built for it
	p := RangePickerFor(ctx, rangePickerProps())

	// then the calendar gets Polish month and weekday names, 12 and 7 of them
	require.Len(t, p.MonthsShort, 12)
	require.Len(t, p.WeekdaysShort, 7)
	out := renderRangePicker(ctx, t, p)
	assert.Contains(t, attrValue(t, out, "data-months"), "sty")
	assert.Contains(t, attrValue(t, out, "data-weekdays"), "pon.")
}

func TestRangePickerFor_EnglishIsTheFallbackCalendar(t *testing.T) {
	// given a request with no locale, then the picker still ships English names
	p := RangePickerFor(context.Background(), rangePickerProps())
	require.Len(t, p.MonthsShort, 12)
	assert.Equal(t, "Jan", p.MonthsShort[0])
	require.Len(t, p.WeekdaysShort, 7)
	assert.Equal(t, "Mon", p.WeekdaysShort[0])
}
