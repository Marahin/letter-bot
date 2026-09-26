package web

import (
	"context"
	"encoding/json"

	"spot-assistant/internal/infrastructure/i18n"
)

// RangePickerHidden is one extra query value the picker's GET submit must carry
// over, so applying a range does not widen the page back (the lineup scope).
type RangePickerHidden struct {
	Name  string
	Value string
}

// RangePickerProps carries the whole day-range picker. Stats and Reservations
// differ only in these values, so the atom needs no page knowledge.
type RangePickerProps struct {
	// Action is the page's own URL, the GET target the Apply submit posts to.
	Action string
	// Label is the already-localized selection text on the closed toggle.
	Label string
	// SelectedDays and DataDays are YYYY-MM-DD keys: the current selection, and
	// the days that have recorded data (the green dots).
	SelectedDays []string
	DataDays     []string
	// Explicit is true when the selection is pinned rather than the page default.
	Explicit bool
	// DataLegend is the already-localized dot legend, which differs per page.
	DataLegend string
	// Hidden carries the page's non-range query values through the submit.
	Hidden []RangePickerHidden
	// MonthsShort and WeekdaysShort are the locale's own calendar names, 12
	// January first and 7 Monday first. DatePattern is the locale's day/month
	// pattern, which the browser needs to order a date itself. RangePickerFor
	// fills all three.
	MonthsShort   []string
	WeekdaysShort []string
	DatePattern   string
}

// RangePickerFor completes a day-range picker with the locale's own calendar
// vocabulary. The calendar is drawn in the browser, which has no catalog, so the
// month names, the weekday names and the date pattern travel with the markup.
func RangePickerFor(ctx context.Context, p RangePickerProps) RangePickerProps {
	p.MonthsShort = i18n.MonthNamesShort(ctx)
	p.WeekdaysShort = i18n.WeekdayNamesShort(ctx)
	p.DatePattern = i18n.DatePattern(ctx, i18n.DayMonth)
	return p
}

// jsonArray encodes a string list as a JSON array for a data attribute. A slice
// of strings cannot fail to marshal, so a failure degrades to an empty array.
func jsonArray(values []string) string {
	if values == nil {
		values = []string{}
	}
	b, err := json.Marshal(values)
	if err != nil {
		return "[]"
	}
	return string(b)
}
