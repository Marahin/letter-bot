package web

import (
	"context"
	"encoding/json"
	"io"

	"github.com/a-h/templ"
)

// Combobox (combobox.templ) is the app's searchable picker atom, modeled on the
// Build Calculator's sort control: a text input that filters an options panel,
// with a hidden input carrying the picked value for the form. dist/combobox.js
// owns the Alpine behaviour, so a page rendering the atom loads
// /assets/combobox.js before /assets/alpine.js. A page that pulls both in after
// Alpine started works too, which is how the shell fills its time-zone picker.

// ComboboxOption is one pickable entry. Values must be unique: the atom resolves
// a pick, and the panel keys its rows, by value.
type ComboboxOption struct {
	// Value is what the form submits when this option is picked.
	Value string `json:"value"`
	// Label is what the reader sees and filters by.
	Label string `json:"label"`
	// Group is the heading the panel prints above this option, already
	// localized. Consecutive options sharing one Group sit under one heading,
	// and an option with no Group joins the heading above it, so a run cannot
	// be closed without opening another. Group orders nothing on its own: the
	// caller floats the options it wants read first to the top of Options.
	Group string `json:"group,omitempty"`
}

// ComboboxProps configures the Combobox atom.
type ComboboxProps struct {
	// ID names this combobox uniquely on the page; the input, the options panel
	// and the options island derive their element ids from it. Two pickers
	// sharing an ID would read one another's entries.
	ID string
	// Name is the form field the hidden input submits.
	Name string
	// Label is the input's accessible name, already localized.
	Label string
	// Options are the pickable entries, in display order.
	Options []ComboboxOption
	// Selected is the initially picked option's value. A value no option
	// carries falls back to the first option, like a native select.
	Selected string
	// Class adds layout classes (width, margin) to the wrapper.
	Class string
}

// selectedLabel is the closed control's initial readout, resolved server-side
// so the input never flashes empty before Alpine hydrates.
func (p ComboboxProps) selectedLabel() string {
	for _, o := range p.Options {
		if o.Value == p.Selected {
			return o.Label
		}
	}
	if len(p.Options) > 0 {
		return p.Options[0].Label
	}
	return ""
}

// xData is the x-data expression naming the component, its element ids and the
// initial value. The entries themselves ride the JSON island below, not this
// attribute.
func (p ComboboxProps) xData() string {
	cfg := struct {
		ID       string `json:"id"`
		Selected string `json:"selected"`
	}{p.ID, p.Selected}
	b, err := json.Marshal(cfg)
	if err != nil { // a struct of strings cannot fail to marshal
		return "letterCombobox({})"
	}
	return "letterCombobox(" + string(b) + ")"
}

// optionsIsland ships the entries as a JSON island the component reads by id, the
// way TranslatedStrings ships copy. They rode the x-data attribute before, where
// templ escapes every quote: on a picker of hundreds of entries that escaping was
// half the payload. encoding/json escapes the angle brackets and the ampersand, so
// a label can neither end the element nor inject markup.
//
// A Group the entry above already opened is dropped. The component carries a
// heading forward, and one heading repeated per entry is most of what is left.
func (p ComboboxProps) optionsIsland() templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		opts := make([]ComboboxOption, len(p.Options))
		heading := ""
		for i, o := range p.Options {
			if o.Group == heading {
				o.Group = ""
			} else {
				heading = o.Group
			}
			opts[i] = o
		}
		b, err := json.Marshal(opts)
		if err != nil { // a slice of strings cannot fail to marshal
			b = []byte("[]")
		}
		if _, err := io.WriteString(w, `<script type="application/json" id="`+p.ID+`-options">`); err != nil {
			return err
		}
		if _, err := w.Write(b); err != nil {
			return err
		}
		_, err = io.WriteString(w, `</script>`)
		return err
	})
}
