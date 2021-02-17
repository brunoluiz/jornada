package main

import (
	"log"
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
			&cli.StringFlag{Name: "service-url", Value: "http://localhost:3000", EnvVars: []string{"SERVICE_URL"}},
			&cli.StringFlag{Name: "address", Value: "127.0.0.1", EnvVars: []string{"ADDRESS"}},
			&cli.StringSliceFlag{Name: "allowed-domains", Value: cli.NewStringSlice("*"), EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "db-dsn", Value: "sqlite:///tmp/jornada.db?cache=shared&mode=rwc&_journal_mode=WAL", EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "events-dsn", Value: "badger:///tmp/jornada.events", EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "port", Value: "3000", EnvVars: []string{"PORT"}},
			&cli.StringFlag{Name: "log-level", Value: "info", EnvVars: []string{"LOG_LEVEL"}},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
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
		c.String("address")+":"+c.String("port"),
		c.String("service-url"),
		c.StringSlice("allowed-domains"),
		log,
		recordings,
		events,
	)
	if err != nil {
		return err
	}

	return server.Open()
}
