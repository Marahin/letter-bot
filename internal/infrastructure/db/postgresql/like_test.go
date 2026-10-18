package postgresql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeLike(t *testing.T) {
	cases := map[string]string{
		"":           "",
		"Hero Cave":  "Hero Cave",
		"100%":       `100\%`,
		"a_b":        `a\_b`,
		`back\slash`: `back\\slash`,
		`%_\`:        `\%\_\\`,
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			// when
			got := EscapeLike(in)

			// then
			assert.Equal(t, want, got)
		})
	}
}
