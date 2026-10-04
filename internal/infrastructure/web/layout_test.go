package web

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/infrastructure/branding"
	"spot-assistant/internal/infrastructure/i18n"
)

func render(t *testing.T, ctx context.Context, c templ.Component) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, c.Render(ctx, &b))
	return b.String()
}

func signedInNav() Nav {
	return Nav{
		Authenticated:    true,
		Username:         "Knight",
		Servers:          []NavServer{{ID: "1", Name: "Celesta", Icon: "abc"}, {ID: "2", Name: "other"}},
		CurrentGuildID:   "1",
		CurrentGuildName: "Celesta",
		CurrentGuildIcon: "abc",
		Active:           "stats-players",
	}
}

var assetRef = regexp.MustCompile(`AssetURL\("(/assets/[^"]+)"\)`)

// TestLayout_EveryLinkedAssetIsEmbedded keeps the script list in step with dist/:
// a renamed or dropped file would 404 on every page.
func TestLayout_EveryLinkedAssetIsEmbedded(t *testing.T) {
	// given every AssetURL the shell templates name
	var refs []string
	for _, name := range []string{"layout.templ", "landing.templ", "errorpage.templ"} {
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		for _, m := range assetRef.FindAllStringSubmatch(string(src), -1) {
			refs = append(refs, m[1])
		}
	}
	require.NotEmpty(t, refs)
	refs = append(refs, "/assets/l10n/pl.js")

	// then each one resolves in the embedded tree
	for _, ref := range refs {
		_, err := fs.Stat(FS(), strings.TrimPrefix(ref, assetsPrefix))
		assert.NoErrorf(t, err, "%s is linked but not embedded", ref)
	}
}

func TestFavicons_HaveTheirDeclaredSizes(t *testing.T) {
	for name, size := range map[string]int{
		"favicon-16.png": 16, "favicon-32.png": 32, "favicon-48.png": 48,
		"favicon-192.png": 192, "favicon-512.png": 512, "apple-touch-icon.png": 180,
	} {
		f, err := FS().Open(name)
		require.NoError(t, err)
		cfg, err := png.DecodeConfig(f)
		_ = f.Close()
		require.NoErrorf(t, err, name)
		assert.Equalf(t, size, cfg.Width, "%s width", name)
		assert.Equalf(t, size, cfg.Height, "%s height", name)
	}
}

func TestLayout_SignedOutShowsTheTopBar(t *testing.T) {
	// when
	out := render(t, context.Background(), Layout("T", "D", "http://x", "/p", Nav{}))

	// then
	assert.Contains(t, out, `href="/login"`)
	assert.Contains(t, out, `data-topbar-menu`)
	assert.NotContains(t, out, "<aside")
	assert.NotContains(t, out, "combobox.js")
	assert.Contains(t, out, `<link rel="canonical" href="http://x/p">`)
	assert.Contains(t, out, `name="to" value="/p"`)
}

func TestLayout_SignedInShowsTheSidebar(t *testing.T) {
	// when
	out := render(t, context.Background(), Layout("T", "D", "http://x", "/p", signedInNav()))

	// then
	assert.Contains(t, out, "<aside")
	assert.Contains(t, out, `href="/servers/1/reservations"`)
	assert.Contains(t, out, `href="/servers/1/spots"`)
	assert.Contains(t, out, `href="/servers/1/stats/players"`)
	assert.Contains(t, out, "https://cdn.discordapp.com/icons/1/abc.png?size=64")
	assert.Contains(t, out, "2 servers")
	assert.Contains(t, out, "combobox.js")
	assert.Contains(t, out, "is-corner")
	assert.NotContains(t, out, `href="/servers/1/settings"`, "settings are admin-only")
	assert.NotContains(t, out, `href="/admin/guilds"`, "the admin section is for site admins")
}

func TestLayout_AdminLinks(t *testing.T) {
	// given
	nav := signedInNav()
	nav.IsAdmin = true
	nav.SiteAdmin = true

	// when
	out := render(t, context.Background(), Layout("T", "D", "http://x", "/p", nav))

	// then
	assert.Contains(t, out, `href="/servers/1/settings"`)
	assert.Contains(t, out, `href="/servers/1/channels"`)
	assert.Contains(t, out, `href="/admin/guilds"`)
}

func TestLayout_NoServerSelectedOffersTheDashboard(t *testing.T) {
	// given
	nav := Nav{Authenticated: true, Username: "Knight"}

	// when
	out := render(t, context.Background(), Layout("T", "D", "http://x", "/p", nav))

	// then
	assert.Contains(t, out, "Choose a server")
	assert.NotContains(t, out, "/servers/")
}

func TestLayout_MarketingNavKeepsTheTopBarWhenSignedIn(t *testing.T) {
	// given
	nav := signedInNav()
	nav.Marketing = true

	// when
	out := render(t, context.Background(), Layout("T", "D", "http://x", "/", nav))

	// then
	assert.NotContains(t, out, "<aside")
	assert.Contains(t, out, `href="/dashboard"`)
	assert.Contains(t, out, "Knight")
}

func TestErrorPage_SignedInLinksToTheDashboard(t *testing.T) {
	// when
	out := render(t, context.Background(), ErrorPage("http://x", 403, "Denied", "No.", signedInNav()))

	// then
	assert.Contains(t, out, "Back to dashboard")
	assert.Contains(t, out, "<title>Denied - TibiaLoot.com</title>")
}

func TestLandingCopyIsTranslated(t *testing.T) {
	// given a Polish context
	ctx := i18n.WithLocale(context.Background(), i18n.Normalize("pl"))

	// when
	out := render(t, ctx, Landing("http://x", "https://invite", Nav{Marketing: true}, nil))

	// then
	assert.Contains(t, out, "Dodaj Letter do Discorda")
	assert.Contains(t, out, "Podziel loot z huntu.")
	assert.Contains(t, out, "Otwórz kalkulator lootu")
	assert.Contains(t, out, `href="https://invite"`)
	assert.Contains(t, out, "Serwis niezwiązany z CipSoft.")
}

func TestLanding_EmbedsTheToolAboveTheBotFeatures(t *testing.T) {
	// given
	tool := templ.Raw(`<div id="embedded-tool"></div>`)

	// when
	out := render(t, context.Background(), Landing("http://x", "https://invite", Nav{Marketing: true}, tool))

	// then
	assert.Contains(t, out, `<h1 id="landing-calculator-heading"`)
	assert.Less(t, strings.Index(out, `id="embedded-tool"`), strings.Index(out, `id="features"`))
	assert.NotContains(t, out, "Open the Loot Calculator")
}

func TestLayout_FooterAndLockup(t *testing.T) {
	// when
	out := render(t, context.Background(), Layout("T", "D", "http://x", "/", Nav{}))

	// then
	assert.Contains(t, out, branding.Notice())
	assert.Contains(t, out, "Not affiliated with CipSoft.")
	assert.Contains(t, out, `aria-label="TibiaLoot.com"`)
	assert.Contains(t, out, `<span class="text-zone-100">Tibia</span><span class="text-signal">Loot</span>`)
}

func TestDeps_ErrorHelpers(t *testing.T) {
	d := newTestDeps(t)
	for name, tc := range map[string]struct {
		call   func(w http.ResponseWriter, r *http.Request)
		status int
		body   string
	}{
		"bad request": {func(w http.ResponseWriter, r *http.Request) { d.BadRequest(w, r, "nope") }, http.StatusBadRequest, "nope\n"},
		"forbidden":   {d.Forbidden, http.StatusForbidden, "You don't have access to manage this server.\n"},
		"not found":   {d.NotFound, http.StatusNotFound, "Not found.\n"},
		"server error": {func(w http.ResponseWriter, r *http.Request) {
			d.ServerError(w, r, "boom", errors.New("x"))
		}, http.StatusInternalServerError, "internal server error\n"},
		"unavailable": {func(w http.ResponseWriter, r *http.Request) {
			d.Unavailable(w, r, "down", errors.New("x"))
		}, http.StatusServiceUnavailable, "service temporarily unavailable, please try again\n"},
	} {
		t.Run(name, func(t *testing.T) {
			// given an htmx request, which gets plain text
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set("HX-Request", "true")
			req.Header.Set("Accept", "text/html")
			rec := httptest.NewRecorder()

			// when
			tc.call(rec, req)

			// then
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, tc.body, rec.Body.String())
		})
	}
}

func TestDeps_UnavailableSetsRetryAfter(t *testing.T) {
	// when
	rec := httptest.NewRecorder()
	newTestDeps(t).Unavailable(rec, httptest.NewRequest(http.MethodGet, "/x", nil), "down", errors.New("x"))

	// then
	assert.Equal(t, "5", rec.Header().Get("Retry-After"))
}

func TestDeps_PathInt64(t *testing.T) {
	d := newTestDeps(t)

	// given a numeric path value
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	r.SetPathValue("sid", "42")
	v, ok := d.PathInt64(httptest.NewRecorder(), r, "sid")
	assert.True(t, ok)
	assert.Equal(t, int64(42), v)

	// given a non-numeric one
	r.SetPathValue("sid", "abc")
	rec := httptest.NewRecorder()
	_, ok = d.PathInt64(rec, r, "sid")
	assert.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeps_InviteURLs(t *testing.T) {
	// given
	d := newTestDeps(t)
	d.Cfg.Discord.ClientID = "4242"

	// when
	generic := d.InviteURLGeneric()
	scoped := d.InviteURL("99")

	// then
	assert.Equal(t, "https://discord.com/oauth2/authorize?client_id=4242&permissions=268561424&scope=bot+applications.commands", generic)
	assert.Contains(t, scoped, "guild_id=99")
	assert.Contains(t, scoped, "disable_guild_select=true")
}

func TestHelpers(t *testing.T) {
	assert.Equal(t, "/servers/1/stats", GuildPath("1", "/stats"))
	assert.Equal(t, "", guildIconURL("1", ""))
	assert.Equal(t, "", guildIconURL("", "abc"))
	assert.Equal(t, "Ł", guildInitial("  łódź"))
	assert.Equal(t, "?", guildInitial(" "))
	assert.Equal(t, "/p", Nav{}.returnTarget("/p"))
	assert.Equal(t, "/", Nav{}.returnTarget(""))
	assert.Equal(t, "/q?x=1", Nav{ReturnTo: "/q?x=1"}.returnTarget("/p"))
	assert.True(t, Nav{Active: "stats-spots"}.InStats())
	assert.False(t, Nav{Active: "spots"}.InStats())
}

func TestNav_SubRailFor(t *testing.T) {
	n := Nav{Active: "stats-players"}
	assert.Equal(t, subRailRun, n.subRailFor(statsSubviews, "stats"))
	assert.Equal(t, subRailCorner, n.subRailFor(statsSubviews, "stats-players"))
	assert.Equal(t, subRailNone, n.subRailFor(statsSubviews, "stats-characters"))
	assert.Equal(t, subRailNone, Nav{Active: "spots"}.subRailFor(statsSubviews, "stats"))
}

func TestServerBadge_KeepsTheInitialUnderAnIconThatFailsToLoad(t *testing.T) {
	// given
	ctx := context.Background()

	// when
	withIcon := render(t, ctx, serverBadge("https://cdn.discordapp.com/icons/1/abc.png", "celesta"))
	option := render(t, ctx, serverOptionIcon("https://cdn.discordapp.com/icons/1/abc.png", "celesta"))
	without := render(t, ctx, serverBadge("", "celesta"))

	// then
	for _, out := range []string{withIcon, option} {
		assert.Contains(t, out, ">C")
		assert.Contains(t, out, `onerror="this.remove()"`)
	}
	assert.Contains(t, without, ">C")
	assert.NotContains(t, without, "<img")
}

func TestLayout_RendersTheHTMXErrorToastOnEveryShell(t *testing.T) {
	for name, nav := range map[string]Nav{"signed out": {}, "signed in": signedInNav()} {
		t.Run(name, func(t *testing.T) {
			// when
			out := render(t, context.Background(), Layout("T", "D", "http://x", "/", nav))

			// then
			assert.Contains(t, out, `id="letter-toast"`)
			assert.Contains(t, out, `role="status"`)
			assert.Contains(t, out, `aria-live="polite"`)
			assert.Contains(t, out, `"shell.toast.error":"Something went wrong. Try again."`)
			assert.Contains(t, out, "/assets/htmx-errors.js")
		})
	}
}
