package i18n

import (
	"context"
	"time"
)

// Day and larger buckets are calendar-length approximations (no DST, months of
// 30 days): Ago labels an age, not an interval anyone measures against.
const (
	day   = 24 * time.Hour
	week  = 7 * day
	month = 30 * day
	year  = 365 * day
)

// Ago renders how long before now t was, in ctx's locale ("5 minutes ago").
//
// Each unit is a whole phrase in the catalog rather than a count composed with a
// shared "%s ago", because Slavic languages inflect the noun for the phrase: ru
// wants the accusative "1 минуту назад" against the nominative "1 минута", so
// gluing a plural count to a suffix produces wrong grammar for n = 1.
//
// Anything under a minute reads as "just now", and so does a t in the future:
// callers label recorded pasts (a last-synced stamp), where a future instant is
// clock skew, not a fact worth phrasing.
func Ago(ctx context.Context, t, now time.Time) string {
	switch d := now.Sub(t); {
	case d < time.Minute:
		return T(ctx, "time.relative.now")
	case d < time.Hour:
		return N(ctx, "time.relative.minutes_ago", int(d/time.Minute))
	case d < day:
		return N(ctx, "time.relative.hours_ago", int(d/time.Hour))
	case d < week:
		return N(ctx, "time.relative.days_ago", int(d/day))
	case d < month:
		return N(ctx, "time.relative.weeks_ago", int(d/week))
	case d < year:
		return N(ctx, "time.relative.months_ago", int(d/month))
	default:
		return N(ctx, "time.relative.years_ago", int(d/year))
	}
}

// Until renders how long after now t is, in ctx's locale ("in 6 hours").
//
// It carries its own catalog keys instead of gluing a prefix onto Ago's, for the
// reason Ago documents: the phrase inflects. ru wants the accusative "через 1
// минуту", pl "za 1 minutę", and de puts the preposition first ("in 1 Minute"),
// so no shared affix would render.
//
// Anything under a minute ahead reads as "any moment now", and so does a t before
// now: callers label a scheduled future, where a past instant is a stale render,
// not a fact worth phrasing.
func Until(ctx context.Context, t, now time.Time) string {
	switch d := t.Sub(now); {
	case d < time.Minute:
		return T(ctx, "time.relative.soon")
	case d < time.Hour:
		return N(ctx, "time.relative.in_minutes", int(d/time.Minute))
	case d < day:
		return N(ctx, "time.relative.in_hours", int(d/time.Hour))
	case d < week:
		return N(ctx, "time.relative.in_days", int(d/day))
	case d < month:
		return N(ctx, "time.relative.in_weeks", int(d/week))
	case d < year:
		return N(ctx, "time.relative.in_months", int(d/month))
	default:
		return N(ctx, "time.relative.in_years", int(d/year))
	}
}
