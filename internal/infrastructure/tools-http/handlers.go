package toolshttp

import (
	"errors"
	"net/http"

	"github.com/a-h/templ"

	"spot-assistant/internal/core/dto/loot"
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
	h.D.Render(w, r, LootCalculator(h.D.Cfg.BaseURL, lootView{Autofocus: true}, h.nav(r)))
}

// LandingCalculator is the empty calculator the landing page opens with. Its form
// posts to the calculator route, which htmx swaps back in place.
func LandingCalculator() templ.Component {
	return calculator(lootView{From: landingFrom, Rows: landingRows, Autofocus: true})
}

// HandleLootCalculate splits the pasted session and counts it in the public stats
// unless the user unticked that (the raw text is never stored). htmx gets the main
// column (the result, or the form with the error under the field); a plain post
// gets the whole page.
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
		var s *lootcalc.Session
		v, s = calculate(r.PostFormValue("session"))
		v.Uncounted = !countRequested(r)
		if s != nil {
			v.Result.Entry.Count = !v.Uncounted
			v.Result.Stats = h.record(r, *s, !v.Uncounted)
		}
	}
	v.From = startPage(r.PostFormValue("from"))

	if web.IsHTMX(r) {
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

// countRequested reads the "count in public stats" box. A form without the box
// (count_field absent, e.g. an older page) counts, as the box is ticked by default.
func countRequested(r *http.Request) bool {
	return r.PostFormValue("count_field") == "" || r.PostFormValue("count") == "1"
}

// record counts the session in the stats. A failure is logged and noticed: the
// split itself is still right.
func (h *Handlers) record(r *http.Request, s lootcalc.Session, count bool) statsNotice {
	out, err := h.D.Loot.Record(r.Context(), s.Submission(), count)
	if err != nil {
		h.D.Log.Warnw("record loot session", "error", err)
		return statsNotice{Status: loot.StatusFailed}
	}
	return newStatsNotice(out)
}

// calculate returns the parsed session too, nil when the text does not parse.
func calculate(text string) (lootView, *lootcalc.Session) {
	v := lootView{Text: text}
	s, err := lootcalc.Parse(text)
	if err != nil {
		v.Error = err
		return v, nil
	}
	v.Result = newResultView(text, s, lootcalc.Split(s))
	return v, &s
}
