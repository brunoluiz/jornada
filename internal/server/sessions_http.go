package server

import (
	"embed"
	"encoding/json"
	"html/template"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/brunoluiz/jornada/internal/repo"
	"github.com/brunoluiz/jornada/internal/search/v1"
	"github.com/go-chi/chi"
	"github.com/oklog/ulid"
	"github.com/ua-parser/uap-go/uaparser"
)

//go:embed templates
var templates embed.FS

const (
	templatePathSessionList = "session_list.html"
	templatePathSessionByID = "session_by_id.html"

	sessionListLimit = 10
)

type sessionListParams struct {
	Sessions []repo.Session
	URL      string
	Query    string
	Error    error
	PrevPage int
	NextPage int
}

func (s *Server) registerSessionRoutes(r *chi.Mux) error {
	t, err := template.ParseFS(templates, "templates/*")
	if err != nil {
		return err
	}

	r.Get("/sessions", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil {
			page = 0
		}
		opts := []repo.GetOpt{repo.WithPagination(uint64(page)*sessionListLimit, sessionListLimit)}

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
			err = t.ExecuteTemplate(w, templatePathSessionList, sessionListParams{Sessions: data, URL: s.config.PublicURL, Query: query, Error: err, NextPage: -1, PrevPage: -1})
			s.Error(w, r, err, http.StatusInternalServerError)
			return
		}

		nextPage := page + 1
		if len(data) < sessionListLimit {
			nextPage = -1
		}

		err = t.ExecuteTemplate(w, templatePathSessionList, sessionListParams{
			Sessions: data,
			URL:      s.config.PublicURL,
			Query:    query,
			NextPage: nextPage,
			PrevPage: page - 1,
		})
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

		err = t.ExecuteTemplate(w, templatePathSessionByID, struct {
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
