//go:build e2e

package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorlds_AnonymousGoesFromAWorldToItsServer(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, "/worlds")

	// then
	requireHas(t, page, `header a[href="/worlds"][aria-current="page"]`)
	requireNotHas(t, page, "aside")

	// when the visitor opens the seeded world
	clickAndWaitReload(t, page, page.MustElement(`main a[href="/worlds/Celesta"]`))

	// then
	assert.Equal(t, "/worlds/Celesta", currentPath(page))
	requireHas(t, page, `main a[href="/spawns/hero%20cave"]`)

	// when the visitor opens the server on it
	clickAndWaitReload(t, page, page.MustElement(`main a[href="`+serverPath(guildID, "/stats")+`"]`))

	// then
	requireHas(t, page, "[data-public-stats]")
}

func TestSpawns_ARespawnBookedOnTwoWorlds(t *testing.T) {
	// given
	page := newPage(t)
	open(t, page, "/spawns")
	requireHas(t, page, `header a[href="/spawns"][aria-current="page"]`)

	// when
	clickAndWaitReload(t, page, page.MustElement(`main table a[href="/spawns/hero%20cave"]`))

	// then
	assert.Equal(t, "/spawns/hero%20cave", currentPath(page))
	rows := page.MustElements(`section[aria-labelledby="spawn-worlds"] tbody tr`)
	require.Len(t, rows, 2, "Celesta and Antica")
	requireHas(t, page, `section[aria-labelledby="spawn-guilds"] a[href^="`+serverPath(guildID, "/stats/spots/")+`"]`)
}

func TestSpawns_FilterByWorld(t *testing.T) {
	// given
	page := newPage(t)

	// when
	open(t, page, "/spawns?world=antica")

	// then
	requireHas(t, page, `#spawns-world option[value="antica"][selected]`)
	requireHas(t, page, `main table a[href="/spawns/banuta"]`)
	requireNotHas(t, page, `main table a[href="/spawns/library"]`)
}
