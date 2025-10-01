package http

import (
	stdhttp "net/http"

	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"spot-assistant/internal/ports"
)

// Server is a simple HTTP server wrapper living in the infrastructure layer.
// It exposes a mux for registering handlers (e.g., Prometheus /metrics) and
// starts listening in a background goroutine.
type Server struct {
	addr string
	mux  *stdhttp.ServeMux
	log  *zap.SugaredLogger
}

// NewServer constructs a new Server with the provided address and logger.
// If addr is empty, it defaults to ":2112".
func NewServer(addr string, log *zap.SugaredLogger) *Server {
	if addr == "" {
		addr = ":2112"
	}
	return &Server{
		addr: addr,
		mux:  stdhttp.NewServeMux(),
		log:  log.With("layer", "infrastructure", "name", "http"),
	}
}

// Mux returns the server's mux so callers can attach handlers.
func (s *Server) Mux() *stdhttp.ServeMux { return s.mux }

// Start begins listening and serving HTTP requests in a background goroutine.
func (s *Server) Start() {
	go func() {
		if err := stdhttp.ListenAndServe(s.addr, s.mux); err != nil {
			s.log.With("addr", s.addr).Warnf("http server stopped: %v", err)
		}
	}()
}

// NewMetricsServer constructs a Server pre-configured with Prometheus /metrics handler.
func NewMetricsServer(addr string, log *zap.SugaredLogger) *Server {
	srv := NewServer(addr, log)
	srv.Mux().Handle("/metrics", promhttp.Handler())
	return srv
}

// CheckFunc is a function that returns nil if the check passes.
type CheckFunc func() error

// WithHealth registers liveness and readiness endpoints using provided checks.
//
// - /livez returns 200 if live() == nil, otherwise 503.
// - /readyz returns 200 if ready() == nil, otherwise 503.
func (s *Server) WithHealth(live, ready CheckFunc) *Server {
	s.mux.HandleFunc("/livez", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if live == nil || live() == nil {
			w.WriteHeader(stdhttp.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(stdhttp.StatusServiceUnavailable)
		_, _ = w.Write([]byte("unhealthy"))
	})
	s.mux.HandleFunc("/readyz", func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if ready != nil && ready() == nil {
			w.WriteHeader(stdhttp.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(stdhttp.StatusServiceUnavailable)
		_, _ = w.Write([]byte("not ready"))
	})

	return s
}

// WithHealthProvider registers health endpoints using a HealthPort implementation.
func (s *Server) WithHealthProvider(h ports.HealthPort) *Server {
	if h == nil {
		return s
	}
	return s.WithHealth(h.Live, h.Ready)
}
