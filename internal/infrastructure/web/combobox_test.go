package web

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func comboProps() ComboboxProps {
	return ComboboxProps{
		ID:    "post-channel",
		Name:  "channel_id",
		Label: "Post channel",
		Options: []ComboboxOption{
			{Value: "", Label: "Off"},
			{Value: "c1", Label: "#general"},
			{Value: "c2", Label: "#war-room"},
		},
		Selected: "c2",
		Class:    "min-w-56",
	}
}

func TestComboboxProps_SelectedLabel(t *testing.T) {
	// given a selected value an option carries, then its label is the readout
	p := comboProps()
	assert.Equal(t, "#war-room", p.selectedLabel())

	// a value no option carries falls back to the first option, like a native select
	p.Selected = "gone"
	assert.Equal(t, "Off", p.selectedLabel())

	// no options at all reads empty
	p.Options = nil
	assert.Equal(t, "", p.selectedLabel())
}

func TestComboboxProps_XData(t *testing.T) {
	// given the props, then the x-data expression names the component, the element
	// ids and the stored value. The options ride the island, not the attribute.
	got := comboProps().xData()
	assert.True(t, strings.HasPrefix(got, "letterCombobox("))
	assert.True(t, strings.HasSuffix(got, ")"))
	assert.Contains(t, got, `"selected":"c2"`)
	// the id feeds optionId() and names the island the component reads
	assert.Contains(t, got, `"id":"post-channel"`)
	assert.NotContains(t, got, "#war-room")
}

// renderIsland is the options island's own markup.
func renderIsland(t *testing.T, p ComboboxProps) string {
	t.Helper()
	var sb strings.Builder
	require.NoError(t, p.optionsIsland().Render(context.Background(), &sb))
	return sb.String()
}

func TestComboboxProps_OptionsIsland(t *testing.T) {
	// given the props, then the entries arrive as JSON the component reads by id
	got := renderIsland(t, comboProps())
	assert.Contains(t, got, `<script type="application/json" id="post-channel-options">`)
	assert.Contains(t, got, `{"value":"c2","label":"#war-room"}`)
	// no groups asked for, none sent
	assert.NotContains(t, got, `"group"`)
}

func TestComboboxProps_OptionsIslandSendsAHeadingOncePerRun(t *testing.T) {
	// given a picker whose options carry a heading each, as a caller writes them
	p := comboProps()
	p.Options = []ComboboxOption{
		{Value: "c1", Label: "#general", Group: "Text"},
		{Value: "c2", Label: "#war-room", Group: "Text"},
		{Value: "v1", Label: "Comms", Group: "Voice"},
	}
	got := renderIsland(t, p)

	// then each run sends its heading once: the component carries it forward, and
	// a heading per option is most of the payload on a picker of hundreds
	assert.Contains(t, got, `{"value":"c1","label":"#general","group":"Text"}`)
	assert.Contains(t, got, `{"value":"c2","label":"#war-room"}`)
	assert.Contains(t, got, `{"value":"v1","label":"Comms","group":"Voice"}`)
}

func TestComboboxProps_OptionsIslandCannotEndItsOwnElement(t *testing.T) {
	// given a label carrying markup, then the island escapes it: a JSON island can
	// neither be closed early nor inject a tag
	p := comboProps()
	p.Options = []ComboboxOption{{Value: "x", Label: "</script><img src=x>"}}
	got := renderIsland(t, p)
	assert.Equal(t, 1, strings.Count(got, "</script>"))
	assert.NotContains(t, got, "<img")
}

func TestCombobox_RendersTheGroupedPanel(t *testing.T) {
	// given the atom rendered, then the panel prints a heading per run of options
	// and keys the keyboard highlight to the option's place in the filtered list
	var sb strings.Builder
	require.NoError(t, Combobox(comboProps()).Render(context.Background(), &sb))
	html := sb.String()

	assert.Contains(t, html, `x-for="g in groups"`)
	assert.Contains(t, html, `letter-combo-group`)
	assert.Contains(t, html, `optionId(it.i)`)
}

func TestCombobox_RendersFormFieldAndARIA(t *testing.T) {
	// given the atom rendered, then the hidden input carries the form contract
	// and the visible input carries the combobox ARIA wiring
	var sb strings.Builder
	require.NoError(t, Combobox(comboProps()).Render(context.Background(), &sb))
	html := sb.String()

	assert.Contains(t, html, `name="channel_id"`)
	assert.Contains(t, html, `type="hidden"`)
	assert.Contains(t, html, `value="c2"`)
	assert.Contains(t, html, `role="combobox"`)
	assert.Contains(t, html, `aria-controls="post-channel-panel"`)
	assert.Contains(t, html, `id="post-channel-panel"`)
	assert.Contains(t, html, `role="listbox"`)
	assert.Contains(t, html, `aria-label="Post channel"`)
	assert.Contains(t, html, `:aria-activedescendant`)
	// the closed control's readout is pre-rendered, so it never flashes empty
	assert.Contains(t, html, `value="#war-room"`)
	// the options ride a JSON island the component reads, and the panel is an x-if
	// template Alpine fills, so a closed picker renders no option text
	assert.Contains(t, html, `id="post-channel-options"`)
	assert.NotContains(t, html, `>#general<`)
}
