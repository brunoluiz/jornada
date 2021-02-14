package http

import (
	"encoding/json"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/brunoluiz/rrweb-explorer/internal/http/view"
	"github.com/brunoluiz/rrweb-explorer/internal/storage"
	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"github.com/mssola/user_agent"
	"github.com/oklog/ulid"
)

type Server struct {
	serviceURL string
	server     *http.Server
	router     *chi.Mux
	recordings *storage.RecordingStoreSQL
	events     *storage.EventStore
}

func New(
	addr string,
	serviceURL string,
	allowedOrigins []string,
	recordings *storage.RecordingStoreSQL,
	events *storage.EventStore,
) *Server {
	s := &Server{
		server: &http.Server{
			Addr: addr,
		},
		router:     chi.NewRouter(),
		recordings: recordings,
		events:     events,
	}

	s.router.Use(cors.Handler(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/records", http.StatusPermanentRedirect)
	})

	s.registerRecordRoutes(s.router)
	s.server.Handler = http.HandlerFunc(s.router.ServeHTTP)

	return s
}

func (s *Server) registerRecordRoutes(r *chi.Mux) error {
	tmplRecorder, err := template.New("recorder").Parse(view.JSRecorder)
	if err != nil {
		return err
	}

	tmplPlayerHTML, err := template.New("player_html").Parse(view.HTMLRecordByID)
	if err != nil {
		return err
	}

	tmplListHTML, err := template.New("list_html").Parse(view.HTMLRecordList)
	if err != nil {
		return err
	}

	// Renders a basic record script
	r.Get("/record.js", func(w http.ResponseWriter, r *http.Request) {
		err := tmplRecorder.Execute(w, struct {
			URL string
		}{URL: s.serviceURL})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Renders records list
	r.Get("/records", func(w http.ResponseWriter, r *http.Request) {
		records, err := s.recordings.GetAll(r.Context(), "", 10)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmplListHTML.Execute(w, struct {
			Records []storage.Record
			URL     string
		}{Records: records, URL: s.serviceURL})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	r.Get("/records/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		rec, err := s.recordings.GetByID(r.Context(), id)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmplPlayerHTML.Execute(w, struct {
			ID     string
			Record storage.Record
		}{ID: id, Record: rec})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	r.Route("/api/v1/records", func(r chi.Router) {
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			rec, err := s.recordings.GetByID(r.Context(), id)
			if err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if err := json.NewEncoder(w).Encode(&rec); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})

		r.Get("/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			w.Write([]byte("["))
			s.events.Get(r.Context(), id, func(b []byte, last bool) error {
				if !last {
					b = append(b, ',', '\n')
				}
				_, err := w.Write(b)
				return err
			})
			w.Write([]byte("]"))
		})

		// Entry point for recordings
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var req storage.Record
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			id := req.ID
			if id == "" {
				t := time.Now()
				id = ulid.MustNew(ulid.Timestamp(t), ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)).String()
			}

			ua := user_agent.New(r.UserAgent())
			browserName, browserVersion := ua.Browser()

			rec := storage.Record{
				ID:   id,
				User: req.User,
				Meta: req.Meta,
				Client: storage.Client{
					UserAgent: r.UserAgent(),
					OS:        ua.OS(),
					Browser:   browserName,
					Version:   browserVersion,
				},
			}

			if err := s.recordings.Save(r.Context(), rec); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if err := json.NewEncoder(w).Encode(&rec); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})

		r.Put("/records/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			req := []interface{}{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			jsons := [][]byte{}
			for _, v := range req {
				event, err := json.Marshal(v)
				if err != nil {
					log.Println(err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				jsons = append(jsons, event)
			}

			if err := s.events.Add(r.Context(), id, jsons...); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		})
	})

	return nil
}

func (s *Server) Open() error {
	return s.server.ListenAndServe()
}
