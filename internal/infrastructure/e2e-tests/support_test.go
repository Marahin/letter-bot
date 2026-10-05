//go:build e2e

package e2e

import "testing"

func supportLink() string {
	return `a[data-support-link][href="` + supportInvite + `"]`
}

func TestSupportLink_InTheFooterForAnonymousVisitors(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, "/")

	// then
	requireHas(t, page, "footer "+supportLink())
	requireNotHas(t, page, "header "+supportLink())
	if attr := page.MustElement("footer " + supportLink()).MustAttribute("target"); attr == nil || *attr != "_blank" {
		t.Fatalf("the support link must open a new tab")
	}
}

func TestSupportLink_InTheSidebarWhenSignedIn(t *testing.T) {
	// given
	page := newPage(t)
	loginAs(t, page, memberID)

	// when
	open(t, page, "/dashboard")

	// then
	requireHas(t, page, "aside "+supportLink())
	requireHas(t, page, "footer "+supportLink())
}

func TestSupportLink_404InvitesAReport(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, "/no-such-page")

	// then
	requireHas(t, page, "main "+supportLink())
}
