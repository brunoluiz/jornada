package server

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"text/template"
	"time"

	"github.com/brunoluiz/jornada/internal/repo"
	"github.com/brunoluiz/jornada/internal/server/view"
	"github.com/go-chi/chi"
	"github.com/mssola/user_agent"
	"github.com/oklog/ulid"
)

func (s *Server) registerSessionRoutes(r *chi.Mux) error {
	tmplRecorder, err := template.New("recorder").Parse(view.JSRecorder)
	if err != nil {
		return err
	}

	tmplPlayerHTML, err := template.New("player_html").Parse(view.HTMLSessionByID)
	if err != nil {
		return err
	}

	tmplListHTML, err := template.New("list_html").Parse(view.HTMLSessionList)
	if err != nil {
		return err
	}

	r.Get("/record.js", func(w http.ResponseWriter, r *http.Request) {
		err := tmplRecorder.Execute(w, struct {
			URL string
		}{URL: s.config.PublicURL})
		if err != nil {
			s.Error(w, r, err, http.StatusInternalServerError)
			return
		}
	})

	r.Get("/sessions", func(w http.ResponseWriter, r *http.Request) {
		data, err := s.sessions.GetAll(r.Context(), "", 10)
		if err != nil {
			s.Error(w, r, err, http.StatusInternalServerError)
			return
		}

		err = tmplListHTML.Execute(w, struct {
			Sessions []repo.Session
			URL      string
		}{Sessions: data, URL: s.config.PublicURL})
		if err != nil {
			s.Error(w, r, err, http.StatusInternalServerError)
			return
		}
	})

	r.Get("/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		rec, err := s.sessions.GetByID(r.Context(), id)
		if err != nil {
			s.Error(w, r, err, http.StatusInternalServerError)
			return
		}

		err = tmplPlayerHTML.Execute(w, struct {
			ID      string
			Session repo.Session
		}{ID: id, Session: rec})
		if err != nil {
			s.Error(w, r, err, http.StatusInternalServerError)
			return
		}
	})

	r.Route("/api/v1/sessions", func(r chi.Router) {
		r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			rec, err := s.sessions.GetByID(r.Context(), id)
			if err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}

			if err := json.NewEncoder(w).Encode(&rec); err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}
		})

		// TODO: this might be better off if delivered as a stream or if the player is configured to have request chunks instead of all
		r.Get("/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			err := s.events.Get(r.Context(), id, func(b []byte, pos, size uint64) error {
				if pos == 0 {
					bshadow := make([]byte, len(b)+1)
					bshadow[0] = '['
					copy(bshadow[1:], b)
					b = bshadow
					b = append(b, ',', '\n')
				} else if pos == (size - 1) {
					b = append(b, ']')
				} else {
					b = append(b, ',', '\n')
				}
				_, err := w.Write(b)
				return err
			})
			if err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}
		})

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var req repo.Session
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}

			id := req.ID
			if id == "" {
				t := time.Now()
				//nolint
				id = ulid.MustNew(ulid.Timestamp(t), ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)).String()
			}

			if s.config.Anonymise {
				req.User.Email = ""
				req.User.Name = ""
			}

			ua := user_agent.New(r.UserAgent())
			browserName, browserVersion := ua.Browser()

			rec := repo.Session{
				ID:        id,
				UserAgent: r.UserAgent(),
				OS:        ua.OS(),
				Browser:   browserName,
				Version:   browserVersion,
				User:      req.User,
				Meta:      req.Meta,
			}

			if err := s.sessions.Save(r.Context(), rec); err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}

			if err := json.NewEncoder(w).Encode(&rec); err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}
		})

		r.Put("/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			req := []interface{}{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}

			jsons := [][]byte{}
			for _, v := range req {
				event, err := json.Marshal(v)
				if err != nil {
					s.Error(w, r, err, http.StatusInternalServerError)
					return
				}
				jsons = append(jsons, event)
			}

			if err := s.events.Add(r.Context(), id, jsons...); err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
		})
	})

	return nil
}
