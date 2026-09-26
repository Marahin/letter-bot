package web

import (
	"context"
	"slices"
	"strings"
	"unicode"

	"github.com/a-h/templ"

	"spot-assistant/internal/infrastructure/i18n"
)

// lootCalculatorPath is the public tool the marketing top bar and the sidebar link to.
const lootCalculatorPath = "/tools/loot-calculator"

// toolCard is one no-login tool card on the landing page.
type toolCard struct {
	Name string
	CTA  string
	Href string
	// Icon names the toolGlyph the card draws.
	Icon string
}

// serverFeature is one card in the landing feature grid.
type serverFeature struct {
	Name string
	Desc string
	// Icon names the glyph landingFeaturesSection draws.
	Icon string
}

func landingTools(ctx context.Context) []toolCard {
	return []toolCard{
		{Icon: "calculator", Name: i18n.T(ctx, "shell.nav.loot_calculator"), CTA: i18n.T(ctx, "landing.tool.loot.cta"), Href: lootCalculatorPath},
	}
}

func landingFeatures(ctx context.Context) []serverFeature {
	return []serverFeature{
		{Icon: "reservations", Name: i18n.T(ctx, "landing.feature.reservations.name"), Desc: i18n.T(ctx, "landing.feature.reservations.desc")},
		{Icon: "bot", Name: i18n.T(ctx, "landing.feature.summary.name"), Desc: i18n.T(ctx, "landing.feature.summary.desc")},
		{Icon: "stats", Name: i18n.T(ctx, "landing.feature.stats.name"), Desc: i18n.T(ctx, "landing.feature.stats.desc")},
		{Icon: "roles", Name: i18n.T(ctx, "landing.feature.roles.name"), Desc: i18n.T(ctx, "landing.feature.roles.desc")},
		{Icon: "channels", Name: i18n.T(ctx, "landing.feature.channels.name"), Desc: i18n.T(ctx, "landing.feature.channels.desc")},
		{Icon: "characters", Name: i18n.T(ctx, "landing.feature.characters.name"), Desc: i18n.T(ctx, "landing.feature.characters.desc")},
	}
}

// NavServer is one server in the sidebar switcher.
type NavServer struct {
	ID   string
	Name string
	Icon string
}

// Nav carries the navigation state shared by every page: who is signed in, which
// servers they can switch between, and the selected server context.
type Nav struct {
	Authenticated bool
	Username      string
	// Servers are the bot-present servers the user may open.
	Servers          []NavServer
	CurrentGuildID   string // selected server, empty when none is in context
	CurrentGuildName string
	// CurrentGuildIcon is the selected server's Discord icon hash, empty when it has none.
	CurrentGuildIcon string
	// IsAdmin gates the Channels and Settings links (owner or Administrator).
	IsAdmin bool
	// CanManage gates respawn management and every reservation's edit controls.
	CanManage bool
	// CanReserve gates the new-reservation controls.
	CanReserve bool
	// SiteAdmin gates the sidebar's site-wide Admin section.
	SiteAdmin bool
	// Active is the current section or sub-view: reservations | spots | stats |
	// channels | settings | loot-calculator | admin-guilds, or a key from statsSubviews.
	Active string
	// Wide drops the centred content max-width so a page can fill the width.
	Wide bool
	// Stylesheets are page-specific sheet hrefs the shell links from <head>.
	Stylesheets []string
	// Marketing renders the marketing top bar (not the app sidebar) even for a
	// signed-in visitor, and gives the top bar the landing's section width.
	Marketing bool
	// ReturnTo is the original request URI, where the language picker's POST sends
	// the visitor back to. Empty on a Nav built without a request; the shell then
	// falls back to the path Layout was given.
	ReturnTo string
}

// WithStylesheet returns a copy of n with href added to the page stylesheets. The
// copy gets a fresh backing array, so the caller's slice is never written through.
func (n Nav) WithStylesheet(href string) Nav {
	sheets := make([]string, 0, len(n.Stylesheets)+1)
	sheets = append(sheets, n.Stylesheets...)
	sheets = append(sheets, href)
	n.Stylesheets = sheets
	return n
}

// GuildPath is a server-scoped URL: "/servers/{id}" plus suffix ("/reservations").
func GuildPath(guildID, suffix string) string {
	return "/servers/" + guildID + suffix
}

// guildIconURL builds the Discord CDN URL for a server icon, "" when it has none so
// the switcher draws a lettered fallback instead.
func guildIconURL(guildID, iconHash string) string {
	if guildID == "" || iconHash == "" {
		return ""
	}
	return "https://cdn.discordapp.com/icons/" + guildID + "/" + iconHash + ".png?size=64"
}

// CurrentGuildIconURL is the framed selected-server icon for the switcher.
func (n Nav) CurrentGuildIconURL() string {
	return guildIconURL(n.CurrentGuildID, n.CurrentGuildIcon)
}

// guildInitial is the lettered fallback for a server without an icon.
func guildInitial(name string) string {
	for _, r := range strings.TrimSpace(name) {
		return string(unicode.ToUpper(r))
	}
	return "?"
}

// dropdownProps configures the shared dropdown shell: a <details> menu with a
// clickable summary and a floating panel (the component's children).
type dropdownProps struct {
	// detailsClass replaces the default "relative", so a panel can anchor to an
	// outer positioned box instead.
	detailsClass string
	summaryClass string
	panelClass   string
	summary      templ.Component
	attrs        templ.Attributes
	summaryAttrs templ.Attributes
}

// topBarTool is one tool link in the marketing top bar.
type topBarTool struct {
	Name    string
	Href    string
	Icon    string
	Current bool
}

func (n Nav) topBarTools(ctx context.Context) []topBarTool {
	return []topBarTool{
		{Name: i18n.T(ctx, "shell.nav.loot_calculator"), Href: lootCalculatorPath, Icon: "calculator", Current: n.Active == "loot-calculator"},
	}
}

// InStats reports whether the current section is any Stats sub-view, so the
// sidebar keeps the parent link highlighted while a sub-link marks the exact page.
func (n Nav) InStats() bool {
	return strings.HasPrefix(n.Active, "stats")
}

// subRail is the tree-rail segment a sidebar sub-view row draws: nothing, a
// vertical run through the row, or the corner that turns into the active row.
type subRail int

const (
	subRailNone subRail = iota
	subRailRun
	subRailCorner
)

// statsSubviews must match the render order in layout.templ.
var statsSubviews = []string{"stats", "stats-spots", "stats-players", "stats-characters"}

// subRailFor picks the tree-rail segment for the sub-view key: a run above the
// active row, a corner on it, nothing below it or when no sub-view is active.
func (n Nav) subRailFor(order []string, key string) subRail {
	activeIdx := slices.Index(order, n.Active)
	idx := slices.Index(order, key)
	switch {
	case activeIdx < 0 || idx < 0 || idx > activeIdx:
		return subRailNone
	case idx == activeIdx:
		return subRailCorner
	default:
		return subRailRun
	}
}

// returnTarget is where the language picker posts the visitor back to. Validated
// server-side all the same (Deps.safeReturnTo).
func (n Nav) returnTarget(renderedPath string) string {
	if n.ReturnTo != "" {
		return n.ReturnTo
	}
	if renderedPath != "" {
		return renderedPath
	}
	return "/"
}
