package sqldb

import (
	"context"
	"database/sql"
)

// Cmd define a SQL instruction to be executed by sqldb.Exec
type Cmd struct {
	SQL    string
	Params []interface{}
}

// Exec execute SQL instructions inside of a transaction block. Useful for INSERT/UPSERTs.
func Exec(ctx context.Context, db *sql.DB, cmds ...Cmd) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err := tx.Rollback(); err != nil {
			return
		}
	}()

	for _, cmd := range cmds {
		_, err := tx.ExecContext(ctx, cmd.SQL, cmd.Params...)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
