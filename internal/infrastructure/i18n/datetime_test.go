package i18n

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ts is the instant every format case renders: Monday 15 June 2026, 09:05 local.
func ts() time.Time { return time.Date(2026, time.June, 15, 9, 5, 0, 0, time.UTC) }

func localeCtx(code string) context.Context {
	return WithLocale(context.Background(), Normalize(code))
}

// TestDate_EnglishOutputIsUnchanged pins the English rendering to what the
// time.Format layouts these call sites used to carry, so routing them through the
// catalog changed no English page.
func TestDate_EnglishOutputIsUnchanged(t *testing.T) {
	ctx := context.Background()
	for f, want := range map[DateFormat]string{
		Weekday:                 "Mon",
		DayMonth:                "15 Jun",
		DayMonthTime:            "15 Jun 09:05",
		DayMonthYear:            "15 Jun 2026",
		DayMonthYearTime:        "15 Jun 2026 09:05",
		WeekdayDayMonth:         "Mon 15 Jun",
		WeekdayDayMonthTime:     "Mon 15 Jun · 09:05",
		WeekdayDayMonthYearTime: "Mon 15 Jun 2026 09:05",
	} {
		assert.Equalf(t, want, Date(ctx, f, ts()), "en %s", f)
	}
}

func TestDate_PerLocale(t *testing.T) {
	for code, want := range map[string]map[DateFormat]string{
		"pl": {
			Weekday:                 "pon.",
			DayMonthYear:            "15 cze 2026",
			WeekdayDayMonthYearTime: "pon., 15 cze 2026 09:05",
		},
	} {
		ctx := localeCtx(code)
		for f, w := range want {
			assert.Equalf(t, w, Date(ctx, f, ts()), "%s %s", code, f)
		}
	}
}

// TestDate_WeekdaysCoverTheWholeWeek checks the Monday-first indexing, which is
// the one place an off-by-one would silently label every date wrongly.
func TestDate_WeekdaysCoverTheWholeWeek(t *testing.T) {
	ctx := context.Background()
	want := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	for i, w := range want {
		// 15 June 2026 is a Monday.
		assert.Equal(t, w, Date(ctx, Weekday, ts().AddDate(0, 0, i)))
	}
}

// TestDate_FallsBackToEnglishPattern proves the field-level inheritance: pl ships
// no "day_month" pattern of its own, so it renders English's shape with Polish
// month names.
func TestDate_FallsBackToEnglishPattern(t *testing.T) {
	assert.Equal(t, "15 cze", Date(localeCtx("pl"), DayMonth, ts()))
}

// TestDate_NumbersAreNotGroupedByThePrinter guards the one trap: the message
// printer would render a large number under pl with a group separator. Dates are built with strconv
// precisely so they are not.
func TestDate_NumbersAreNotGroupedByThePrinter(t *testing.T) {
	assert.Contains(t, Date(localeCtx("pl"), DayMonthYear, ts()), "2026")
}

func TestDate_ZeroPadsDayAndTime(t *testing.T) {
	early := time.Date(2026, time.January, 2, 3, 4, 0, 0, time.UTC)
	assert.Equal(t, "02 Jan 2026 03:04", Date(context.Background(), DayMonthYearTime, early))
}

func TestDate_UnknownFormatRendersItsName(t *testing.T) {
	// A DateFormat that is not a constant cannot come from a catalog, so it can only
	// come from a caller inventing one; it names itself rather than blanking the cell.
	assert.Equal(t, "nonsense", Date(context.Background(), DateFormat("nonsense"), ts()))
}

func TestResolveDates_RejectsMalformedSections(t *testing.T) {
	base, err := resolveDates("en", catalogFileDates(t, "en"), dateNames{})
	require.NoError(t, err)

	for name, sec := range map[string]dateSection{
		"short months":   {MonthsShort: []string{"Jan"}},
		"short weekdays": {WeekdaysShort: []string{"Mon"}},
		"unknown format": {Formats: map[string]string{"day_month_epoch": "{DD}"}},
		"unknown token":  {Formats: map[string]string{"day_month": "{Dd}"}},
		"unclosed token": {Formats: map[string]string{"day_month": "{DD"}},
	} {
		_, err := resolveDates("pl", sec, base)
		assert.Errorf(t, err, "%s should not load", name)
	}
}

// TestResolveDates_EnglishMustBeComplete: English is what every gap resolves to,
// so an incomplete en.json is a broken build artefact, not a degraded locale.
func TestResolveDates_EnglishMustBeComplete(t *testing.T) {
	_, err := resolveDates("en", dateSection{}, dateNames{})
	assert.Error(t, err)
}

// catalogFileDates reads one shipped catalog's dates block.
func catalogFileDates(t *testing.T, code string) dateSection {
	t.Helper()
	f, err := loadFile(localesFS, "locales/"+code+".json")
	require.NoError(t, err)
	return f.Dates
}

// TestMonthAndWeekdayNamesShort_PerLocale covers the calendar the day-range
// picker draws in the browser: it reads names, not formatted dates, so a locale
// that ships its own must hand them over.
func TestMonthAndWeekdayNamesShort_PerLocale(t *testing.T) {
	for code, want := range map[string]struct{ month, weekday string }{
		"en": {"Jan", "Mon"},
		"pl": {"sty", "pon."},
	} {
		months := MonthNamesShort(localeCtx(code))
		weekdays := WeekdayNamesShort(localeCtx(code))
		require.Lenf(t, months, 12, "%s months", code)
		require.Lenf(t, weekdays, 7, "%s weekdays", code)
		assert.Equalf(t, want.month, months[0], "%s: January first", code)
		assert.Equalf(t, want.weekday, weekdays[0], "%s: Monday first", code)
	}
}

// TestMonthAndWeekdayNamesShort_AreCopies proves a caller cannot corrupt the
// catalog every later request reads.
func TestMonthAndWeekdayNamesShort_AreCopies(t *testing.T) {
	ctx := localeCtx("pl")
	MonthNamesShort(ctx)[0] = "broken"
	WeekdayNamesShort(ctx)[0] = "broken"

	assert.Equal(t, "sty", MonthNamesShort(ctx)[0])
	assert.Equal(t, "pon.", WeekdayNamesShort(ctx)[0])
}
