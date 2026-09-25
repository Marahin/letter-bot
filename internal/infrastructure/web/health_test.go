package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHealthzHandler(t *testing.T) {
	tests := []struct {
		name     string
		pingErr  error
		wantCode int
		wantBody string
	}{
		{name: "healthy", wantCode: http.StatusOK, wantBody: "ok"},
		{name: "database down", pingErr: errors.New("down"), wantCode: http.StatusServiceUnavailable, wantBody: "unhealthy\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			handler := HealthzHandler(func(ctx context.Context) error { return tt.pingErr })
			rec := httptest.NewRecorder()

			// when
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

			// then
			assert.Equal(t, tt.wantCode, rec.Code)
			assert.Equal(t, tt.wantBody, rec.Body.String())
		})
	}
}
