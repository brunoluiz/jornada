package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/brunoluiz/jornada/internal/repo"
	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
)

// SessionRepository defines a session repository
type SessionRepository interface {
	Save(ctx context.Context, in repo.Session) error
	GetByID(ctx context.Context, id string) (repo.Session, error)
	GetAll(ctx context.Context, offset string, limit int) ([]repo.Session, error)
}

// EventRepository defines an events repository
type EventRepository interface {
	Add(ctx context.Context, id string, msgs ...[]byte) error
	Get(ctx context.Context, id string, cb func(b []byte, pos, size uint64) error) error
}

// Server defines an HTTP Server
type Server struct {
	address    string
	serviceURL string
	server     *http.Server
	router     *chi.Mux
	sessions   SessionRepository
	events     EventRepository
}

// New returns an HTTP server, initialising routes and middlewares
func New(
	addr string,
	serviceURL string,
	allowedOrigins []string,
	sessions SessionRepository,
	events EventRepository,
) (*Server, error) {
	s := &Server{
		address:    addr,
		serviceURL: serviceURL,
		router:     chi.NewRouter(),
		sessions:   sessions,
		events:     events,
	}

	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sessions", http.StatusTemporaryRedirect)
	})

	if err := s.registerSessionRoutes(s.router); err != nil {
		return nil, err
	}

	s.server = &http.Server{
		Addr:         addr,
		Handler:      http.HandlerFunc(s.router.ServeHTTP),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s, nil
}

// Open start serving requests through configurations done in *Server
func (s *Server) Open() error {
	log.Println("Running on " + s.address)
	return s.server.ListenAndServe()
}

// Close http server graceful shutdown
func (s *Server) Close() error {
	tCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	return s.server.Shutdown(tCtx)
}
