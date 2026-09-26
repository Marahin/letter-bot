package toolshttp

import (
	"errors"
	"net/http"

	"spot-assistant/internal/core/lootcalc"
	"spot-assistant/internal/infrastructure/web"
)

// MaxSessionBytes caps the form body. A 10-player analyser text is under 2 KB.
const MaxSessionBytes = 64 << 10

type Handlers struct {
	D *web.Deps
}

func New(d *web.Deps) *Handlers { return &Handlers{D: d} }

func (h *Handlers) HandleLootCalculator(w http.ResponseWriter, r *http.Request) {
	h.D.Render(w, r, LootCalculator(h.D.Cfg.BaseURL, lootView{}, h.nav(r)))
}

// HandleLootCalculate splits the pasted session. htmx gets the main column
// (the result, or the form with the error under the field); a plain post gets
// the whole page. Nothing is stored on the server.
func (h *Handlers) HandleLootCalculate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxSessionBytes)
	var v lootView
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if !errors.As(err, &tooLarge) {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		v.Error = errTooLong
	} else {
		v = calculate(r.PostFormValue("session"))
	}

	if r.Header.Get("HX-Request") == "true" {
		h.D.Render(w, r, lootMain(v))
		return
	}
	h.D.Render(w, r, LootCalculator(h.D.Cfg.BaseURL, v, h.nav(r)))
}

func (h *Handlers) nav(r *http.Request) web.Nav {
	n := h.D.Nav(r, "")
	n.Active = "loot-calculator"
	return n
}

func calculate(text string) lootView {
	v := lootView{Text: text}
	s, err := lootcalc.Parse(text)
	if err != nil {
		v.Error = err
		return v
	}
	v.Result = newResultView(text, s, lootcalc.Split(s))
	return v
}
