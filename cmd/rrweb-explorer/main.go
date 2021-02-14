package main

// ☢️ WARNING: If you are alergic to messy cowboy codes, please don't read the code below ☢️
//
// # Intro:
//
// This snippet of magic is to test rrweb as a possible replacement to FullStory. It uses badger as storage because I
// didn't want to deal with setting up a container and migrations for storage.
// > FullStory is a tool to record user sessions for further analysis (can be for debugging, UX etc)
//
// # Things which would require more work:
//
// - Easy search: FullStory has an easy search tool while here it... well, it is inexistent
// - Events integration: FullStory recognises GA events... this is not supported at the moment
// - Storage: I just picked badgerdb because it was easy win, but it needs to be replaced ASAP (probably with S3 for events and SQL for data)
//
// # Endpoints:
//
// - /: list all recorded sessions
// - /records/{id}: load the session details and player
// - /api/v1/records/{id}: retrive record by ID (api used by the player JS)
// - /record.js: used in the target application to send data to the server
//
// # Usage:
// To use this, add the following snippet to your app and then head to http://localhost:3000 to see recorded sessions
// ```js
// <script type="application/javascript" src="http://localhost:3000/record.js" ></script>
// <script type="application/javascript">
//   window.recorder.setUser({ id: 'USER_ID', email: 'test@test.com', name: 'Bruno Luiz Silva' }).setMeta({ foo: 'bar' }).setClientId('client-id')
// </script>
// ```
//
// # To-do list after testing the hack:
//
// - [ ] See if this makes sense
// - [ ] Proper UI -- could be kept as server-side rendered
// - [ ] Make it searchable through generic parameters (example: client_id, appplication_id, user_id etc), without full scans
// - [ ] Reconsider storage (badger was a quick win)

import (
	"database/sql"
	"log"
	"net/url"
	"os"

	"github.com/brunoluiz/rrweb-explorer/internal/http"
	"github.com/brunoluiz/rrweb-explorer/internal/storage"
	_ "github.com/joho/godotenv/autoload"
	_ "github.com/mattn/go-sqlite3"
	"github.com/urfave/cli/v2"
)

func run(c *cli.Context) error {
	ctx := c.Context
	dbDSN, err := url.Parse(c.String("db-dsn"))
	if err != nil {
		panic(err)
	}

	eventsDSN, err := url.Parse(c.String("events-dsn"))
	if err != nil {
		panic(err)
	}

	badgerDB, err := storage.NewBadgerStore(eventsDSN.Path, 0)
	if err != nil {
		return err
	}
	defer badgerDB.Close()

	db, err := sql.Open("sqlite3", dbDSN.Path)
	if err != nil {
		return err
	}
	defer db.Close()

	recordings, err := storage.NewRecordingSQLite(ctx, db)
	if err != nil {
		return err
	}

	events := storage.NewEventStoreBadger(badgerDB.BadgerDB)

	return http.New(
		c.String("address")+":"+c.String("port"),
		c.String("service-url"),
		c.StringSlice("allowed-domains"),
		recordings,
		events,
	).Open()
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "service-url", Value: "http://localhost:3000", EnvVars: []string{"SERVICE_URL"}},
			&cli.StringFlag{Name: "address", Value: "127.0.0.1", EnvVars: []string{"ADDRESS"}},
			&cli.StringSliceFlag{Name: "allowed-domains", Value: cli.NewStringSlice("*"), EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "db-dsn", Value: "sqlite:///tmp/rrweb-explorer.db", EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "events-dsn", Value: "badger:///tmp/rrweb-explorer-events", EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "port", Value: "3000", EnvVars: []string{"PORT"}},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
