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

	OS struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	Browser struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	// Session session model, mostly with data from user and browser used
	Session struct {
		ID        string            `json:"id"`
		ClientID  string            `json:"clientId"`
		UserAgent string            `json:"userAgent"`
		OS        OS                `json:"os"`
		Browser   Browser           `json:"browser"`
		Device    string            `json:"device"`
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
				device TEXT,
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
			SQL: `CREATE TABLE IF NOT EXISTS oses (
				session_id TEXT PRIMARY KEY,
				name TEXT,
				version TEXT
			)`,
		},
		{
			SQL: `CREATE TABLE IF NOT EXISTS browsers (
				session_id TEXT PRIMARY KEY,
				name TEXT,
				version TEXT
			)`,
		},
		{SQL: "CREATE INDEX IF NOT EXISTS sessions_client_id_idx ON sessions (client_id)"},
		{SQL: "CREATE INDEX IF NOT EXISTS sessions_updated_at_idx ON sessions (updated_at)"},
		{SQL: "CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions (user_id)"},
		{SQL: "CREATE INDEX IF NOT EXISTS browser_name_idx ON browsers (name)"},
		{SQL: "CREATE INDEX IF NOT EXISTS browser_version_idx ON browsers (version)"},
		{SQL: "CREATE INDEX IF NOT EXISTS oses_name_idx ON oses (name)"},
		{SQL: "CREATE INDEX IF NOT EXISTS oses_version_idx ON oses (version)"},
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
		SQL: `INSERT INTO sessions (id, client_id, user_id, user_agent, device, updated_at, meta)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				user_id = EXCLUDED.user_id,
				updated_at = EXCLUDED.updated_at,
				meta = EXCLUDED.meta
			`,
		Params: []interface{}{in.ID, in.ClientID, in.User.ID, in.UserAgent, in.Device, time.Now(), meta},
	}, {
		SQL: `INSERT INTO users (id, name, email)
			VALUES ($1, $2, $3)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				email = EXCLUDED.email`,
		Params: []interface{}{in.User.ID, in.User.Name, in.User.Email},
	}, {
		SQL: `INSERT INTO browsers (session_id, name, version) VALUES ($1, $2, $3)
				ON CONFLICT (session_id) DO UPDATE SET
				name = EXCLUDED.name,
				version = EXCLUDED.version
			`,
		Params: []interface{}{in.ID, in.Browser.Name, in.Browser.Version},
	}, {
		SQL: `INSERT INTO oses (session_id, name, version) VALUES ($1, $2, $3)
				ON CONFLICT (session_id) DO UPDATE SET
				name = EXCLUDED.name,
				version = EXCLUDED.version
			`,
		Params: []interface{}{in.ID, in.OS.Name, in.OS.Version},
	},
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
	q := sq.Select(`s.id, s.client_id, s.user_agent, device, os.name, os.version, browser.name, browser.version, s.updated_at, s.meta, user.id, user.name, user.email`).
		From("sessions s").
		Join("users user ON s.user_id = user.id").
		Join("browsers browser ON s.id = browser.session_id").
		Join("oses os ON s.id = os.session_id").
		OrderBy("s.updated_at DESC")
	for _, opt := range opts {
		opt(&q)
	}

	sql, params, err := q.ToSql()
	store.log.WithFields(logrus.Fields{
		"sql":    sql,
		"params": params,
	}).Debug("query")
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
	var session Session
	err := rs.Scan(
		&session.ID,
		&session.ClientID,
		&session.UserAgent,
		&session.Device,
		&session.OS.Name,
		&session.OS.Version,
		&session.Browser.Name,
		&session.Browser.Version,
		&session.UpdatedAt,
		&meta,
		&session.User.ID,
		&session.User.Name,
		&session.User.Email,
	)
	if err != nil {
		return session, err
	}

	if err := json.Unmarshal(meta, &session.Meta); err != nil {
		return session, err
	}

	return session, nil
}
