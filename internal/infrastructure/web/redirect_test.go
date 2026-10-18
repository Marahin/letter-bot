package web

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLocalURL(t *testing.T) {
	cases := map[string]bool{
		"/":                                  true,
		"/dashboard":                         true,
		"/servers/1/reservations?scope=past": true,
		"/a%2F%2Fb":                          true,
		"":                                   false,
		"dashboard":                          false,
		"//evil.com":                         false,
		`/\evil.com`:                         false,
		`/a\b`:                               false,
		`\\evil`:                             false,
		"https://evil.com":                   false,
		"javascript:alert(1)":                false,
		"/\t/evil.com":                       false,
		"/\r\n/x":                            false,
		"/\x00":                              false,
		" /x":                                false,
		"http:/x":                            false,
		"/a b":                               false,
	}
	for to, want := range cases {
		t.Run(to, func(t *testing.T) {
			// when
			got := isLocalURL(to)

			// then
			assert.Equal(t, want, got)
		})
	}
}

func TestLoginDestination(t *testing.T) {
	cases := map[string]string{
		"":                        "/dashboard",
		"/":                       "/dashboard",
		"//evil.com":              "/dashboard",
		`/\evil.com`:              "/dashboard",
		"https://evil.com":        "/dashboard",
		"/servers/1/reservations": "/servers/1/reservations",
		"/tools/loot?lang=pl":     "/tools/loot?lang=pl",
	}
	for to, want := range cases {
		t.Run(to, func(t *testing.T) {
			// when
			got := loginDestination(to)

			// then
			assert.Equal(t, want, got)
		})
	}
}
