package web

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderStrings(t *testing.T, msgs map[string]string) string {
	t.Helper()
	var b strings.Builder
	require.NoError(t, TranslatedStrings(msgs).Render(context.Background(), &b))
	return b.String()
}

func TestTranslatedStrings_RendersAMarkedJSONIsland(t *testing.T) {
	out := renderStrings(t, map[string]string{"richtext.editor.upload_failed": "Upload failed."})

	assert.Contains(t, out, `<script type="application/json" data-letter-strings>`)
	assert.Contains(t, out, `"richtext.editor.upload_failed":"Upload failed."`)
	assert.True(t, strings.HasSuffix(out, "</script>"))
}

// TestTranslatedStrings_EscapesMarkup is the XSS guard. A translation is content a
// contributor wrote, so the island must survive one containing a quote, a tag or a
// closing script tag without ending the element or emitting markup.
func TestTranslatedStrings_EscapesMarkup(t *testing.T) {
	msgs := map[string]string{
		"a.b.quote":  `He said "no"`,
		"a.b.script": `</script><script>alert(1)</script>`,
		"a.b.tag":    `<img src=x onerror=alert(1)>`,
		"a.b.amp":    `Tom & Jerry`,
	}
	out := renderStrings(t, msgs)

	// Exactly one script element: nothing in the payload closed or opened one.
	assert.Equal(t, 1, strings.Count(out, "<script"), out)
	assert.Equal(t, 1, strings.Count(out, "</script>"), out)
	// No angle bracket or ampersand from a value survives as itself; they leave as
	// \u-escapes, which is what makes the payload inert as markup.
	assert.NotContains(t, out, "<img")
	assert.NotContains(t, out, "<script>alert")
	assert.NotContains(t, out, "Tom &")
	assert.Contains(t, out, `</script>`)
	// A quote is JSON-escaped, so it cannot terminate the value either.
	assert.Contains(t, out, `He said \"no\"`)

	// And the escaping is lossless: what the browser parses back is what the
	// catalog held.
	payload := out[strings.Index(out, ">")+1 : strings.LastIndex(out, "</script>")]
	var round map[string]string
	require.NoError(t, json.Unmarshal([]byte(payload), &round))
	assert.Equal(t, msgs, round)
}

func TestTranslatedStrings_EmptyMapStillRendersValidJSON(t *testing.T) {
	assert.Contains(t, renderStrings(t, nil), ">null")
	assert.Contains(t, renderStrings(t, map[string]string{}), ">{}")
}
