package http

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"spot-assistant/internal/common/test/mocks"
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
	srv.WithHealth(hp.Live, hp.Ready)

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
	srv.WithHealth(hp.Live, hp.Ready)

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

func TestListen_ServesUntilShutdown(t *testing.T) {
	// given
	reg := prometheus.NewRegistry()
	srv := NewServerWithMetrics("127.0.0.1:0", reg, zap.NewNop().Sugar()).WithHealth(nil, nil)

	// when
	require.NoError(t, srv.Listen())
	resp, err := http.Get("http://" + srv.Addr() + "/livez")
	require.NoError(t, err)
	_ = resp.Body.Close()
	metricsResp, err := http.Get("http://" + srv.Addr() + "/metrics")
	require.NoError(t, err)
	_ = metricsResp.Body.Close()
	shutdownErr := srv.Shutdown(context.Background())

	// then
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, http.StatusOK, metricsResp.StatusCode)
	assert.NoError(t, shutdownErr)
	after, err := http.Get("http://" + srv.Addr() + "/livez")
	if after != nil {
		_ = after.Body.Close()
	}
	assert.Error(t, err)
}

func TestListen_FailsOnABusyPort(t *testing.T) {
	// given
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	srv := NewServer(listener.Addr().String(), zap.NewNop().Sugar())

	// when
	err = srv.Listen()

	// then
	assert.Error(t, err)
}

func TestShutdown_WithoutListenDoesNothing(t *testing.T) {
	// given
	srv := NewServer("127.0.0.1:0", zap.NewNop().Sugar())

	// when
	err := srv.Shutdown(context.Background())

	// then
	assert.NoError(t, err)
}
