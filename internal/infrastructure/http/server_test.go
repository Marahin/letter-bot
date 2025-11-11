package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	mocks "spot-assistant/internal/common/test/mocks"
)

func newTestLogger() *zap.SugaredLogger {
	l, _ := zap.NewDevelopment()
	return l.Sugar()
}

func TestHealthEndpoints_OK(t *testing.T) {
	// given
	log := newTestLogger()
	srv := NewServer(":0", log)
	hp := &mocks.MockHealthPort{}
	hp.On("Live").Return(nil)
	hp.On("Ready").Return(nil)
	srv.WithHealthProvider(hp)

	// when
	liveReq := httptest.NewRequest(http.MethodGet, "/livez", nil)
	liveRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(liveRec, liveReq)

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(readyRec, readyReq)

	// then
	assert.Equal(t, http.StatusOK, liveRec.Code)
	assert.Equal(t, "ok", liveRec.Body.String())
	assert.Equal(t, http.StatusOK, readyRec.Code)
	assert.Equal(t, "ok", readyRec.Body.String())
}

func TestHealthEndpoints_Failures(t *testing.T) {
	// given
	log := newTestLogger()
	srv := NewServer(":0", log)
	hp := &mocks.MockHealthPort{}
	hp.On("Live").Return(assert.AnError)
	hp.On("Ready").Return(assert.AnError)
	srv.WithHealthProvider(hp)

	// when
	liveReq := httptest.NewRequest(http.MethodGet, "/livez", nil)
	liveRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(liveRec, liveReq)

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	srv.mux.ServeHTTP(readyRec, readyReq)

	// then
	assert.Equal(t, http.StatusServiceUnavailable, liveRec.Code)
	assert.Equal(t, "unhealthy", liveRec.Body.String())
	assert.Equal(t, http.StatusServiceUnavailable, readyRec.Code)
	assert.Equal(t, "not ready", readyRec.Body.String())
}
