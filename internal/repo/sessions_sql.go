package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/rand"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/brunoluiz/jornada/internal/storage/sqldb"
	"github.com/oklog/ulid"
	"github.com/sirupsen/logrus"
)

const (
	getFields = `s.id, s.client_id, s.user_agent, s.os, s.browser, s.version, s.updated_at, s.meta, u.id, u.name, u.email`
)

type (
	// SessionSQL defines a session SQL repository
	SessionSQL struct {
		db  *sql.DB
		log *logrus.Logger
	}

	// User details about session's user
	User struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}

	// Session session model, mostly with data from user and browser used
	Session struct {
		ID        string            `json:"id"`
		ClientID  string            `json:"clientId"`
		UserAgent string            `json:"userAgent"`
		OS        string            `json:"os"`
		Browser   string            `json:"browser"`
		Version   string            `json:"version"`
		Meta      map[string]string `json:"meta"`
		User      User              `json:"user"`
		UpdatedAt time.Time         `json:"updatedAt"`
	}

	GetOpt func(b *sq.SelectBuilder)
)

// GetOrCreateID get or create an ID (based on ULID)
func (s *Session) GetOrCreateID() string {
	if s.ID != "" {
		return s.ID
	}

	t := time.Now()
	//nolint
	return ulid.MustNew(ulid.Timestamp(t), ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)).String()
}

// NewSessionSQL cretes a session repository using SQL, running the migrations on init.
// If a new migration is added, ensure that something previously created doesn't exist through
// IF NOT EXISTS operations.
func NewSessionSQL(ctx context.Context, db *sql.DB, log *logrus.Logger) (*SessionSQL, error) {
	cmds := []sqldb.Cmd{
		{
			SQL: `CREATE TABLE IF NOT EXISTS sessions (
				id TEXT PRIMARY KEY,
				client_id TEXT,
				user_id TEXT,
				user_agent TEXT,
				os TEXT,
				browser TEXT,
				version TEXT,
				meta JSON,
				updated_at DATETIME
			)`,
		},
		{
			SQL: `CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				name TEXT,
				email TEXT
			)`,
		},
		{
			// This will enable users later on to query it easily through meta
			SQL: `CREATE TABLE IF NOT EXISTS meta (
				session_id TEXT,
				key TEXT,
				value TEXT
			)`,
		},
		{
			SQL: "CREATE INDEX IF NOT EXISTS sessions_updated_at_idx ON sessions (updated_at)",
		},
	}
	if err := sqldb.Exec(ctx, db, cmds...); err != nil {
		return nil, err
	}

	return &SessionSQL{db, log}, nil
}

// Save save resource
func (store *SessionSQL) Save(ctx context.Context, in Session) error {
	meta, err := json.Marshal(in.Meta)
	if err != nil {
		return err
	}

	cmds := []sqldb.Cmd{{
		SQL: `INSERT INTO sessions (
			id,
			client_id,
			user_id,
			user_agent,
			os,
			browser,
			version,
			updated_at,
			meta
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			updated_at = EXCLUDED.updated_at,
			meta = EXCLUDED.meta
		`,
		Params: []interface{}{in.ID, in.ClientID, in.User.ID, in.UserAgent, in.OS, in.Browser, in.Version, time.Now(), meta},
	}, {
		SQL: `INSERT INTO users (
			id,
			name,
			email
		) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email
		`,
		Params: []interface{}{in.User.ID, in.User.Name, in.User.Email},
	}}

	cmds = append(cmds, sqldb.Cmd{
		SQL:    "DELETE FROM meta WHERE session_id = $1",
		Params: []interface{}{in.ID},
	})
	for k, v := range in.Meta {
		cmds = append(cmds, sqldb.Cmd{
			SQL:    "INSERT INTO meta (session_id, key, value) VALUES ($1, $2, $3)",
			Params: []interface{}{in.ID, k, v},
		})
	}

	return sqldb.Exec(ctx, store.db, cmds...)
}

// GetByID get resource by id
func (store *SessionSQL) GetByID(ctx context.Context, id string) (out Session, err error) {
	res, err := store.Get(ctx, WithSearchFilter("s.id = ?", []interface{}{id}))
	if err != nil || len(res) == 0 {
		return Session{}, err
	}
	return res[0], nil
}

func WithSearchFilter(cond string, params []interface{}) func(b *sq.SelectBuilder) {
	return func(b *sq.SelectBuilder) {
		*b = b.Where(cond, params...)
	}
}

// Get get all available resources
func (store *SessionSQL) Get(ctx context.Context, opts ...GetOpt) (out []Session, err error) {
	q := sq.Select(getFields).
		Distinct().
		From("sessions s").
		Join("users u ON s.user_id = u.id").
		LeftJoin("meta ON meta.session_id = s.id").
		OrderBy("s.updated_at DESC")
	for _, opt := range opts {
		opt(&q)
	}

	sql, params, err := q.ToSql()
	store.log.WithFields(logrus.Fields{
		"sql":    sql,
		"params": params,
	}).Info("query run")
	if err != nil {
		return out, err
	}

	rows, err := store.db.QueryContext(ctx, sql, params...)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		res, err := scanSession(rows)
		if err != nil {
			return out, err
		}

		out = append(out, res)
	}

	return out, nil
}

func scanSession(rs *sql.Rows) (Session, error) {
	var meta []byte
	var out Session
	err := rs.Scan(
		&out.ID,
		&out.ClientID,
		&out.UserAgent,
		&out.OS,
		&out.Browser,
		&out.Version,
		&out.UpdatedAt,
		&meta,
		&out.User.ID,
		&out.User.Name,
		&out.User.Email,
	)
	if err != nil {
		return out, err
	}

	if err := json.Unmarshal(meta, &out.Meta); err != nil {
		return out, err
	}

	return out, nil
}
