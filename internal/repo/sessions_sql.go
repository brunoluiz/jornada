package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/brunoluiz/jornada/internal/storage/sqldb"
)

// SessionSQL defines a session SQL repository
type SessionSQL struct {
	db *sql.DB
}

// Client keeps the session client information, mostly parsed from .UserAgent
type Client struct {
	UserAgent string `json:"userAgent"`
	OS        string `json:"os"`
	Browser   string `json:"browser"`
	Version   string `json:"version"`
}

// User details about session's user
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Session session model, mostly with data from user and browser used
type Session struct {
	ID        string            `json:"id"`
	ClientID  string            `json:"clientId"`
	Meta      map[string]string `json:"meta"`
	User      User              `json:"user"`
	Client    Client            `json:"client"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

// NewSessionSQL cretes a session repository using SQL, running the migrations on init.
// If a new migration is added, ensure that something previously created doesn't exist through
// IF NOT EXISTS operations.
func NewSessionSQL(ctx context.Context, db *sql.DB) (*SessionSQL, error) {
	cmds := []sqldb.Cmd{
		{
			SQL: `CREATE TABLE IF NOT EXISTS sessions (
				id TEXT PRIMARY KEY,
				client_id TEXT,
				user_id TEXT,
				os TEXT,
				browser TEXT,
				version TEXT,
				meta JSON,
				updated_at DATE
			)`,
		},
		{
			SQL: `CREATE TABLE IF NOT EXISTS users (
				id TEXT PRIMARY KEY,
				name TEXT,
				email TEXT
			)`,
		},
	}
	if err := sqldb.Exec(ctx, db, cmds...); err != nil {
		return nil, err
	}

	return &SessionSQL{db}, nil
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
			os,
			browser,
			version,
			updated_at,
			meta
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			client_id = EXCLUDED.client_id,
			user_id = EXCLUDED.user_id,
			os = EXCLUDED.os,
			browser = EXCLUDED.browser,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at,
			meta = EXCLUDED.meta
		`,
		Params: []interface{}{in.ID, in.ClientID, in.User.ID, in.Client.OS, in.Client.Browser, in.Client.Version, time.Now(), meta},
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

	return sqldb.Exec(ctx, store.db, cmds...)
}

// GetByID get resource by id
func (store *SessionSQL) GetByID(ctx context.Context, id string) (Session, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT
		s.id,
		s.client_id,
		s.os,
		s.browser,
		s.version,
		s.updated_at,
		u.id,
		u.name,
		u.email
	FROM sessions s
	JOIN users u ON s.user_id = u.id
	WHERE s.id = $1`, id)
	if err != nil {
		return Session{}, err
	}
	defer rows.Close()

	var in Session
	for rows.Next() {
		err = rows.Scan(
			&in.ID,
			&in.ClientID,
			&in.Client.OS,
			&in.Client.Browser,
			&in.Client.Version,
			&in.UpdatedAt,
			&in.User.ID,
			&in.User.Name,
			&in.User.Email,
		)
		if err != nil {
			return Session{}, err
		}
	}

	return in, nil
}

// GetAll get all available resources
func (store *SessionSQL) GetAll(ctx context.Context, offset string, limit int) ([]Session, error) {
	out := []Session{}

	rows, err := store.db.QueryContext(ctx, `SELECT
		s.id,
		s.client_id,
		s.os,
		s.browser,
		s.version,
		s.updated_at,
		s.meta,
		u.id,
		u.name,
		u.email
	FROM sessions s
	JOIN users u ON s.user_id = u.id
	ORDER BY s.updated_at DESC`)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var meta []byte
		var in Session
		err = rows.Scan(
			&in.ID,
			&in.ClientID,
			&in.Client.OS,
			&in.Client.Browser,
			&in.Client.Version,
			&in.UpdatedAt,
			&meta,
			&in.User.ID,
			&in.User.Name,
			&in.User.Email,
		)
		if err != nil {
			return out, err
		}

		if err := json.Unmarshal(meta, &in.Meta); err != nil {
			return out, err
		}

		out = append(out, in)
	}

	return out, nil
}
