package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type RecordingStoreSQL struct {
	db *sql.DB
}

// Client keeps the recording client information, mostly parsed from .UserAgent
type Client struct {
	UserAgent string `json:"userAgent"`
	OS        string `json:"os"`
	Browser   string `json:"browser"`
	Version   string `json:"version"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Record recording record model, mostly with data from the events, user and browser used
type Record struct {
	ID        string            `json:"id"`
	ClientID  string            `json:"clientId"`
	Meta      map[string]string `json:"meta"`
	User      User              `json:"user"`
	Client    Client            `json:"client"`
	UpdatedAt time.Time         `json:"updatedAt"`
}

type SQLCmd struct {
	SQL    string
	Params []interface{}
}

func execTx(ctx context.Context, db *sql.DB, cmds ...SQLCmd) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, cmd := range cmds {
		_, err := tx.ExecContext(ctx, cmd.SQL, cmd.Params...)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func NewRecordingSQLite(ctx context.Context, db *sql.DB) (*RecordingStoreSQL, error) {
	cmds := []SQLCmd{
		{SQL: `CREATE TABLE IF NOT EXISTS recordings (
			id TEXT PRIMARY KEY,
			client_id TEXT,
			user_id TEXT,
			os TEXT,
			browser TEXT,
			version TEXT,
			meta JSON,
			updated_at DATE
		)`},
		{SQL: `CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			name TEXT,
			email TEXT
		)`},
	}
	if err := execTx(ctx, db, cmds...); err != nil {
		return nil, err
	}

	return &RecordingStoreSQL{db}, nil
}

func (store *RecordingStoreSQL) Save(ctx context.Context, rec Record) error {
	meta, err := json.Marshal(rec.Meta)
	if err != nil {
		return err
	}

	cmds := []SQLCmd{{
		SQL: `INSERT INTO recordings (
			id,
			client_id,
			user_id,
			os,
			browser,
			version,
			updated_at,
			meta
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			client_id = EXCLUDED.client_id,
			user_id = EXCLUDED.user_id,
			os = EXCLUDED.os,
			browser = EXCLUDED.browser,
			version = EXCLUDED.version,
			updated_at = EXCLUDED.updated_at,
			meta = EXCLUDED.meta
		`,
		Params: []interface{}{rec.ID, rec.ClientID, rec.User.ID, rec.Client.OS, rec.Client.Browser, rec.Client.Version, time.Now(), meta},
	}, {
		SQL: `INSERT INTO users (
			id,
			name,
			email
		) VALUES (?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email
		`,
		Params: []interface{}{rec.User.ID, rec.User.Name, rec.User.Email},
	}}

	return execTx(ctx, store.db, cmds...)
}

func (store *RecordingStoreSQL) GetByID(ctx context.Context, id string) (Record, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT
		r.id,
		r.client_id,
		r.os,
		r.browser,
		r.version,
		r.updated_at,
		u.id,
		u.name,
		u.email
	FROM recordings r
	JOIN users u ON r.user_id = u.id
	WHERE r.id = ?`, id)
	if err != nil {
		return Record{}, err
	}
	defer rows.Close()

	var rec Record
	for rows.Next() {
		err = rows.Scan(&rec.ID, &rec.ClientID, &rec.Client.OS, &rec.Client.Browser, &rec.Client.Version, &rec.UpdatedAt, &rec.User.ID, &rec.User.Name, &rec.User.Email)
		if err != nil {
			return Record{}, err
		}
	}

	return rec, nil
}

func (store *RecordingStoreSQL) GetAll(ctx context.Context, offset string, limit int) ([]Record, error) {
	records := []Record{}

	rows, err := store.db.QueryContext(ctx, `SELECT
		r.id,
		r.client_id,
		r.os,
		r.browser,
		r.version,
		r.updated_at,
		r.meta,
		u.id,
		u.name,
		u.email
	FROM recordings r
	JOIN users u ON r.user_id = u.id
	ORDER BY r.updated_at DESC`)
	if err != nil {
		return records, err
	}
	defer rows.Close()

	for rows.Next() {
		var meta []byte
		var rec Record
		err = rows.Scan(&rec.ID, &rec.ClientID, &rec.Client.OS, &rec.Client.Browser, &rec.Client.Version, &rec.UpdatedAt, &meta, &rec.User.ID, &rec.User.Name, &rec.User.Email)
		if err != nil {
			return records, err
		}

		if err := json.Unmarshal(meta, &rec.Meta); err != nil {
			return records, err
		}

		records = append(records, rec)
	}

	return records, nil
}
