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
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"text/template"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	_ "github.com/joho/godotenv/autoload"
	"github.com/mssola/user_agent"
	"github.com/oklog/ulid"
	"github.com/urfave/cli/v2"
)

const rrwebRecord = `
window.recorder = {
	events: [],
	rrweb: undefined,
	runner: undefined,
	session: {
		synced: false,
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
			const session = window.sessionStorage.getItem('rrweb');
			return session ? JSON.parse(session) : {
				user: { id: window.recorder.session.genId(64) },
				clientId: 'default'
			};
		},
		save(data) {
			const session = window.recorder.session.get();
			window.sessionStorage.setItem('rrweb', JSON.stringify(Object.assign({}, session, data)));
			window.recorder.session.synced = false;

			return window.recorder.session
		},
		clear() {
			window.sessionStorage.removeItem('rrweb')
		},
		sync() {
			if (window.recorder.session.synced) return;

			return fetch('{{ .URL }}/api/v1/records', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(window.recorder.session.get()),
			}).then(res => {
				window.recorder.session.synced = true;
				return res.json();
			})
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
	sync() {
		if (!window.recorder.events.length) return;

		const session = window.recorder.session.get();
		fetch('{{ .URL }}/api/v1/records/' + session.id + '/events', {
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(window.recorder.events),
		});
		window.recorder.events = []; // cleans-up events for next cycle
	},
	start() {
		window.recorder.runner = setInterval(function save() {
			window.recorder.session.sync();
			window.recorder.sync();
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
		},
		slimDOMOptions: {
			script: false,
			comment: false,
			headFavicon: false,
			headWhitespace: false,
			headMetaDescKeywords: false,
			headMetaSocial: false,
			headMetaRobots: false,
			headMetaHttpEquiv: false,
			headMetaAuthorship: false,
			headMetaVerification: false,
		},
		inlineStylesheet: false,
		sampling: {
			mousemove: true,
			mouseInteraction: false,
			scroll: 150,
			input: 'last',
		},
	});

	return window.recorder.session.sync();
}).then(res => {
	window.recorder.session.save({ id: res.id });
	window.recorder.start();
})
.catch(console.err);`

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
		<script type="application/javascript" src="https://cdn.jsdelivr.net/npm/rrweb@0.9.14/dist/rrweb.min.js" ></script>
		<script type="application/javascript">
			fetch('/api/v1/records/{{ .ID }}/events', {
				method: 'GET',
			})
			.then(res => res.json())
			.then((res) => {
				new rrwebPlayer({
					target: document.getElementById("player"), // customizable root element
					props: {
						width: document.getElementById("player").offsetWidth,
						events: res,
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

var RecordCreate struct {
	ClientID string            `json:"clientId"`
	Meta     map[string]string `json:"meta"`
	User     struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
	Client Client `json:"client"`
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
	db, err := NewBadgerStore(dbDSN.Path, 0)
	if err != nil {
		return err
	}
	defer db.Close()

	events := eventsStore{db.BadgerDB}
	recordings := recordingStore{db.BadgerDB}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: c.StringSlice("allowed-domains"),
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

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

		rec, err := recordings.GetByID(r.Context(), id)
		if err != nil {
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
		records, err := recordings.GetAll(r.Context(), "", 10)

		// // Sorting in memory 🤠
		// sort.Slice(records, func(i, j int) bool {
		//   return records[i].UpdatedAt.After(records[j].UpdatedAt)
		// })

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

			rec, err := recordings.GetByID(r.Context(), id)
			if err != nil {
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

		r.Get("/records/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			w.Write([]byte("["))
			events.Get(r.Context(), id, func(b []byte, last bool) error {
				if !last {
					b = append(b, ',', '\n')
				}
				_, err := w.Write(b)
				return err
			})
			w.Write([]byte("]"))
		})

		// Entry point for recordings
		r.Post("/records", func(w http.ResponseWriter, r *http.Request) {
			var req Record
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			id := req.ID
			if id == "" {
				t := time.Now()
				id = ulid.MustNew(ulid.Timestamp(t), ulid.Monotonic(rand.New(rand.NewSource(t.UnixNano())), 0)).String()
			}

			ua := user_agent.New(r.UserAgent())
			browserName, browserVersion := ua.Browser()

			rec := Record{
				ID:   id,
				User: req.User,
				Meta: req.Meta,
				Client: Client{
					UserAgent: r.UserAgent(),
					OS:        ua.OS(),
					Browser:   browserName,
					Version:   browserVersion,
				},
			}

			if err := recordings.Save(r.Context(), rec); err != nil {
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

		r.Put("/records/{id}/events", func(w http.ResponseWriter, r *http.Request) {
			id := chi.URLParam(r, "id")

			req := []interface{}{}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			jsons := [][]byte{}
			for _, v := range req {
				event, err := json.Marshal(v)
				if err != nil {
					log.Println(err)
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				jsons = append(jsons, event)
			}

			if err := events.Add(r.Context(), id, jsons...); err != nil {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
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
