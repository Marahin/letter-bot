package i18n

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// keyRef matches a literal call site, i18n.T(ctx, "some.key") or
// i18n.N(r.Context(), "some.key", n), in Go and .templ sources. The context
// expression is matched loosely (anything without a comma or a quote) so both
// spellings count; the key itself must be a literal, which is exactly why keys
// are never built at runtime.
var keyRef = regexp.MustCompile(`i18n\.[TN]\([^,"]*,\s*"([^"]+)"`)

// selfKeyRef matches the same call sites inside this package, where T and N are
// unqualified (relative.go's Ago picks one plural key per unit). Both patterns run
// over every file and their results are unioned: outside this package the same
// text also matches keyRef, so the overlap is harmless.
var selfKeyRef = regexp.MustCompile(`\b[TN]\(ctx, "([^"]+)"`)

// repoRoot is this package's path back to the module root.
const repoRoot = "../../.."

// catalogKeys reads one shipped catalog's key set.
func catalogKeys(t *testing.T, code string) map[string]bool {
	t.Helper()
	f, err := loadFile(localesFS, "locales/"+code+".json")
	require.NoError(t, err)
	keys := make(map[string]bool, len(f.Messages))
	for k := range f.Messages {
		keys[k] = true
	}
	return keys
}

// TestCatalogs_NoKeysEnglishDoesNotHave guards against orphans: a key only a
// translation has can never render, so it is either a typo or a leftover. The
// reverse is deliberately not checked, a partial translation is legal and falls
// back to English.
func TestCatalogs_NoKeysEnglishDoesNotHave(t *testing.T) {
	english := catalogKeys(t, "en")
	for _, code := range []string{"pl"} {
		for key := range catalogKeys(t, code) {
			assert.Truef(t, english[key],
				"%s.json has %q, which en.json does not: a key only a translation has never renders", code, key)
		}
	}
}

// TestCatalogs_ReferencedKeysExistAndNoneAreOrphaned is the substitute for a
// compiler check on message keys, and the thing that stops catalog rot in both
// directions: a key the code asks for but no catalog has would render as the raw
// key on a page, and a key nothing references is dead copy translators keep paying
// for.
func TestCatalogs_ReferencedKeysExistAndNoneAreOrphaned(t *testing.T) {
	referenced := referencedKeys(t)
	require.NotEmpty(t, referenced, "the walker found no call sites at all, which means it is broken, not that there are none")

	english := catalogKeys(t, "en")
	for key, where := range referenced {
		assert.Truef(t, english[key], "%s references %q, which en.json does not define", where, key)
	}
	for key := range english {
		assert.Truef(t, referenced[key] != "", "en.json defines %q, which nothing references", key)
	}
}

// referencedKeys walks the tree for literal T/N call sites, mapping each key to
// the first file that used it. Test files are skipped: a test may legitimately
// name a key that only exists inside it.
func referencedKeys(t *testing.T) map[string]string {
	t.Helper()
	found := make(map[string]string)
	err := filepath.WalkDir(filepath.Join(repoRoot, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		name := d.Name()
		switch {
		case strings.HasSuffix(name, "_test.go"):
			return nil
		// _templ.go is generated from the .templ beside it; counting one is enough.
		case strings.HasSuffix(name, "_templ.go"):
			return nil
		case strings.HasSuffix(name, ".go"), strings.HasSuffix(name, ".templ"):
		default:
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, re := range []*regexp.Regexp{keyRef, selfKeyRef} {
			for _, m := range re.FindAllStringSubmatch(string(raw), -1) {
				if _, seen := found[m[1]]; !seen {
					found[m[1]] = path
				}
			}
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

// formatVerb matches a printf verb, including the %% escape for a literal percent.
var formatVerb = regexp.MustCompile(`%(?:%|\[\d+\][sdvqtfxX]|[sdvqtfxX])`)

// TestCatalogs_NoBareLiteralPercent guards the trap every catalog value sits in: a
// value is a printf format, so a lone "%" renders as "%!(NOVERB)" and swallows the
// next few characters. A percent sign a string genuinely needs is written "%%".
// Cheap to get wrong and invisible until someone reads the page, which is why it is
// checked statically rather than by rendering.
func TestCatalogs_NoBareLiteralPercent(t *testing.T) {
	for _, def := range supportedDefs {
		f, err := loadFile(localesFS, "locales/"+def.code+".json")
		require.NoError(t, err)
		for key, msg := range f.Messages {
			for form, value := range formsOf(msg) {
				rest := formatVerb.ReplaceAllString(value, "")
				assert.NotContainsf(t, rest, "%",
					"%s.json %q%s has a bare %%: write %%%% for a literal percent sign (%q)",
					def.code, key, form, value)
			}
		}
	}
}

// verbIndex matches the explicit argument index of a printf verb, "%[2]d" -> "%d".
var verbIndex = regexp.MustCompile(`%\[\d+\]`)

// verbsOf is the sorted multiset of printf verbs in value, with argument indices stripped and
// the "%%" escape dropped, so a reordered translation compares equal to its source.
func verbsOf(value string) []string {
	var out []string
	for _, v := range formatVerb.FindAllString(value, -1) {
		if v == "%%" {
			continue
		}
		out = append(out, verbIndex.ReplaceAllString(v, "%"))
	}
	slices.Sort(out)
	return out
}

// TestCatalogs_VerbsMatchEnglish guards the argument list the code passes: a translation with a
// verb English lacks prints "%!d(MISSING)", one short of a verb drops a figure. Keys a locale
// leaves out fall back to English by design and are skipped. A plural form is compared with the
// English form of the same category, or with English "other" when English lacks the category.
func TestCatalogs_VerbsMatchEnglish(t *testing.T) {
	english, err := loadFile(localesFS, "locales/en.json")
	require.NoError(t, err)
	for _, def := range supportedDefs {
		if def.code == "en" {
			continue
		}
		f, err := loadFile(localesFS, "locales/"+def.code+".json")
		require.NoError(t, err)
		for key, msg := range f.Messages {
			src, ok := english.Messages[key]
			if !ok {
				continue
			}
			srcForms := formsOf(src)
			for form, value := range formsOf(msg) {
				want, ok := srcForms[form]
				if !ok {
					want = srcForms[" (other)"]
				}
				assert.Equalf(t, verbsOf(want), verbsOf(value),
					"%s.json %q%s must carry the printf verbs en.json has (%q vs %q)",
					def.code, key, form, value, want)
			}
		}
	}
}

// formsOf flattens a raw message to its renderable strings: one for a plain entry,
// one per plural category otherwise, labelled for the failure message.
func formsOf(raw json.RawMessage) map[string]string {
	var single string
	if json.Unmarshal(raw, &single) == nil {
		return map[string]string{"": single}
	}
	var plural map[string]string
	if json.Unmarshal(raw, &plural) != nil {
		return nil
	}
	out := make(map[string]string, len(plural))
	for cat, v := range plural {
		out[" ("+cat+")"] = v
	}
	return out
}

// messageNaming is the shape every message key must have: <area>.<component>.<thing>.
var messageNaming = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9][a-z0-9_]*)+$`)

// messageKeyLine matches a message key where it is written in the file. It insists
// on a dotted key so the undotted entries in the "dates" block (months_short,
// day_month_year, …) are not mistaken for message keys; TestCatalogs_KeysAreDotted
// checks the parsed map, which is what catches an undotted message key.
var messageKeyLine = regexp.MustCompile(`(?m)^\s{4}"([a-z0-9_]+(?:\.[a-z0-9_]+)+)":`)

// TestCatalogs_KeysAreSortedAndDotted keeps the files reviewable by hand: a
// translator diffing a merge request should see a change, not a reshuffle.
func TestCatalogs_KeysAreSortedAndDotted(t *testing.T) {
	for _, def := range supportedDefs {
		raw, err := localesFS.ReadFile("locales/" + def.code + ".json")
		require.NoError(t, err)

		var order []string
		for _, m := range messageKeyLine.FindAllStringSubmatch(string(raw), -1) {
			order = append(order, m[1])
		}
		require.NotEmptyf(t, order, "%s.json: no keys found at the expected indent", def.code)
		assert.Truef(t, slices.IsSorted(order), "%s.json keys are not in alphabetical order", def.code)

		f, err := loadFile(localesFS, "locales/"+def.code+".json")
		require.NoError(t, err)
		for key := range f.Messages {
			assert.Truef(t, messageNaming.MatchString(key),
				"%s.json key %q does not follow <area>.<component>.<thing>", def.code, key)
		}
	}
}
