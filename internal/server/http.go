package server

import (
	"context"
	"net/http"
	"time"

	"github.com/brunoluiz/jornada/internal/repo"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

// SessionRepository defines a session repository
type SessionRepository interface {
	Save(ctx context.Context, in repo.Session) error
	GetByID(ctx context.Context, id string) (repo.Session, error)
	Get(ctx context.Context, opts ...repo.GetOpt) ([]repo.Session, error)
}

// EventRepository defines an events repository
type EventRepository interface {
	Add(ctx context.Context, id string, msgs ...[]byte) error
	Get(ctx context.Context, id string, cb func(b []byte, pos, size uint64) error) error
}

// Server defines an HTTP Server
type Server struct {
	config   Config
	log      *logrus.Logger
	server   *http.Server
	router   *chi.Mux
	sessions SessionRepository
	events   EventRepository
}

// Config server configs
type Config struct {
	Addr           string
	PublicURL      string
	AllowedOrigins []string
	Anonymise      bool
}

// New returns an HTTP server, initialising routes and middlewares
func New(
	log *logrus.Logger,
	sessions SessionRepository,
	events EventRepository,
	config Config,
) (*Server, error) {
	s := &Server{
		config:   config,
		log:      log,
		router:   chi.NewRouter(),
		sessions: sessions,
		events:   events,
	}

	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.Timeout(60 * time.Second))
	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins: config.AllowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	s.router.Handle("/__/metrics", promhttp.Handler())
	s.router.Mount("/__/debug", middleware.Profiler())
	s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sessions", http.StatusTemporaryRedirect)
	})

	if err := s.registerSessionRoutes(s.router); err != nil {
		return nil, err
	}

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      http.HandlerFunc(s.router.ServeHTTP),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s, nil
}

// Run start serving requests through configurations done in *Server
func (s *Server) Run(_ context.Context) error {
	s.log.Infof("Running ⚡️ %s", s.config.Addr)
	return s.server.ListenAndServe()
}

// Close http server graceful shutdown
func (s *Server) Close() error {
	tCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	return s.server.Shutdown(tCtx)
}

// Error handle http errors
func (s *Server) Error(w http.ResponseWriter, r *http.Request, err error, code int) {
	if err == nil {
		return
	}

	s.log.Error(err)
	http.Error(w, err.Error(), code)
}
