package http

import (
	"context"
	"errors"
	"net"
	stdhttp "net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// Server is the internal metrics and health server. Listen binds the address
// and serves in a background goroutine; Shutdown stops it.
type Server struct {
	addr string
	mux  *stdhttp.ServeMux
	log  *zap.SugaredLogger
	srv  *stdhttp.Server
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

// NewServerWithMetrics constructs a Server that serves reg on /metrics.
func NewServerWithMetrics(addr string, reg *prometheus.Registry, log *zap.SugaredLogger) *Server {
	srv := NewServer(addr, log)
	srv.Mux().Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return srv
}

// Mux returns the server's mux so callers can attach handlers.
func (s *Server) Mux() *stdhttp.ServeMux { return s.mux }

// Listen binds the address, so a busy port fails here, and then serves in the background.
func (s *Server) Listen() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.addr = listener.Addr().String()
	s.srv = &stdhttp.Server{Handler: s.mux, ReadHeaderTimeout: 10 * time.Second}
	s.log.Infow("metrics server listening", "addr", s.addr)
	go func() {
		if err := s.srv.Serve(listener); err != nil && !errors.Is(err, stdhttp.ErrServerClosed) {
			s.log.Errorw("metrics server stopped", "addr", s.addr, "error", err)
		}
	}()
	return nil
}

// Addr is the listen address. After Listen it is the bound address, with the real port.
func (s *Server) Addr() string { return s.addr }

// Shutdown stops the server. It does nothing before Listen.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
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
