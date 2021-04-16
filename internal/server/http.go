package server

import (
	"context"
	"math/rand"
	"net/http"
	"time"

	"github.com/brunoluiz/jornada/internal/repo"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/oklog/ulid"
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

// Run start serving requests through configurations done in *Server
func (s *Server) Run(ctx context.Context) error {
	s.log.Infof("Running ⚡️ %s", s.config.Addr)
	cerr := make(chan error)
	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			cerr <- err
		}
	}()

	select {
	case err := <-cerr:
		return err
	case <-ctx.Done():
		return nil
	}
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

// New returns an HTTP server, initialising routes and middlewares
func New(
	log *logrus.Logger,
	sessions SessionRepository,
	events EventRepository,
	config Config,
) *Server {
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

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      http.HandlerFunc(s.router.ServeHTTP),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// NewAdmin Returns the admin HTTP server.
func NewAdmin(
	log *logrus.Logger,
	sessions SessionRepository,
	events EventRepository,
	config Config,
) (*Server, error) {
	s := New(log, sessions, events, config)

	if err := registerAdminRoutes(s); err != nil {
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

// NewPublic Returns the HTTP server with the public APIs, used by jornda client.
func NewPublic(
	log *logrus.Logger,
	sessions SessionRepository,
	events EventRepository,
	config Config,
) *Server {
	s := New(log, sessions, events, config)

	registerSessionRoutes(s)

	s.server = &http.Server{
		Addr:         config.Addr,
		Handler:      http.HandlerFunc(s.router.ServeHTTP),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

func genULID() string {
	t := time.Now()
	//nolint
	return ulid.MustNew(ulid.Timestamp(t), ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)).String()
}
