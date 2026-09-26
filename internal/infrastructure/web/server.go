// Package web is the web delivery adapter: an SSR site (templ + htmx) with Discord
// OAuth sessions. It owns the HTTP server setup, the cross-cutting middleware and
// Deps, the app shell (layout/landing + Nav), and the embedded static assets. Each
// feature lives in its own sibling package (internal/infrastructure/<feature>-http)
// and registers its routes via Mount, so web never imports the feature packages.
package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/pgxstore"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	sessionCookieName = "letter_session"
	sessionTableName  = "web_sessions"
	sessionLifetime   = 7 * 24 * time.Hour
)

// Server wires HTTP routes to the core services.
type Server struct {
	cfg      Config
	log      *zap.SugaredLogger
	sessions *scs.SessionManager
	ping     func(context.Context) error
	// features holds the per-feature route registrars mounted by the composition
	// root; Handler() calls each with the shared Deps after the shell routes.
	features []func(*Router, *Deps)
	routes   *Router
	srv      *http.Server
}

// NewServer constructs the web server with a Postgres-backed session store.
func NewServer(cfg Config, log *zap.SugaredLogger, pool *pgxpool.Pool) *Server {
	store := pgxstore.NewWithConfig(pool, pgxstore.Config{TableName: sessionTableName, CleanUpInterval: 5 * time.Minute})
	s := newServer(cfg, log, NewSessionManager(cfg, store))
	s.ping = pool.Ping
	return s
}

func newServer(cfg Config, log *zap.SugaredLogger, sessions *scs.SessionManager) *Server {
	return &Server{cfg: cfg, log: log, sessions: sessions}
}

// NewSessionManager configures the session cookie. A nil store keeps scs's
// in-memory default, which is what tests use.
func NewSessionManager(cfg Config, store scs.Store) *scs.SessionManager {
	sessions := scs.New()
	if store != nil {
		sessions.Store = store
	}
	sessions.Lifetime = sessionLifetime
	sessions.Cookie.Name = sessionCookieName
	sessions.Cookie.HttpOnly = true
	sessions.Cookie.SameSite = http.SameSiteLaxMode
	sessions.Cookie.Secure = isHTTPS(cfg.BaseURL)
	return sessions
}

// Mount registers a feature package's route registrar. Handler() invokes each
// registrar with the shared Deps so feature routes join the same router and
// middleware chain.
func (s *Server) Mount(register func(*Router, *Deps)) {
	s.features = append(s.features, register)
}

func (s *Server) deps() *Deps {
	return &Deps{
		Cfg:      s.cfg,
		Log:      s.log,
		Sessions: s.sessions,
		Routes:   s.routes,
	}
}

// Handler builds the routed, middleware-wrapped HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	router := NewRouter(mux)
	s.routes = router
	d := s.deps()

	router.Handle(http.MethodGet, "/assets/", AssetHandler())
	// Browsers probe /favicon.ico whatever the <link> tags say. The Location is left
	// unstamped on purpose: a cached 301 outlives the release that issued it.
	router.Handle(http.MethodGet, "/favicon.ico",
		http.RedirectHandler("/assets/favicon-32.png", http.StatusMovedPermanently))

	router.Get("/{$}", s.handleLanding)
	// Catch-all for unmatched paths: brand the 404 instead of the stdlib default.
	// More specific routes (including the features' below) win.
	router.Get("/", s.handleNotFound)
	// Not behind sign-in: anonymous visitors switch language too.
	router.Post("/language", s.handleSetLanguage)
	if s.ping != nil {
		router.Handle(http.MethodGet, "/healthz", HealthzHandler(s.ping))
	}

	for _, register := range s.features {
		register(router, d)
	}

	// LogMiddleware sits inside LoadAndSave so it can read the session. CSRFMiddleware
	// sits outside it so a forged mutation is refused before it costs a session read.
	// WithLocale is outermost: RecoverMiddleware renders its 500 from the request it
	// was handed, so a locale resolved further in would never reach that page.
	return d.WithLocale(d.RecoverMiddleware(d.CSRFMiddleware(s.sessions.LoadAndSave(d.LogMiddleware(mux)))))
}

// isHTTPS reports whether the configured base URL uses TLS, controlling the
// Secure flag on the cookies.
func isHTTPS(baseURL string) bool {
	return strings.HasPrefix(strings.ToLower(baseURL), "https://")
}

// Start begins serving and blocks until the server stops.
func (s *Server) Start() error {
	s.srv = &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	s.log.Infow("web server listening", "addr", s.cfg.Addr)
	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}
