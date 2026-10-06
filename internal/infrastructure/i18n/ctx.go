package i18n

import "context"

// localeKey namespaces the request locale in a context.
type localeKey struct{}

// WithLocale returns ctx carrying l. The request context is how views reach the
// locale: templ puts ctx in scope for the whole component body, so a view
// localizes without a printer threaded through every component signature.
func WithLocale(ctx context.Context, l Locale) context.Context {
	return context.WithValue(ctx, localeKey{}, l)
}

// From returns the locale ctx carries, English when it carries none, so a
// component rendered outside the request middleware degrades to English instead
// of panicking.
func From(ctx context.Context) Locale {
	l, ok := ctx.Value(localeKey{}).(Locale)
	if !ok || l.printer == nil {
		return English()
	}
	return l
}

// T renders the message for key in ctx's locale; args fill the message's verbs.
//
// A key missing from every catalog renders as the key itself; the reference guard
// in catalog_coverage_test.go is what keeps that off a page. Note that the
// printer localizes numbers (%d of 1000 is "1 000" under ru): where exact digits
// matter, pre-format with strconv and pass %s.
func T(ctx context.Context, key string, args ...any) string {
	return From(ctx).printer.Sprintf(key, args...)
}

// N renders key's plural form for n. n is always the message's first argument,
// which is what the one/few/many/other selector reads, so extra args start at
// %[2]s in the catalog.
func N(ctx context.Context, key string, n int, args ...any) string {
	return From(ctx).printer.Sprintf(key, append([]any{n}, args...)...)
}
