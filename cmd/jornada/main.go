package main

import (
	"os"

	"github.com/brunoluiz/jornada/internal/op/logger"
	"github.com/brunoluiz/jornada/internal/repo"
	"github.com/brunoluiz/jornada/internal/server"
	"github.com/brunoluiz/jornada/internal/storage/badgerdb"
	"github.com/brunoluiz/jornada/internal/storage/sqldb"
	_ "github.com/joho/godotenv/autoload"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "public-url", Value: "http://localhost:3000", EnvVars: []string{"PUBLIC_URL"}, Usage: "Public URL where the service is exposed. The service might be running on :3000, but the public access can be proxied through 80"},
			&cli.BoolFlag{Name: "non-anonymised-mode", EnvVars: []string{"NON_ANONYMISED_MODE"}, Usage: "If set, it will allow user details to be recorded"},
			&cli.StringFlag{Name: "address", Value: "0.0.0.0", EnvVars: []string{"ADDRESS"}, Usage: "Service address -- change to 127.0.0.1 if developing on Mac (avoids network warnings)"},
			&cli.StringFlag{Name: "port", Value: "3000", EnvVars: []string{"PORT"}, Usage: "Service port"},
			&cli.StringSliceFlag{Name: "allowed-origins", Value: cli.NewStringSlice("*"), EnvVars: []string{"ALLOWED_ORIGINS"}, Usage: "CORS allowed origins"},
			&cli.StringFlag{Name: "db-dsn", Value: "sqlite:///tmp/jornada.db?cache=shared&mode=rwc&_journal_mode=WAL", EnvVars: []string{"DB_DSN"}, Usage: "DSN for SQL database (see github.com/mattn/go-sqlite3 for more options)"},
			&cli.StringFlag{Name: "events-dsn", Value: "badger:///tmp/jornada.events", EnvVars: []string{"EVENTS_DSN"}, Usage: "Events storage path (BadgerDB)"},
			&cli.StringFlag{Name: "log-level", Value: "info", EnvVars: []string{"LOG_LEVEL"}, Usage: "Log level"},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		logger.New("error").Fatal(err)
	}
}

func run(c *cli.Context) error {
	ctx := c.Context
	log := logger.New(c.String("log-level"))

	b, err := badgerdb.New(c.String("events-dsn"), log)
	if err != nil {
		return err
	}
	defer b.Close()

	db, err := sqldb.New(c.String("db-dsn"))
	if err != nil {
		return err
	}
	defer db.Close()

	events := repo.NewEventBadger(b.BadgerDB)
	recordings, err := repo.NewSessionSQL(ctx, db)
	if err != nil {
		return err
	}

	server, err := server.New(
		log,
		recordings,
		events,
		server.Config{
			Addr:           c.String("address") + ":" + c.String("port"),
			PublicURL:      c.String("public-url"),
			AllowedOrigins: c.StringSlice("allowed-origins"),
			Anonymise:      !c.Bool("non-anonymised-mode"),
		},
	)
	if err != nil {
		return err
	}

	return server.Run()
}
