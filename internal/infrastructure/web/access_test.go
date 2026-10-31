package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"spot-assistant/internal/core/dto/access"
	"spot-assistant/internal/core/dto/guildconfig"
)

func TestMustAccess_ReturnsTheGuardedAccess(t *testing.T) {
	// given
	d := newTestDeps(t)
	want := access.GuildAccess{Config: guildconfig.Config{GuildID: "g1"}}
	r := httptest.NewRequest(http.MethodGet, "/servers/g1/spots", nil).WithContext(WithCurrentAccess(context.Background(), want))
	rec := httptest.NewRecorder()

	// when
	got, ok := d.MustAccess(rec, r)

	// then
	assert.True(t, ok)
	assert.Equal(t, want, got)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestMustAccess_WithoutAGuardAnswers500(t *testing.T) {
	// given
	d := newTestDeps(t)
	rec := httptest.NewRecorder()

	// when
	_, ok := d.MustAccess(rec, httptest.NewRequest(http.MethodGet, "/servers/g1/spots", nil))

	// then
	assert.False(t, ok)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestIsHTMX(t *testing.T) {
	// given
	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	htmx := httptest.NewRequest(http.MethodGet, "/", nil)
	htmx.Header.Set("HX-Request", "true")

	// then
	assert.False(t, IsHTMX(plain))
	assert.True(t, IsHTMX(htmx))
}
