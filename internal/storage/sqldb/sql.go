package sqldb

import (
	"database/sql"
	"errors"
	"net/url"

	_ "github.com/mattn/go-sqlite3" // sqlite driver
)

// New return a new instance of *sql.DB
// TODO: eventually this could support other databases and do something extra processes
// like the badger one
func New(dsn string) (*sql.DB, error) {
	dbDSN, err := url.Parse(dsn)
	if err != nil {
		return nil, err
	}

	// TODO: support proper sql here
	if dbDSN.Scheme != "sqlite" {
		return nil, errors.New(dsn + " not supported")
	}

	dbDSN.Scheme = ""
	sqliteDSN := dbDSN.String()

	db, err := sql.Open("sqlite3", sqliteDSN)
	if err != nil {
		return nil, err
	}

	return db, nil
}
