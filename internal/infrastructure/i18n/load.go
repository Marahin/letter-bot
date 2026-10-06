package i18n

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
	"golang.org/x/text/message/catalog"
)

//go:embed locales/*.json
var localesFS embed.FS

// catalogFile is one locales/<code>.json. status and reviewed_by carry the
// translation's provenance: reviewed_by stays empty until a native speaker signs
// off, which is what Locale.Reviewed reports.
//
// dates is deliberately outside messages: month and weekday names are structural
// date data, not copy, and keeping them out of messages keeps them out of the
// key-reference guard in catalog_coverage_test.go (nothing names them by key).
type catalogFile struct {
	Locale     string                     `json:"locale"`
	Status     string                     `json:"status"`
	ReviewedBy []string                   `json:"reviewed_by"`
	Messages   map[string]json.RawMessage `json:"messages"`
	Dates      dateSection                `json:"dates"`
}

// dateSection is a catalog's "dates" block. Every field is optional in a
// translation and falls back to English field by field, so a translator can ship
// month names before the patterns (see dateNames).
type dateSection struct {
	MonthsShort   []string          `json:"months_short"`
	WeekdaysShort []string          `json:"weekdays_short"`
	Formats       map[string]string `json:"formats"`
}

// pluralForms are the CLDR plural categories a message may spell out, tried in
// this order. Which of them a language actually uses is CLDR's business, not
// ours: x/text/feature/plural carries the tables, so a Slavic catalog and a
// two-form one like tr select correctly from whatever forms the file provides.
var pluralForms = []struct {
	name string
	form plural.Form
}{
	{"zero", plural.Zero},
	{"one", plural.One},
	{"two", plural.Two},
	{"few", plural.Few},
	{"many", plural.Many},
	{"other", plural.Other},
}

// loadFile reads and parses one catalog file.
func loadFile(fsys fs.FS, name string) (catalogFile, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return catalogFile{}, err
	}
	return parseFile(name, raw)
}

// parseFile decodes a catalog file, refusing unknown top-level fields so a
// translator's typo is an error rather than a silently dropped section.
func parseFile(name string, raw []byte) (catalogFile, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var f catalogFile
	if err := dec.Decode(&f); err != nil {
		return catalogFile{}, fmt.Errorf("%s: %w", name, err)
	}
	return f, nil
}

// addMessages registers a catalog file's messages for tag. A string value is a
// plain message; an object value spells out plural forms and becomes a selector
// on the message's first argument (see N).
func addMessages(b *catalog.Builder, tag language.Tag, msgs map[string]json.RawMessage) error {
	for _, key := range slices.Sorted(maps.Keys(msgs)) {
		var s string
		if err := json.Unmarshal(msgs[key], &s); err == nil {
			if err := b.SetString(tag, key, s); err != nil {
				return fmt.Errorf("%s %q: %w", tag, key, err)
			}
			continue
		}
		var forms map[string]string
		if err := json.Unmarshal(msgs[key], &forms); err != nil {
			return fmt.Errorf("%s %q: must be a string or an object of plural forms", tag, key)
		}
		msg, err := pluralSelector(tag, key, forms)
		if err != nil {
			return err
		}
		if err := b.Set(tag, key, msg); err != nil {
			return fmt.Errorf("%s %q: %w", tag, key, err)
		}
	}
	return nil
}

// pluralSelector builds one plural message. "other" is mandatory: it is the form
// x/text falls back to, so a file without it would render nothing for the
// categories it omitted.
func pluralSelector(tag language.Tag, key string, forms map[string]string) (catalog.Message, error) {
	if strings.TrimSpace(forms["other"]) == "" {
		return nil, fmt.Errorf(`%s %q: plural message needs an "other" form`, tag, key)
	}
	cases := make([]any, 0, 2*len(forms))
	for _, f := range pluralForms {
		v, ok := forms[f.name]
		if !ok {
			continue
		}
		// language.Und carries only the "other" category, so the English copy
		// registered under it as the fallback (see buildCatalog) keeps just that
		// form; every other form would be rejected outright.
		if tag == language.Und && f.form != plural.Other {
			continue
		}
		cases = append(cases, f.form, v)
	}
	if tag != language.Und && len(cases) != 2*len(forms) {
		return nil, fmt.Errorf("%s %q: unknown plural form (allowed: zero, one, two, few, many, other)", tag, key)
	}
	// The selector reads argument 1 formatted with %d, which is where N puts n, so
	// every form's own verbs address the same argument list.
	return plural.Selectf(1, "%d", cases...), nil
}
