package settingshttp

import (
	"context"
	"fmt"
	"math"
	"slices"
	"time"

	"github.com/a-h/templ"

	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/worlds"
	"spot-assistant/internal/infrastructure/i18n"
	"spot-assistant/internal/infrastructure/web"
)

// rankCopy is the copy of one rank section.
type rankCopy struct {
	Heading string
	Body    string
	// Empty says what an empty list means for this kind.
	Empty string
	Saved string
}

// The keys are spelled out per kind so the catalog coverage test can find them.
func copyFor(ctx context.Context, kind guildconfig.RoleKind) rankCopy {
	switch kind {
	case guildconfig.RoleKindManage:
		return rankCopy{
			Heading: i18n.T(ctx, "settings.ranks.manage.heading"),
			Body:    i18n.T(ctx, "settings.ranks.manage.body"),
			Empty:   i18n.T(ctx, "settings.ranks.manage.empty"),
			Saved:   i18n.T(ctx, "settings.ranks.manage.saved"),
		}
	case guildconfig.RoleKindView:
		return rankCopy{
			Heading: i18n.T(ctx, "settings.ranks.view.heading"),
			Body:    i18n.T(ctx, "settings.ranks.view.body"),
			Empty:   i18n.T(ctx, "settings.ranks.view.empty"),
			Saved:   i18n.T(ctx, "settings.ranks.view.saved"),
		}
	case guildconfig.RoleKindReserve:
		return rankCopy{
			Heading: i18n.T(ctx, "settings.ranks.reserve.heading"),
			Body:    i18n.T(ctx, "settings.ranks.reserve.body"),
			Empty:   i18n.T(ctx, "settings.ranks.reserve.empty"),
			Saved:   i18n.T(ctx, "settings.ranks.reserve.saved"),
		}
	case guildconfig.RoleKindOverbook:
		return rankCopy{
			Heading: i18n.T(ctx, "settings.ranks.overbook.heading"),
			Body:    i18n.T(ctx, "settings.ranks.overbook.body"),
			Empty:   i18n.T(ctx, "settings.ranks.overbook.empty"),
			Saved:   i18n.T(ctx, "settings.ranks.overbook.saved"),
		}
	}
	return rankCopy{}
}

// refreshLabel is the refresh button's own label, spliced into the sync hints so
// both name the control the same way in every language.
func refreshLabel(ctx context.Context) string { return i18n.T(ctx, "settings.refresh.action") }

// worldOptions lists every Tibia world. A guild without a world gets a leading
// empty entry, so the picker does not pretend the first world is stored.
func worldOptions(ctx context.Context, current string) []web.ComboboxOption {
	opts := make([]web.ComboboxOption, 0, len(worlds.Worlds)+1)
	if current == "" {
		opts = append(opts, web.ComboboxOption{Value: "", Label: i18n.T(ctx, "settings.world.none")})
	}
	for _, w := range worlds.Worlds {
		opts = append(opts, web.ComboboxOption{Value: w, Label: w})
	}
	return opts
}

func roleFieldID(kind guildconfig.RoleKind) string { return "ranks-" + string(kind) }

func contains(ss []string, s string) bool { return slices.Contains(ss, s) }

// cooldownMinutes rounds up, so the page never says "0 min".
func cooldownMinutes(d time.Duration) int {
	return int(math.Ceil(d.Minutes()))
}

// roleColorStyle paints a role's Discord colour swatch. The value is an int, so
// it cannot carry anything but a hex colour.
func roleColorStyle(color int) templ.SafeCSS {
	return templ.SafeCSS(fmt.Sprintf("background-color:#%06x", color&0xffffff))
}
