package web

// This file hands catalog strings to the page's scripts. The Go side localizes
// through i18n.T; a script cannot, so the page that loads the script ships the
// strings it needs with it.

import (
	"context"
	"encoding/json"
	"io"

	"github.com/a-h/templ"
)

// TranslatedStrings renders one JSON island of catalog strings for the scripts on
// this page, keyed by the same dotted ids the Go side uses. dist/i18n.js merges
// every island in the document (later ones winning), so the shell and a feature
// page can each ship their own, and an htmx-swapped fragment can bring more.
//
// Call it next to the <script> it feeds, and pass the values through i18n.T so the
// keys stay literal for catalog_coverage_test.go:
//
//	@web.TranslatedStrings(map[string]string{
//	    "range.picker.apply": i18n.T(ctx, "range.picker.apply"),
//	})
//
// The island is JSON, never a JavaScript object literal. encoding/json's Encoder
// escapes the angle brackets and ampersand to unicode sequences, so a translated
// string that carries a quote or a closing script tag can neither end the element
// nor inject markup. Keep it that way.
//
// A string a script substitutes into uses {name} placeholders, not Go's %s verbs.
// The value goes over unformatted, and the printer renders a %s with no argument
// as %!s(MISSING).
func TranslatedStrings(msgs map[string]string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		if _, err := io.WriteString(w, `<script type="application/json" data-letter-strings>`); err != nil {
			return err
		}
		if err := json.NewEncoder(w).Encode(msgs); err != nil {
			return err
		}
		_, err := io.WriteString(w, `</script>`)
		return err
	})
}
