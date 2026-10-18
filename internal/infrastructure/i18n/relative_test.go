package i18n

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRelativeTime_SlavicPlurals is the reason this helper exists instead of
// go-humanize: the counts each language groups differently, and the noun each
// phrase inflects. The expectations are hand-written per language, not derived
// from the code. The phrases are read straight from the catalog rather than
// through Ago, whose buckets would roll 101 minutes into hours.
//
// 1, 21 and 101 are "one" in ru/uk (and so take the accusative singular "минуту")
// but "many" in pl; 2 and 4 are "few"; 5 and 11 are "many" everywhere.
func TestRelativeTime_SlavicPlurals(t *testing.T) {
	counts := []int{1, 2, 4, 5, 11, 21, 101}

	for code, want := range map[string][]string{
		"en": {
			"1 minute ago", "2 minutes ago", "4 minutes ago",
			"5 minutes ago", "11 minutes ago", "21 minutes ago", "101 minutes ago",
		},
		"pl": {
			"1 minutę temu", "2 minuty temu", "4 minuty temu",
			"5 minut temu", "11 minut temu", "21 minut temu", "101 minut temu",
		},
	} {
		ctx := WithLocale(context.Background(), Normalize(code))
		got := make([]string, 0, len(counts))
		for _, n := range counts {
			got = append(got, N(ctx, "time.relative.minutes_ago", n))
		}
		require.Equalf(t, want, got, "%s minute plurals", code)
	}
}

// TestRelativeTime_SingularIsAccusative is why each unit is a whole phrase in the
// catalog rather than a count glued to a shared "%s ago": ru/uk/pl put the noun in
// the accusative here, which differs from the nominative for the feminine ones
// (минута -> минуту, неделя -> неделю, minuta -> minutę).
func TestRelativeTime_SingularIsAccusative(t *testing.T) {
	units := []string{
		"time.relative.minutes_ago",
		"time.relative.hours_ago",
		"time.relative.days_ago",
		"time.relative.weeks_ago",
		"time.relative.months_ago",
		"time.relative.years_ago",
	}
	for code, want := range map[string][]string{
		"pl": {"1 minutę temu", "1 godzinę temu", "1 dzień temu", "1 tydzień temu", "1 miesiąc temu", "1 rok temu"},
	} {
		ctx := WithLocale(context.Background(), Normalize(code))
		got := make([]string, 0, len(units))
		for _, key := range units {
			got = append(got, N(ctx, key, 1))
		}
		require.Equalf(t, want, got, "%s singulars", code)
	}
}

// TestAgo_UnitBuckets pins where each unit takes over, including the boundaries a
// refactor slides by one.
func TestAgo_UnitBuckets(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	for _, tc := range []struct {
		ago  time.Duration
		want string
	}{
		{0, "just now"},
		{59 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{59 * time.Minute, "59 minutes ago"},
		{time.Hour, "1 hour ago"},
		{23 * time.Hour, "23 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{6 * 24 * time.Hour, "6 days ago"},
		{7 * 24 * time.Hour, "1 week ago"},
		{29 * 24 * time.Hour, "4 weeks ago"},
		{30 * 24 * time.Hour, "1 month ago"},
		{364 * 24 * time.Hour, "12 months ago"},
		{365 * 24 * time.Hour, "1 year ago"},
		{800 * 24 * time.Hour, "2 years ago"},
	} {
		assert.Equalf(t, tc.want, Ago(ctx, now.Add(-tc.ago), now), "%s ago", tc.ago)
	}
}

// TestAgo_FutureReadsAsJustNow: callers label recorded pasts, so a t after now is
// clock skew. It must not render "-3 minutes ago".
func TestAgo_FutureReadsAsJustNow(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	ctx := WithLocale(context.Background(), Normalize("pl"))
	assert.Equal(t, "przed chwilą", Ago(ctx, now.Add(time.Hour), now))
}

func TestAgo_LargerUnitsPerLocale(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	for code, want := range map[string]string{
		"pl": "5 dni temu",
	} {
		ctx := WithLocale(context.Background(), Normalize(code))
		assert.Equalf(t, want, Ago(ctx, now.Add(-5*24*time.Hour), now), "%s days", code)
	}
}

// TestUntil_UnitBuckets pins where each forward unit takes over, including the
// boundaries a refactor slides by one.
func TestUntil_UnitBuckets(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	for _, tc := range []struct {
		ahead time.Duration
		want  string
	}{
		{0, "any moment now"},
		{59 * time.Second, "any moment now"},
		{time.Minute, "in 1 minute"},
		{59 * time.Minute, "in 59 minutes"},
		{time.Hour, "in 1 hour"},
		{23 * time.Hour, "in 23 hours"},
		{24 * time.Hour, "in 1 day"},
		{6 * 24 * time.Hour, "in 6 days"},
		{7 * 24 * time.Hour, "in 1 week"},
		{29 * 24 * time.Hour, "in 4 weeks"},
		{30 * 24 * time.Hour, "in 1 month"},
		{364 * 24 * time.Hour, "in 12 months"},
		{365 * 24 * time.Hour, "in 1 year"},
		{800 * 24 * time.Hour, "in 2 years"},
	} {
		assert.Equalf(t, tc.want, Until(ctx, now.Add(tc.ahead), now), "in %s", tc.ahead)
	}
}

// TestUntil_PastReadsAsAnyMoment: callers label a scheduled future, so a t before
// now is a stale render. It must not read "in -3 minutes".
func TestUntil_PastReadsAsAnyMoment(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	ctx := WithLocale(context.Background(), Normalize("pl"))
	assert.Equal(t, "za chwilę", Until(ctx, now.Add(-time.Hour), now))
}

// TestUntil_EveryUnitIsTranslatedInEveryLocale is the parity the catalog walkers
// cannot give: a catalog missing a key renders English, not the raw key, so only
// the words prove the translation arrived. One count per unit, in both locales.
func TestUntil_EveryUnitIsTranslatedInEveryLocale(t *testing.T) {
	now := time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC)
	// Sub-minute, then 5 of each unit except weeks, where 5 weeks would already be
	// a month: 21 days reads as 3 weeks.
	ahead := []time.Duration{
		30 * time.Second, 5 * time.Minute, 5 * time.Hour,
		5 * 24 * time.Hour, 21 * 24 * time.Hour, 150 * 24 * time.Hour, 5 * 365 * 24 * time.Hour,
	}
	for code, want := range map[string][]string{
		"en": {
			"any moment now", "in 5 minutes", "in 5 hours",
			"in 5 days", "in 3 weeks", "in 5 months", "in 5 years",
		},
		"pl": {
			"za chwilę", "za 5 minut", "za 5 godzin",
			"za 5 dni", "za 3 tygodnie", "za 5 miesięcy", "za 5 lat",
		},
	} {
		ctx := WithLocale(context.Background(), Normalize(code))
		got := make([]string, 0, len(ahead))
		for _, d := range ahead {
			got = append(got, Until(ctx, now.Add(d), now))
		}
		require.Equalf(t, want, got, "%s forward units", code)
	}
}

// TestUntil_SingularIsAccusative is why the forward phrases are their own catalog
// keys rather than "in" glued to Ago's: ru and uk take "через" plus the
// accusative, pl "za" plus the accusative, and the feminine nouns differ from
// their nominative there (минута -> минуту, tydzień stays, minuta -> minutę).
func TestUntil_SingularIsAccusative(t *testing.T) {
	units := []string{
		"time.relative.in_minutes",
		"time.relative.in_hours",
		"time.relative.in_days",
		"time.relative.in_weeks",
		"time.relative.in_months",
		"time.relative.in_years",
	}
	for code, want := range map[string][]string{
		"pl": {"za 1 minutę", "za 1 godzinę", "za 1 dzień", "za 1 tydzień", "za 1 miesiąc", "za 1 rok"},
	} {
		ctx := WithLocale(context.Background(), Normalize(code))
		got := make([]string, 0, len(units))
		for _, key := range units {
			got = append(got, N(ctx, key, 1))
		}
		require.Equalf(t, want, got, "%s forward singulars", code)
	}
}

// TestUntil_SlavicPlurals reads the minute phrase straight from the catalog at the
// counts each language groups differently, as the Ago counterpart does: 1, 21 and
// 101 are "one" in ru/uk but "many" in pl; 2 and 4 are "few"; 5 and 11 are "many"
// everywhere.
func TestUntil_SlavicPlurals(t *testing.T) {
	counts := []int{1, 2, 4, 5, 11, 21, 101}

	for code, want := range map[string][]string{
		"en": {
			"in 1 minute", "in 2 minutes", "in 4 minutes",
			"in 5 minutes", "in 11 minutes", "in 21 minutes", "in 101 minutes",
		},
		"pl": {
			"za 1 minutę", "za 2 minuty", "za 4 minuty",
			"za 5 minut", "za 11 minut", "za 21 minut", "za 101 minut",
		},
	} {
		ctx := WithLocale(context.Background(), Normalize(code))
		got := make([]string, 0, len(counts))
		for _, n := range counts {
			got = append(got, N(ctx, "time.relative.in_minutes", n))
		}
		require.Equalf(t, want, got, "%s forward minute plurals", code)
	}
}
