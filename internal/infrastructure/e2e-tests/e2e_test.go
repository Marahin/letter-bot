//go:build e2e

// Package e2e drives a running devauth stack with a headless system Chrome
// (go-rod): sign-in through /dev/login, the premium lock, the public stats and
// the support links. It runs only under the e2e build tag (make e2e, make
// e2e-stack); see README.md.
package e2e

import (
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/go-rod/rod/lib/proto"
)

// The mock users and servers of internal/infrastructure/devauth. Copied, not
// imported: that package compiles only under the devauth tag.
const (
	managerID   = "700000000000000001"
	memberID    = "700000000000000002"
	outsiderID  = "700000000000000003"
	siteAdminID = "700000000000000004"
	ownerID     = "700000000000000005"
)

// actionTimeout bounds every wait. The suite never waits for network idle: the
// blocked Discord redirect keeps it open, so it waits on concrete conditions.
const actionTimeout = 30 * time.Second

const lockCopy = "Unlock the feature with Premium. Join the Discord and get on board!"

var (
	baseURL       string
	guildID       string
	lockedGuildID string
	supportInvite string
	browser       *rod.Browser
)

func TestMain(m *testing.M) {
	baseURL = envOr("E2E_BASE_URL", "http://localhost:18080")
	guildID = envOr("E2E_GUILD_ID", "700000000000000900")
	lockedGuildID = envOr("E2E_LOCKED_GUILD_ID", "700000000000000901")
	supportInvite = envOr("E2E_SUPPORT_URL", "https://discord.gg/b7Qq8V2XFR")
	health := envOr("E2E_HEALTH_URL", "http://localhost:13005/readyz")

	if !waitHealthy(health, 60*time.Second) {
		log.Println("e2e: the stack is not ready at " + health + "; start it with make e2e-stack, or see README.md")
		os.Exit(1)
	}
	if err := checkSeededGuild(); err != nil {
		log.Println("e2e: " + err.Error())
		os.Exit(1)
	}

	// Chrome's new headless mode hangs on page load in some sandboxes; the old one does not.
	u := launcher.New().Bin(envOr("E2E_CHROME_BIN", "/usr/sbin/google-chrome-stable")).
		Set(flags.Headless, "old").NoSandbox(true).MustLaunch()
	browser = rod.New().ControlURL(u).MustConnect()

	code := m.Run()

	browser.MustClose()
	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func waitHealthy(target string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(target)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(time.Second)
	}
	return false
}

// checkSeededGuild fails early on a stale database: a dev stack that ran the real
// bot clears bot_present on the fake servers, and every server page then 404s.
func checkSeededGuild() error {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second, Jar: jar}
	resp, err := client.Get(url("/dev/login/" + memberID))
	if err != nil {
		return fmt.Errorf("dev login as the seeded member: %w", err)
	}
	_ = resp.Body.Close()
	resp, err = client.Get(url("/servers/" + guildID + "/reservations"))
	if err != nil {
		return fmt.Errorf("open the seeded server: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the seeded member got HTTP %d on /servers/%s/reservations: the database is stale or /dev/login is off. "+
			"Reset the stack: make e2e-stack", resp.StatusCode, guildID)
	}
	return nil
}

func url(path string) string {
	return baseURL + path
}

func serverPath(id, suffix string) string {
	return "/servers/" + id + suffix
}

// newPage is a fresh incognito page, closed at the end of the test.
func newPage(t *testing.T) *rod.Page {
	t.Helper()
	inc, err := browser.Incognito()
	if err != nil {
		t.Fatalf("incognito browser context: %v", err)
	}
	page, err := inc.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		t.Fatalf("open page: %v", err)
	}
	// /login redirects to Discord, which the sandbox cannot reach: block it, so the
	// navigation stops at once and a test can assert on the blocked URL.
	page.MustSetBlockedURLs("*discord.com*")
	t.Cleanup(func() { _ = page.Close() })
	return page
}

func waitReady(t *testing.T, page *rod.Page) {
	t.Helper()
	if err := rod.Try(func() {
		page.Timeout(actionTimeout).MustWait(`() => document.readyState === 'complete'`)
	}); err != nil {
		t.Fatalf("page did not reach readyState=complete within %s: %v", actionTimeout, err)
	}
}

// open navigates to path and waits for the page.
func open(t *testing.T, page *rod.Page, path string) {
	t.Helper()
	if err := page.Navigate(url(path)); err != nil {
		t.Fatalf("navigate to %s: %v", path, err)
	}
	waitReady(t, page)
}

// waitURLContains waits for the URL; substr must not match the URL being left.
func waitURLContains(t *testing.T, page *rod.Page, substr string) {
	t.Helper()
	if err := rod.Try(func() {
		page.Timeout(actionTimeout).MustWait(`(s) => location.href.includes(s)`, substr)
	}); err != nil {
		t.Fatalf("page URL did not contain %q within %s (was %q): %v", substr, actionTimeout, page.MustInfo().URL, err)
	}
}

// clickAndWaitReload clicks el and waits for a new document. A form that
// redirects back to the same URL changes neither the URL nor readyState, so a
// stamp on the old document is the signal.
func clickAndWaitReload(t *testing.T, page *rod.Page, el *rod.Element) {
	t.Helper()
	page.MustEval(`() => { window.__e2eDocStamp = true }`)
	el.MustClick()
	if err := rod.Try(func() {
		page.Timeout(actionTimeout).MustWait(`() => window.__e2eDocStamp === undefined && document.readyState === 'complete'`)
	}); err != nil {
		t.Fatalf("the click did not load a new document within %s: %v", actionTimeout, err)
	}
}

func loginAs(t *testing.T, page *rod.Page, userID string) {
	t.Helper()
	open(t, page, "/dev/login/"+userID)
}

// currentPath is the path and query of the page's URL.
func currentPath(page *rod.Page) string {
	return strings.TrimPrefix(page.MustInfo().URL, baseURL)
}

func has(page *rod.Page, selector string) bool {
	return page.MustHas(selector)
}

func text(page *rod.Page, selector string) string {
	return page.Timeout(actionTimeout).MustElement(selector).MustText()
}

func requireHas(t *testing.T, page *rod.Page, selector string) {
	t.Helper()
	if !has(page, selector) {
		t.Fatalf("expected %s on %s", selector, currentPath(page))
	}
}

func requireNotHas(t *testing.T, page *rod.Page, selector string) {
	t.Helper()
	if has(page, selector) {
		t.Fatalf("expected no %s on %s", selector, currentPath(page))
	}
}
