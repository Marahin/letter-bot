package i18n

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DateFormat names one of the date/time shapes this app renders. time.Format has
// no locale seam at all (its reference layout hard-codes English "Mon" and
// "Jan"), so every user-visible date goes through Date and a per-locale pattern
// from locales/<code>.json "dates".
//
// The constant values are the pattern keys in that file. Adding a shape means
// adding a constant, a line in dateFormats, and a pattern in every catalog that
// wants its own; a catalog that omits one inherits English's.
type DateFormat string

const (
	Weekday                 DateFormat = "weekday"                     // Mon
	DayMonth                DateFormat = "day_month"                   // 02 Jan
	DayMonthTime            DateFormat = "day_month_time"              // 02 Jan 15:04
	DayMonthYear            DateFormat = "day_month_year"              // 02 Jan 2026
	DayMonthYearTime        DateFormat = "day_month_year_time"         // 02 Jan 2026 15:04
	WeekdayDayMonth         DateFormat = "weekday_day_month"           // Mon 02 Jan
	WeekdayDayMonthTime     DateFormat = "weekday_day_month_time"      // Mon 02 Jan · 15:04
	WeekdayDayMonthYearTime DateFormat = "weekday_day_month_year_time" // Mon 02 Jan 2026 15:04
)

// dateFormats is every shape a catalog must define in English. init rejects an
// en.json missing one, so a new constant cannot ship without its pattern.
var dateFormats = []DateFormat{
	Weekday,
	DayMonth,
	DayMonthTime,
	DayMonthYear,
	DayMonthYearTime,
	WeekdayDayMonth,
	WeekdayDayMonthTime,
	WeekdayDayMonthYearTime,
}

// dateTokens are the placeholders a pattern may use. Patterns are validated
// against this set at load time, so a translator's typo is a build failure rather
// than a half-rendered date on a page.
var dateTokens = []string{"Wd", "D", "DD", "Mon", "YYYY", "HH", "mm"}

// dateNames is one locale's resolved date vocabulary.
type dateNames struct {
	monthsShort   []string // 12, January first
	weekdaysShort []string // 7, Monday first
	formats       map[DateFormat]string
}

// resolveDates turns a catalog's "dates" block into a locale's date vocabulary,
// filling anything it leaves out from base (English). base is the zero dateNames
// when resolving English itself, which is why English must be complete.
func resolveDates(code string, sec dateSection, base dateNames) (dateNames, error) {
	d := base
	if n := len(sec.MonthsShort); n > 0 {
		if n != 12 {
			return d, fmt.Errorf("%s: dates.months_short has %d entries, want 12", code, n)
		}
		d.monthsShort = sec.MonthsShort
	}
	if n := len(sec.WeekdaysShort); n > 0 {
		if n != 7 {
			return d, fmt.Errorf("%s: dates.weekdays_short has %d entries, want 7 (Monday first)", code, n)
		}
		d.weekdaysShort = sec.WeekdaysShort
	}
	d.formats = make(map[DateFormat]string, len(dateFormats))
	for k, v := range base.formats {
		d.formats[k] = v
	}
	for name, pattern := range sec.Formats {
		f := DateFormat(name)
		if !slices.Contains(dateFormats, f) {
			return d, fmt.Errorf("%s: dates.formats has unknown format %q", code, name)
		}
		if err := checkPattern(code, name, pattern); err != nil {
			return d, err
		}
		d.formats[f] = pattern
	}
	if base.formats != nil {
		return d, nil
	}
	// English is the source: it must cover everything, because every other
	// locale's gaps resolve to it.
	if len(d.monthsShort) != 12 || len(d.weekdaysShort) != 7 {
		return d, fmt.Errorf("%s: dates needs 12 months_short and 7 weekdays_short", code)
	}
	for _, f := range dateFormats {
		if d.formats[f] == "" {
			return d, fmt.Errorf("%s: dates.formats is missing %q", code, f)
		}
	}
	return d, nil
}

// checkPattern rejects a pattern using a token no renderer knows.
func checkPattern(code, name, pattern string) error {
	for rest := pattern; ; {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			return nil
		}
		rest = rest[open+1:]
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			return fmt.Errorf("%s: dates.formats %q has an unclosed placeholder", code, name)
		}
		tok := rest[:end]
		found := false
		for _, known := range dateTokens {
			if tok == known {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%s: dates.formats %q uses unknown placeholder {%s} (known: %s)",
				code, name, tok, strings.Join(dateTokens, ", "))
		}
		rest = rest[end+1:]
	}
}

// Date renders t in ctx's locale using the named shape. The zone is the caller's:
// Date formats whatever t carries, so a call site that wants local or UTC keeps
// its own .In(...) conversion.
//
// Numbers here are built with strconv, never the message printer: the printer
// groups digits per locale ("2 026" for a year under ru), which is right for
// counts and wrong for a date.
func Date(ctx context.Context, f DateFormat, t time.Time) string {
	return From(ctx).date(f, t)
}

// date renders one pattern, substituting the {…} placeholders.
func (l Locale) date(f DateFormat, t time.Time) string {
	pattern, ok := l.dates.formats[f]
	if !ok {
		// Unreachable for a DateFormat constant: init proves en.json covers every
		// one and every locale inherits its patterns. A caller inventing a name
		// gets it back rather than an empty cell.
		return string(f)
	}
	var b strings.Builder
	b.Grow(len(pattern) + 12)
	for rest := pattern; rest != ""; {
		open := strings.IndexByte(rest, '{')
		if open < 0 {
			b.WriteString(rest)
			break
		}
		b.WriteString(rest[:open])
		rest = rest[open+1:]
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			// checkPattern rejects this at load time; belt and braces so a rendering
			// path can never slice out of range.
			break
		}
		b.WriteString(l.dateToken(rest[:end], t))
		rest = rest[end+1:]
	}
	return b.String()
}

// dateToken expands one placeholder.
func (l Locale) dateToken(tok string, t time.Time) string {
	switch tok {
	case "Wd":
		return l.dates.weekdaysShort[(int(t.Weekday())+6)%7]
	case "D":
		return strconv.Itoa(t.Day())
	case "DD":
		return pad2(t.Day())
	case "Mon":
		return l.dates.monthsShort[int(t.Month())-1]
	case "YYYY":
		return strconv.Itoa(t.Year())
	case "HH":
		return pad2(t.Hour())
	case "mm":
		return pad2(t.Minute())
	}
	return ""
}

// pad2 renders a two-digit clock/calendar number.
func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

// DatePattern returns ctx's locale's pattern for one shape, tokens intact. A
// renderer that formats dates in the browser needs the pattern itself, not a
// rendered date, and must keep the locale's own day/month order.
func DatePattern(ctx context.Context, f DateFormat) string {
	return From(ctx).dates.formats[f]
}

// MonthNamesShort returns ctx's locale's 12 short month names, January first.
// The calendar the day-range picker draws needs the names as data, not as a
// formatted date. The result is a copy, so a caller cannot write into the
// catalog.
func MonthNamesShort(ctx context.Context) []string {
	return slices.Clone(From(ctx).dates.monthsShort)
}

// WeekdayNamesShort returns ctx's locale's 7 short weekday names, Monday first.
// The result is a copy, for the same reason as MonthNamesShort.
func WeekdayNamesShort(ctx context.Context) []string {
	return slices.Clone(From(ctx).dates.weekdaysShort)
}
