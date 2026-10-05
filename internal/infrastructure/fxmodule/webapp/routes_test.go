package webapp

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
	"spot-assistant/internal/core/permission"
	"spot-assistant/internal/infrastructure/web"
	"spot-assistant/internal/infrastructure/web/webtest"
)

const lockCopy = "Unlock the feature with Premium. Join the Discord and get on board!"

var wildcard = regexp.MustCompile(`\{[^}]+\}`)

func registerServerRoutes(r *web.Router, d *web.Deps) {
	web.RegisterGuildRoot(r, d)
	for _, register := range features {
		register(r, d)
	}
}

// serverRoutes are the registered routes under /servers/{id}, the root included,
// with their paths filled in for server g1.
func serverRoutes(t *testing.T, d *web.Deps) map[*web.Route]string {
	t.Helper()
	out := map[*web.Route]string{}
	for _, route := range d.Routes.Routes() {
		if !strings.HasPrefix(route.Pattern(), "/servers/{id}") {
			continue
		}
		p := strings.Replace(route.Pattern(), "{id}", "g1", 1)
		out[route] = wildcard.ReplaceAllString(p, "1")
	}
	require.Contains(t, out, d.Routes.Lookup(http.MethodGet, "/servers/g1"), "the server root is covered")
	return out
}

func lockedServer() *access.GuildAccess {
	return &access.GuildAccess{
		Config: guildconfig.Config{GuildID: "g1", Name: "Locked", BotPresent: true},
		Caps:   permission.Capabilities{Admin: true, Manage: true, View: true, Reserve: true, Overbook: true},
	}
}

func TestFeatures_EveryServerRouteIsPremiumGated(t *testing.T) {
	// given the owner of a server without premium
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, registerServerRoutes)
	cookie := webtest.SignIn(t, d, m, "owner", *lockedServer())
	m.Access.EXPECT().Access(mock.Anything, "owner", "g1").Return(lockedServer(), nil)

	for route, p := range serverRoutes(t, d) {
		t.Run(route.Method()+" "+route.Pattern(), func(t *testing.T) {
			// when
			var rec *httptest.ResponseRecorder
			if route.Method() == http.MethodPost {
				rec = webtest.Serve(h, webtest.Post(p, url.Values{}, cookie))
			} else {
				rec = webtest.Serve(h, webtest.Get(p, cookie))
			}

			// then
			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Contains(t, rec.Body.String(), lockCopy)
		})
	}
}

func TestFeatures_NoServerRouteOpensToAnonymousVisitorsOfALockedServer(t *testing.T) {
	// given
	d, m := webtest.NewDeps(t)
	h := webtest.Handler(d, registerServerRoutes)
	m.Access.EXPECT().Public(mock.Anything, "g1").Return(lockedServerForVisitors(), nil).Maybe()

	for route, p := range serverRoutes(t, d) {
		if route.Method() != http.MethodGet {
			continue
		}
		t.Run(route.Pattern(), func(t *testing.T) {
			// when
			rec := webtest.Serve(h, webtest.Get(p, nil))

			// then
			switch rec.Code {
			case http.StatusSeeOther:
				assert.True(t, strings.HasPrefix(rec.Header().Get("Location"), "/login"))
			case http.StatusForbidden:
				assert.Contains(t, rec.Body.String(), lockCopy)
			default:
				t.Errorf("GET %s answered %d", p, rec.Code)
			}
		})
	}
}

func lockedServerForVisitors() *access.GuildAccess {
	return &access.GuildAccess{Config: guildconfig.Config{GuildID: "g1", Name: "Locked", BotPresent: true}}
}
