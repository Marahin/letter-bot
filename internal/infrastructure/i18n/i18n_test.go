package i18n

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestSupported_EnglishFirst(t *testing.T) {
	// given the supported set
	got := Supported()

	// then it is exactly English and Polish, English first so it is the matcher's
	// and every fallback's default
	codes := make([]string, 0, len(got))
	for _, l := range got {
		codes = append(codes, l.Code())
	}
	assert.Equal(t, []string{"en", "pl"}, codes)
	assert.Equal(t, "en", English().Code())
}

func TestLocaleMetadata(t *testing.T) {
	for code, want := range map[string]struct{ html, og, native string }{
		"en": {"en", "en_US", "English"},
		"pl": {"pl", "pl_PL", "Polski"},
	} {
		l := Normalize(code)
		assert.Equal(t, want.html, l.HTMLLang(), "%s: html lang", code)
		assert.Equal(t, want.og, l.OpenGraph(), "%s: og:locale", code)
		assert.Equal(t, want.native, l.NativeName(), "%s: native name", code)
	}
}

func TestReviewed_OnlyEnglishIsSignedOff(t *testing.T) {
	// given the shipped catalogs: English is authored, Polish has an empty reviewed_by
	assert.True(t, English().Reviewed(), "the source catalog counts as reviewed")
	assert.False(t, Normalize("pl").Reviewed(), "pl is unreviewed until a native speaker fills in reviewed_by")
}

func TestLookup(t *testing.T) {
	for _, tc := range []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"pl", "pl", true},
		{"PL", "pl", true},    // case-insensitive
		{"pl-PL", "pl", true}, // region stripped to the base language
		{"pl_PL", "pl", true}, // the underscore form some clients send
		{" pl ", "pl", true},  // trimmed
		{"en", "en", true},
		{"en-GB", "en", true},
		{"", "", false},          // no value: fall through to the next source
		{"xx", "", false},        // not a language
		{"ru", "", false},        // a language we do not ship
		{"../../etc", "", false}, // junk from an untrusted cookie
	} {
		got, ok := Lookup(tc.in)
		assert.Equalf(t, tc.wantOK, ok, "Lookup(%q) ok", tc.in)
		assert.Equalf(t, tc.want, got.Code(), "Lookup(%q) code", tc.in)
	}
}

func TestNormalize_UnsupportedFallsBackToEnglish(t *testing.T) {
	// given values Lookup refuses, Normalize answers English rather than an error,
	// which is what the switch form's untrusted "lang" field needs
	for _, in := range []string{"", "xx", "zh", "ja-JP", "!!!"} {
		assert.Equalf(t, "en", Normalize(in).Code(), "Normalize(%q)", in)
	}
	assert.Equal(t, "pl", Normalize("pl-PL").Code())
}

func TestMatch_AcceptLanguageHeaders(t *testing.T) {
	for _, tc := range []struct{ header, want string }{
		{"pl-PL,pl;q=0.9,en;q=0.8", "pl"},
		{"pl", "pl"},
		{"en-GB,en;q=0.9", "en"},
		{"ja,fr", "en"},           // nothing supported: English
		{"", "en"},                // no header at all
		{"pl;q=notanumber", "en"}, // malformed: no error escapes, English
		{"zh-Hans-CN,zh;q=0.9", "en"},
		{"en;q=0.5,pl;q=0.9", "pl"}, // quality order, not document order
	} {
		assert.Equalf(t, tc.want, Match(tc.header).Code(), "Match(%q)", tc.header)
	}
}

func TestFrom_NoLocaleInContextIsEnglish(t *testing.T) {
	// given a context that never passed through the middleware
	ctx := context.Background()

	// then reads degrade to English rather than panicking, which is what makes a
	// view rendered outside a request safe
	assert.Equal(t, "en", From(ctx).Code())
	assert.Equal(t, "Reservations", T(ctx, "shell.nav.reservations"))
	assert.Equal(t, "2 servers", N(ctx, "shell.switcher.server_count", 2))

	// and so does a context carrying a zero Locale (never constructed by us, but
	// context.WithValue cannot refuse one)
	zero := context.WithValue(ctx, localeKey{}, Locale{})
	assert.Equal(t, "en", From(zero).Code())
}

func TestT_LocalizesAndInterpolates(t *testing.T) {
	// given
	ctx := WithLocale(context.Background(), Normalize("pl"))

	// then
	assert.Equal(t, "Rezerwacje", T(ctx, "shell.nav.reservations"))
	assert.Equal(t, "Język: Polski", T(ctx, "shell.language.current", "Polski"))
}

// TestN_PolishPluralForms is the one thing about this package that cannot be
// eyeballed: which of one/few/many CLDR picks for each count. The expectations are
// hand-written per language, not derived from the code.
func TestN_PolishPluralForms(t *testing.T) {
	counts := []int{0, 1, 2, 4, 5, 11, 21, 22, 101}
	for code, want := range map[string][]string{
		"en": {"0 servers", "1 server", "2 servers", "4 servers", "5 servers", "11 servers", "21 servers", "22 servers", "101 servers"},
		// Polish differs from Russian at 21: "many", not "one".
		"pl": {"0 serwerów", "1 serwer", "2 serwery", "4 serwery", "5 serwerów", "11 serwerów", "21 serwerów", "22 serwery", "101 serwerów"},
	} {
		ctx := WithLocale(context.Background(), Normalize(code))
		got := make([]string, 0, len(counts))
		for _, n := range counts {
			got = append(got, N(ctx, "shell.switcher.server_count", n))
		}
		require.Equalf(t, want, got, "%s plural forms", code)
	}
}

// TestN_OneOtherBoundary pins where CLDR switches category for a two-form language.
// The forms here are deliberately distinguishable, so an x/text bump that moved the
// boundary fails here instead of passing unnoticed.
func TestN_OneOtherBoundary(t *testing.T) {
	printers := buildFrom(t, map[language.Tag]string{
		language.English: `{"locale":"en","messages":{"a.counted":{"one":"one","other":"other"}}}`,
	})
	en := printers[language.English]

	assert.Equal(t, "one", en.Sprintf("a.counted", 1))
	for _, n := range []int{0, 2, 5, 11, 21, 101} {
		assert.Equalf(t, "other", en.Sprintf("a.counted", n), "n=%d", n)
	}
}

// TestN_LocalizesNumberGrouping records the printer behaviour callers have to know
// about: a count is formatted for the locale, so exact digits (ids, table columns)
// must be pre-formatted with strconv and passed as %s instead.
func TestN_LocalizesNumberGrouping(t *testing.T) {
	ctx := WithLocale(context.Background(), Normalize("pl"))
	// The group separator is a non-breaking space under pl; spelled out so the
	// expectation is not an invisible character in the source.
	assert.Equal(t, "15\u00a0000 serwerów", N(ctx, "shell.switcher.server_count", 15000))
}
