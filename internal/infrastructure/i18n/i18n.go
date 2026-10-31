// Package i18n localizes user-facing copy. Catalogs are plain JSON files under
// locales/, one per locale, embedded at build time and hand-editable by
// translators who write no Go (see locales/README.md). Every string the UI shows
// goes through T or N with a stable dotted key.
//
// English is both the source of truth and the automatic fallback: en.json is
// registered under language.Und as well as language.English, so a key a
// translator has not filled in yet renders English rather than leaking a raw key.
//
// It lives in infra, not common: translation is a presentation concern, and
// common is dependency-free by rule.
package i18n

import (
	"errors"
	"slices"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
)

// Locale is one supported language together with the printer that renders its
// catalog. The zero value renders nothing; take one from Supported, Lookup,
// Normalize, Match or From.
type Locale struct {
	tag      language.Tag
	code     string
	native   string
	og       string
	reviewed bool
	printer  *message.Printer
	dates    dateNames
}

// localeDef is the static, catalog-independent part of a supported locale.
type localeDef struct {
	tag    language.Tag
	code   string
	native string
	og     string
}

// supportedDefs lists the supported locales. English is first, which is what
// makes it the matcher's default and every fallback's answer. Adding a locale
// means adding a line here and a locales/<code>.json beside it.
var supportedDefs = []localeDef{
	{language.English, "en", "English", "en_US"},
	{language.Polish, "pl", "Polski", "pl_PL"},
}

var locales, byCode, matcher = buildCatalog()

// buildCatalog builds the catalog from the embedded JSON once. A malformed or
// unloadable catalog is a broken build artefact, not a runtime condition, so it
// panics here rather than degrading silently at the first render;
// catalog_coverage_test.go is what keeps that from reaching a binary.
func buildCatalog() ([]Locale, map[string]Locale, language.Matcher) {
	b := catalog.NewBuilder()
	files := make(map[string]catalogFile, len(supportedDefs))
	for _, def := range supportedDefs {
		f, err := loadFile(localesFS, "locales/"+def.code+".json")
		if err != nil {
			panic("i18n: " + err.Error())
		}
		files[def.code] = f
		if err := addMessages(b, def.tag, f.Messages); err != nil {
			panic("i18n: " + err.Error())
		}
		// English doubles as the fallback catalog. catalog's lookup walks
		// tag.Parent() up to language.Und and never consults catalog.Fallback, so
		// registering en under Und is the whole of "a missing translation renders
		// English", with no per-key branching anywhere.
		if def.tag == language.English {
			if err := addMessages(b, language.Und, f.Messages); err != nil {
				panic("i18n: " + err.Error())
			}
		}
	}

	// English's date vocabulary is resolved first and complete; every other
	// locale's gaps fall back to it field by field.
	englishDates, err := resolveDates("en", files["en"].Dates, dateNames{})
	if err != nil {
		panic("i18n: " + err.Error())
	}

	tags := make([]language.Tag, 0, len(supportedDefs))
	built := make([]Locale, 0, len(supportedDefs))
	byName := make(map[string]Locale, len(supportedDefs))
	for _, def := range supportedDefs {
		dates, err := resolveDates(def.code, files[def.code].Dates, englishDates)
		if err != nil {
			panic("i18n: " + err.Error())
		}
		l := Locale{
			tag:    def.tag,
			code:   def.code,
			native: def.native,
			og:     def.og,
			dates:  dates,
			// English is authored, not translated, so its reviewed_by is what marks
			// it reviewed; every other catalog stays unreviewed until a native
			// speaker signs off in the file.
			reviewed: len(files[def.code].ReviewedBy) > 0,
			printer:  message.NewPrinter(def.tag, message.Catalog(b)),
		}
		built = append(built, l)
		byName[def.code] = l
		tags = append(tags, def.tag)
	}
	return built, byName, language.NewMatcher(tags)
}

// Supported returns the locales the app offers, English first. The order is the
// order the language picker renders.
func Supported() []Locale { return slices.Clone(locales) }

// English is the source locale and every fallback's answer.
func English() Locale { return locales[0] }

// Lookup resolves a language code ("pl", "pl-PL", "PL", "pl_PL") to a supported
// locale. ok is false for an empty or unsupported value, so a caller reading an
// untrusted cookie or query value can fall through to the next source instead of
// pinning English.
func Lookup(code string) (Locale, bool) {
	tag, err := parseCode(code)
	if err != nil {
		return Locale{}, false
	}
	base, _ := tag.Base()
	l, ok := byCode[base.String()]
	return l, ok
}

// Normalize resolves a language code to a supported locale, falling back to
// English. Use it where an unsupported value must not be an error (the switch
// form's untrusted "lang" field).
func Normalize(code string) Locale {
	if l, ok := Lookup(code); ok {
		return l
	}
	return English()
}

// Match picks the best supported locale for an Accept-Language header, English
// when nothing in it is supported or the header is unusable. The header is the
// visitor's declared preference, which is why it, and not a geolocated guess,
// is the default (see locales/README.md).
func Match(acceptLanguage string) Locale {
	_, idx := language.MatchStrings(matcher, acceptLanguage)
	if idx < 0 || idx >= len(locales) {
		return English()
	}
	return locales[idx]
}

// parseCode parses a language code, tolerating the underscore form some clients
// send ("ru_RU").
func parseCode(code string) (language.Tag, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(code), "_", "-")
	if normalized == "" {
		return language.Und, errors.New("empty language code")
	}
	return language.Parse(normalized)
}

// Code is the short language code ("en", "pl"): the cookie
// value, the stored users.language, and the picker's form field.
func (l Locale) Code() string { return l.code }

// HTMLLang is the value for the document's lang attribute.
func (l Locale) HTMLLang() string { return l.code }

// OpenGraph is the og:locale value ("ru_RU"), which wants a language_REGION pair
// rather than a bare code.
func (l Locale) OpenGraph() string { return l.og }

// NativeName is how the language names itself, so the picker reads for someone
// who cannot read the current one.
func (l Locale) NativeName() string { return l.native }

// Reviewed reports whether a native speaker has signed off on this catalog
// (locales/<code>.json "reviewed_by"). False drives the picker's honest "help us
// improve this translation" line instead of pretending machine-authored copy is
// authoritative.
func (l Locale) Reviewed() bool { return l.reviewed }
