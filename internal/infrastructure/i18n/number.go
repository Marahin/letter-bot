package i18n

import (
	"context"

	"golang.org/x/text/number"
)

// Int formats n with the locale's digit grouping ("1,234,567" in en, "1 234 567" in pl).
func Int(ctx context.Context, n int64) string {
	return From(ctx).printer.Sprint(number.Decimal(n))
}

// Decimal formats v with exactly decimals fraction digits and the locale's separators.
func Decimal(ctx context.Context, v float64, decimals int) string {
	return From(ctx).printer.Sprint(number.Decimal(v, number.MinFractionDigits(decimals), number.MaxFractionDigits(decimals)))
}
