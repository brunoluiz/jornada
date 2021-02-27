package server

import (
	"encoding/json"
	"html/template"
	"math/rand"
	"net/http"
	"time"

	"github.com/brunoluiz/jornada/internal/repo"
	"github.com/brunoluiz/jornada/internal/search/v1"
	"github.com/brunoluiz/jornada/internal/server/view"
	"github.com/go-chi/chi"
	"github.com/oklog/ulid"
	"github.com/ua-parser/uap-go/uaparser"
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
		opts := []repo.GetOpt{}
		query := r.URL.Query().Get("q")
		if query != "" {
			q, params, err := search.ToSQL(query)
			if err != nil {
				s.Error(w, r, err, http.StatusInternalServerError)
				return
			}
			opts = append(opts, repo.WithSearchFilter(q, params))
		}

		data, err := s.sessions.Get(r.Context(), opts...)
		if err != nil {
			tmplListHTML.Execute(w, struct {
				Sessions []repo.Session
				URL      string
				Query    string
				Error    error
			}{Sessions: data, URL: s.config.PublicURL, Query: query, Error: err})
			return
		}

		err = tmplListHTML.Execute(w, struct {
			Sessions []repo.Session
			URL      string
			Query    string
			Error    error
		}{Sessions: data, URL: s.config.PublicURL, Query: query})
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

			user := req.User
			if s.config.Anonymise {
				user = repo.User{}
			} else if user.ID == "" {
				user.ID = genULID()
			}

			parser := uaparser.NewFromSaved()
			ua := parser.Parse(r.UserAgent())

			rec := repo.Session{
				ID:        req.GetOrCreateID(),
				UserAgent: r.UserAgent(),
				Device:    ua.Device.ToString(),
				Browser: repo.Browser{
					Name:    ua.UserAgent.Family,
					Version: ua.UserAgent.ToVersionString(),
				},
				OS: repo.OS{
					Name:    ua.Os.Family,
					Version: ua.Os.ToVersionString(),
				},
				User: user,
				Meta: req.Meta,
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

func genULID() string {
	t := time.Now()
	//nolint
	return ulid.MustNew(ulid.Timestamp(t), ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)).String()
}
