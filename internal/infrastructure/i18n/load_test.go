package i18n

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/message/catalog"
)

// buildFrom assembles a printer per locale the way init does, from catalog files
// given inline: English under both English and Und (the fallback registration),
// everything else under its own tag.
func buildFrom(t *testing.T, files map[language.Tag]string) map[language.Tag]*message.Printer {
	t.Helper()
	b := catalog.NewBuilder()
	for tag, raw := range files {
		f, err := parseFile(tag.String()+".json", []byte(raw))
		require.NoError(t, err)
		require.NoError(t, addMessages(b, tag, f.Messages))
		if tag == language.English {
			require.NoError(t, addMessages(b, language.Und, f.Messages))
		}
	}
	printers := make(map[language.Tag]*message.Printer, len(files))
	for tag := range files {
		printers[tag] = message.NewPrinter(tag, message.Catalog(b))
	}
	return printers
}

// TestEnglishFallback_MissingKeyRendersEnglish proves the language.Und
// registration, which is the whole mechanism behind "a partial translation is a
// safe state". catalog.Fallback would not do this: catalog's lookup walks
// tag.Parent() to Und and never consults it.
func TestEnglishFallback_MissingKeyRendersEnglish(t *testing.T) {
	// given a Russian catalog that translated one key and left the other out
	printers := buildFrom(t, map[language.Tag]string{
		language.English: `{"locale":"en","messages":{
			"a.translated":"Translated",
			"a.missing":"Only in English",
			"a.counted":{"one":"%d thing","other":"%d things"}
		}}`,
		language.Russian: `{"locale":"ru","messages":{"a.translated":"Переведено"}}`,
	})
	ru := printers[language.Russian]

	// then the translated key is Russian, the untranslated one falls back to
	// English, and no raw key ever reaches the output
	assert.Equal(t, "Переведено", ru.Sprintf("a.translated"))
	assert.Equal(t, "Only in English", ru.Sprintf("a.missing"))
	// including a plural: Und carries only "other", which is the sane fallback
	assert.Equal(t, "5 things", ru.Sprintf("a.counted", 5))
}

func TestAddMessages_PluralWithoutOtherIsRejected(t *testing.T) {
	// given a plural message missing the mandatory "other" form
	f, err := parseFile("ru.json", []byte(`{"locale":"ru","messages":{"a.count":{"one":"%d штука"}}}`))
	require.NoError(t, err)

	// when it is registered
	err = addMessages(catalog.NewBuilder(), language.Russian, f.Messages)

	// then it fails loudly, naming the key
	require.Error(t, err)
	assert.Contains(t, err.Error(), "a.count")
	assert.Contains(t, err.Error(), `"other"`)
}

func TestAddMessages_UnknownPluralFormIsRejected(t *testing.T) {
	f, err := parseFile("ru.json", []byte(`{"locale":"ru","messages":{"a.count":{"other":"%d","plenty":"%d"}}}`))
	require.NoError(t, err)

	err = addMessages(catalog.NewBuilder(), language.Russian, f.Messages)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown plural form")
}

func TestAddMessages_NonStringNonObjectValueIsRejected(t *testing.T) {
	f, err := parseFile("ru.json", []byte(`{"locale":"ru","messages":{"a.count":42}}`))
	require.NoError(t, err)

	err = addMessages(catalog.NewBuilder(), language.Russian, f.Messages)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be a string or an object of plural forms")
}

func TestParseFile_MalformedAndUnknownFields(t *testing.T) {
	// given a file a translator broke
	for name, raw := range map[string]string{
		"truncated JSON":     `{"locale":"ru","messages":{`,
		"a mistyped section": `{"locale":"ru","mesages":{"a":"b"}}`,
		"the wrong top type": `["ru"]`,
	} {
		_, err := parseFile("ru.json", []byte(raw))
		require.Errorf(t, err, "%s should not load", name)
		assert.Containsf(t, err.Error(), "ru.json", "%s: the error names the file", name)
	}
}

func TestLoadFile_MissingFile(t *testing.T) {
	_, err := loadFile(fstest.MapFS{}, "locales/ru.json")
	require.Error(t, err)
}

func TestShippedCatalogs_LoadAndDeclareTheirOwnLocale(t *testing.T) {
	// given each embedded catalog
	for _, def := range supportedDefs {
		f, err := loadFile(localesFS, "locales/"+def.code+".json")
		require.NoErrorf(t, err, "%s.json", def.code)

		// then it names itself (a copy-pasted file with the wrong "locale" is the
		// easy mistake) and carries messages
		assert.Equalf(t, def.code, f.Locale, "%s.json declares its own locale", def.code)
		assert.NotEmptyf(t, f.Messages, "%s.json has messages", def.code)
		assert.NotEmptyf(t, f.Status, "%s.json records its provenance", def.code)
	}
}

func TestWithLocale_RoundTrips(t *testing.T) {
	for _, l := range Supported() {
		assert.Equal(t, l.Code(), From(WithLocale(context.Background(), l)).Code())
	}
}
