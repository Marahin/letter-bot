//go:build !devauth

package web

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDevLogin_AbsentFromAProductionBuild(t *testing.T) {
	// given the flag on, which a production build ignores
	f := newAuthFixture(t)
	f.srv.cfg.DevAuth = true
	f.h = f.srv.Handler()

	// when
	rec := f.do(htmlGet("/dev/login/700000000000000001"), nil)

	// then
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "Dev login")
}
