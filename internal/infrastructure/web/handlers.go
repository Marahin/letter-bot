package web

import "net/http"

func (s *Server) handleLanding(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	d.Render(w, r, Landing(s.cfg.BaseURL, d.InviteURLGeneric(), d.MarketingNav(r)))
}

// handleNotFound serves the branded 404 for any path no route claimed. Registered
// as the "GET /" catch-all; more specific routes still win.
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	s.deps().NotFound(w, r)
}

// handleSetLanguage records the language picked in the shell's picker. Not behind
// sign-in: for an anonymous visitor the cookie is the only carrier there is.
func (s *Server) handleSetLanguage(w http.ResponseWriter, r *http.Request) {
	d := s.deps()
	if !ParseForm(w, r) {
		return
	}
	d.SetLanguage(w, r.PostFormValue("lang"))
	http.Redirect(w, r, d.safeReturnTo(r.PostFormValue("to")), http.StatusSeeOther)
}
