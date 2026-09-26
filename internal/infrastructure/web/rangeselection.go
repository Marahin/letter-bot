package web

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RangeSelection is the cookie-precedence result the range middleware resolves
// once per request, so Stats and Reservations don't each re-run the read-query /
// restore-cookie / persist dance. It owns only the part that is identical between
// the two pages, *which* query values to parse and whether the choice was
// explicit; each page still expands those values with its own defaults (the
// pages legitimately differ in their defaults).
type RangeSelection struct {
	// Query is the values a handler should parse for its day selection: the
	// explicit "days"/"from"/"to" from the URL, or "days=<stored cookie>" when a
	// shared selection is restored, or empty so the page applies its own default.
	Query url.Values
	// Explicit is true when the selection was pinned (URL params, or a restored
	// cookie), so links should carry it; false for the page's implicit default.
	Explicit bool
	// fromQuery distinguishes a URL-pinned selection (write it to the shared
	// cookie) from a cookie-restored one (already stored, don't rewrite).
	fromQuery bool
	// cleared is set by a present-but-empty "days", the picker's reset. Without
	// it the stored cookie would restore the range the user just cleared.
	cleared bool
}

// hasExplicitRange reports whether the query pins a day selection (the calendar's
// "days", or the noscript "from"/"to").
func hasExplicitRange(q url.Values) bool {
	return strings.TrimSpace(q.Get("days")) != "" ||
		strings.TrimSpace(q.Get("from")) != "" ||
		strings.TrimSpace(q.Get("to")) != ""
}

// isClearedRange reports whether the query carries an empty "days" and no
// from/to, the shape the picker submits to drop the pin and go back to the page
// default. An empty "days" beside a real from/to is a pinned range, not a reset.
func isClearedRange(q url.Values) bool {
	return q.Has("days") && strings.TrimSpace(q.Get("days")) == "" &&
		strings.TrimSpace(q.Get("from")) == "" && strings.TrimSpace(q.Get("to")) == ""
}

// resolveRangeSelection applies the shared cookie precedence: an empty "days"
// clears the pin; else an explicit URL selection wins (and is flagged to
// persist); else the stored cookie is restored as a "days" selection (explicit,
// but not re-persisted); else neither, leaving the page to fall back to its own
// default.
func (d *Deps) resolveRangeSelection(r *http.Request) RangeSelection {
	q := r.URL.Query()
	if isClearedRange(q) {
		return RangeSelection{Query: url.Values{}, cleared: true}
	}
	if hasExplicitRange(q) {
		return RangeSelection{Query: q, Explicit: true, fromQuery: true}
	}
	if stored := d.RangeSelectionCookie(r); stored != "" {
		return RangeSelection{Query: url.Values{"days": {stored}}, Explicit: true}
	}
	return RangeSelection{Query: url.Values{}}
}

// WithRangeSelection resolves the shared day-range selection (URL vs. stored
// cookie) once and stows it in the request context, so Stats and Reservations read
// it via RangeSelectionFrom and persist via PersistRangeSelection rather than
// each re-implementing the cookie precedence.
func (d *Deps) WithRangeSelection(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sel := d.resolveRangeSelection(r)
		next(w, r.WithContext(context.WithValue(r.Context(), ctxRangeSelection, sel)))
	}
}

// RangeSelectionFrom returns the resolved range selection for the request. Absent
// the middleware it yields a zero selection (empty query, implicit default).
func RangeSelectionFrom(ctx context.Context) RangeSelection {
	v, _ := ctx.Value(ctxRangeSelection).(RangeSelection)
	if v.Query == nil {
		v.Query = url.Values{}
	}
	return v
}

// PersistRangeSelection writes the page's resolved days to the shared range
// cookie, but only when the selection came from the URL, a cookie-restored
// selection is already stored, and the implicit default must not pin one. A
// cleared selection drops the cookie instead, so the reset survives the next
// page.
func (d *Deps) PersistRangeSelection(w http.ResponseWriter, r *http.Request, guildID string, days []time.Time) {
	sel := RangeSelectionFrom(r.Context())
	switch {
	case sel.cleared:
		d.SetRangeSelectionCookie(w, guildID, "")
	case sel.fromQuery:
		d.SetRangeSelectionCookie(w, guildID, EncodeDays(days))
	}
}

// EncodeDays joins days as a comma-separated YYYY-MM-DD list, the form of both
// the shared range cookie and the calendar's "days" query value.
func EncodeDays(days []time.Time) string {
	parts := make([]string, 0, len(days))
	for _, d := range days {
		parts = append(parts, d.Format("2006-01-02"))
	}
	return strings.Join(parts, ",")
}
