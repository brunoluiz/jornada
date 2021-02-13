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
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"text/template"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mssola/user_agent"
	"github.com/urfave/cli/v2"
	"github.com/zippoxer/bow"
)

const rrwebRecord = `
window.recorder = {
	events: [],
	rrweb: undefined,
	runner: undefined,
	session: {
		genId(length) {
			const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
			let result = "";
			const charactersLength = characters.length;
			for (let i = 0; i < length; i++) {
				result += characters.charAt(Math.floor(Math.random() * charactersLength));
			}
			return result;
		},
		get() {
			let session = window.sessionStorage.getItem('rrweb');
			if (session) return JSON.parse(session);

			session = {
				id: window.recorder.session.genId(64),
				user: { id: window.recorder.session.genId(64) },
				clientId: 'default'
			};
			window.sessionStorage.setItem('rrweb', JSON.stringify(session));
			return session;
		},
		save(data) {
			const session = window.recorder.session.get();
			window.sessionStorage.setItem('rrweb', JSON.stringify(Object.assign({}, session, data)));
		},
		clear() {
			window.sessionStorage.removeItem('rrweb')
		}
	},
	setUser: function({ id, email, name }) {
		const session = window.recorder.session.get();
		session.user = { id, email, name };
		window.recorder.session.save(session)

		return window.recorder;
	},
	setMeta: function(meta = {}) {
		const session = window.recorder.session.get();
		session.meta = meta;
		window.recorder.session.save(session)

		return window.recorder;
	},
	setClientId(id) {
		const session = window.recorder.session.get();
		session.clientId = id;
		window.recorder.session.save(session)

		return window.recorder;
	},
	stop() {
		clearInterval(window.recorder.runner);
	},
	start() {
		window.recorder.runner = setInterval(function save() {
			const session = window.recorder.session.get();

			fetch('{{ .URL }}/record', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(Object.assign({}, { events: window.recorder.events }, session)),
			});
			window.recorder.events = []; // cleans-up events for next cycle
		}, 5 * 1000);
	},
	close() {
		clearInterval();
		window.recorder.session.clear();
	}
};

new Promise((resolve, reject) => {
	const script = document.createElement('script');
	script.src = 'https://cdn.jsdelivr.net/npm/rrweb@0.9.14/dist/rrweb.min.js';
	script.addEventListener('load', resolve);
	script.addEventListener('error', e => reject(e.error));
	document.head.appendChild(script);
}).then(() => {
	window.recorder.rrweb = rrweb;
	// TODO: This should be optimised 🤠
	rrweb.record({
		emit(event) {
			window.recorder.events.push(event);
		}
	});
	window.recorder.start();
}).catch(console.err);`

const playerHTML = `
<html>
	<head>
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, minimum-scale=1.0, maximum-scale=2.0, user-scalable=yes" />
		<title>Play | rrweb-explorer</title>
		<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.0.0-beta2/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-BmbxuPwQa2lc/FVzBcNJ7UAyJxM6wuqIj61tLrc4wSX0szH/Ev+nYRRuWlolflfl" crossorigin="anonymous" />
		<link href="https://cdn.jsdelivr.net/npm/rrweb-player@latest/dist/style.css" rel="stylesheet" />
	</head>
	<body>
		<div class="main container mt-3">
			<div class="row">
				<div class="col">
					<nav aria-label="breadcrumb">
						<ol class="breadcrumb">
							<li class="breadcrumb-item"><a href="/">Recordings</a></li>
							<li class="breadcrumb-item active" aria-current="page">Player</li>
						</ol>
					</nav>

					<h2 class="mb-3">Recording Re-play</h2>
					<div class="alert alert-warning" role="alert">
						Be aware this is just a proof of concept: the storage is not optimised, searching is not possible and it is not ready for production
					</div>
				</div>
			</div>
			<div class="row mb-3">
				<div class="col">
					<span class="badge bg-success">{{ .Record.Client.OS }}</span>
					<span class="badge bg-primary">{{ .Record.Client.Browser }} {{ .Record.Client.Version }}</span>
				</div>
			</div>
		</div>
		<div class="container mb-3" id="player">
		</div>
		<script type="application/javascript" src="https://cdn.jsdelivr.net/npm/rrweb-player@latest/dist/index.js" ></script>
		<script type="application/javascript">
			fetch('/api/v1/records/{{ .ID }}', {
				method: 'GET',
			})
			.then(res => res.json())
			.then((res) => {
				new rrwebPlayer({
					target: document.getElementById("player"), // customizable root element
					props: {
						width: document.getElementById("player").offsetWidth,
						events: res.events,
					},
				});
			}).catch(console.error);
		</script>
	</body>
</html>
`

const listHTML = `
<html>
	<head>
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, minimum-scale=1.0, maximum-scale=2.0, user-scalable=yes" />
		<title>Recordings | rrweb-explorer</title>
		<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.0.0-beta2/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-BmbxuPwQa2lc/FVzBcNJ7UAyJxM6wuqIj61tLrc4wSX0szH/Ev+nYRRuWlolflfl" crossorigin="anonymous">
	</head>
	<body>
		<div class="container mt-3">
			<nav aria-label="breadcrumb">
				<ol class="breadcrumb">
					<li class="breadcrumb-item active" aria-current="page">Recordings</li>
				</ol>
			</nav>

			<h2 class="mb-3">Recordings</h2>
			<div class="alert alert-warning" role="alert">
				Be aware this is just a proof of concept: the storage is not optimised, searching is not possible and it is not ready for production
			</div>

			<ul class="list-group mb-5">
			{{ range .Records }}
				<a href="/records/{{ .ID }}" class="list-group-item list-group-item-action">
					<div class="d-flex w-100 justify-content-between">
						<h5 class="mb-2 mt-1"><span class="badge bg-secondary">{{ .User.ID }}</span> {{ .User.Name }} </h5>
						<small class="text-muted">{{ .UpdatedAt.Format "Jan 02, 2006 15:04 UTC"  }}</small>
					</div>
					<p class="mb-1">
					{{ range $k, $v := .Meta }}
						<span class="badge bg-primary">{{ $k }} = {{ $v }}</span>
					{{ end }}
					</p>
				</a>
			{{ end }}
			</ul>

			<h2 class="mb-3">Start using</h2>
			<p>Insert the following snippet at the bottom of your <code>&lt;body&gt;</code> tag:</p>
			<pre>
&lt;script type=&quot;application/javascript&quot; src=&quot;{{ .URL }}/record.js&quot; &gt;&lt;/script&gt;
&lt;script type=&quot;application/javascript&quot;&gt;
window.recorder
  .setUser({id: 'USER_ID', email: 'test@test.com', name: 'Bruno Luiz Silva' })
  .setMeta({ foo: 'bar' })
  .setClientId('client-id')
&lt;/script&gt;
			</pre>
		</div>
	</body>
</html>
`

// Client keeps the session client information, mostly parsed from .UserAgent
type Client struct {
	UserAgent string `json:"userAgent"`
	OS        string `json:"os"`
	Browser   string `json:"browser"`
	Version   string `json:"version"`
}

// Record session record model, mostly with data from the events, user and browser used
type Record struct {
	ID string `json:"id" bow:"key"`
	// TODO: these events probably should live outside the database... probably something like S3
	Events []interface{}     `json:"events"`
	Meta   map[string]string `json:"meta"`
	User   struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	Client    Client    `json:"client"`
	ClientID  string    `json:"clientId"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func run(c *cli.Context) error {
	dbDSN, err := url.Parse(c.String("db-dsn"))
	if err != nil {
		panic(err)
	}

	tmplRecorder, err := template.New("recorder").Parse(rrwebRecord)
	if err != nil {
		return err
	}

	tmplPlayerHTML, err := template.New("player_html").Parse(playerHTML)
	if err != nil {
		return err
	}

	tmplListHTML, err := template.New("list_html").Parse(listHTML)
	if err != nil {
		return err
	}

	// Open badgerdb (please replace me)
	db, err := bow.Open(dbDSN.Path, bow.SetBadgerOptions(
		badger.DefaultOptions(dbDSN.Path).
			WithTableLoadingMode(options.FileIO).
			WithValueLogLoadingMode(options.FileIO).
			WithNumVersionsToKeep(1).
			WithNumLevelZeroTables(1).
			WithNumLevelZeroTablesStall(2),
	))
	if err != nil {
		return err
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: c.StringSlice("allowed-domains"),
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	// Entry point for recordings
	r.Post("/record", func(w http.ResponseWriter, r *http.Request) {
		var req Record
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var rec Record
		if err := db.Bucket("records").Get(req.ID, &rec); err != nil {
			if !errors.Is(err, bow.ErrNotFound) {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		ua := user_agent.New(r.UserAgent())

		rec.ID = req.ID
		rec.Events = append(rec.Events, req.Events...)
		rec.User = req.User
		rec.Meta = req.Meta
		rec.UpdatedAt = time.Now()

		browserName, browserVersion := ua.Browser()
		rec.Client = Client{
			UserAgent: r.UserAgent(),
			OS:        ua.OS(),
			Browser:   browserName,
			Version:   browserVersion,
		}

		if err := db.Bucket("records").Put(rec); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	// Renders a basic record script
	r.Get("/record.js", func(w http.ResponseWriter, r *http.Request) {
		err := tmplRecorder.Execute(w, struct {
			URL string
		}{URL: c.String("service-url")})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Renders record player
	r.Get("/records/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var rec Record
		if err := db.Bucket("records").Get(id, &rec); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = tmplPlayerHTML.Execute(w, struct {
			ID     string
			Record Record
		}{ID: id, Record: rec})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	// Renders records list
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var records []Record

		// Brings all results in memory 🤠
		// This is surely not ideal because it brings all .Events to memory as well... certaily it can go kaput
		var record Record
		iter := db.Bucket("records").Iter()
		defer iter.Close()
		for iter.Next(&record) {
			records = append(records, record)
		}

		// Sorting in memory 🤠
		sort.Slice(records, func(i, j int) bool {
			return records[i].UpdatedAt.After(records[j].UpdatedAt)
		})

		err = tmplListHTML.Execute(w, struct {
			Records []Record
			URL     string
		}{Records: records, URL: c.String("service-url")})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	r.Route("/api/v1/", func(r chi.Router) {
		r.Get("/records/{id}", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			var rec Record
			if err := db.Bucket("records").Get(id, &rec); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if err := json.NewEncoder(w).Encode(&rec); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
	})

	return http.ListenAndServe(c.String("address")+":"+c.String("port"), r)
}

func main() {
	app := &cli.App{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "service-url", Value: "http://localhost:3000", EnvVars: []string{"SERVICE_URL"}},
			&cli.StringFlag{Name: "address", Value: "127.0.0.1", EnvVars: []string{"ADDRESS"}},
			&cli.StringSliceFlag{Name: "allowed-domains", Value: cli.NewStringSlice("*"), EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "db-dsn", Value: "badger:///tmp/badgerdb", EnvVars: []string{"DB_DSN"}},
			&cli.StringFlag{Name: "port", Value: "3000", EnvVars: []string{"PORT"}},
		},
		Action: run,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
