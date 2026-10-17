package web

import (
	"net/http"
	"strings"
)

// rangeCookieName holds the day-range selection chosen on a range picker. It is a
// per-server session cookie (see setGuildCookie), so the selection carries between
// the server's pages within a browser window and clears when the window closes.
const rangeCookieName = "letter_range"

// RangeSelectionCookie returns the stored selection for the current server as a
// "days" CSV (YYYY-MM-DD,…), or "" when none is set.
func (d *Deps) RangeSelectionCookie(r *http.Request) string {
	return guildCookie(r, rangeCookieName)
}

// SetRangeSelectionCookie persists the selection (a "days" CSV) for guildID. A
// blank value clears it.
func (d *Deps) SetRangeSelectionCookie(w http.ResponseWriter, guildID, days string) {
	d.setGuildCookie(w, rangeCookieName, guildID, days)
}

// crossRangeScope scopes the range cookie of the cross-server pages (/worlds, /spawns): they
// share one selection, apart from each server's own (letter_range_worlds).
const crossRangeScope = "worlds"

func crossRangeCookie(r *http.Request) string {
	c, err := r.Cookie(guildCookieName(rangeCookieName, crossRangeScope))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}
