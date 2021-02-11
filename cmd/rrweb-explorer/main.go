package main

// ☢️ WARNING: If you are alergic a messy cowboy code, please don't read the code below ☢️
//
// # Intro:
//
// This snippet of magic is to test rrweb as a possible replacement to FullStory. It uses badger as storage because I
// didn't want to mingle with setting up a container and migrations for storage.
// > FullStory is a tool to record user sessions for further analysis (can be for debugging, UX etc)
//
// # Things which would require more work:
//
// - Easy search: FullStory has an easy search tool while here it would require some work around it
// - Events integration: FullStory makes some marks on when certain GA events happened... this is not supported by the player
//
// # Endpoints:
//
// - /record.js: used in the target application to send data to the server
// - /play.js?id=SESSION_ID: used to load the player anywhere -- shouldn't exist, it should be a REST
//   endpoint which would be called by /play, but it is due to how I was initially doing/hacking it
// - /: list all recorded sessions
// - /play?id=SESSION_ID: loads the player page
//
// # Usage:
// To use this, add the following snippet at your app and then head to http://localhost:3000 to see the recorded sessions
// <script type="application/javascript" src="http://localhost:3000/record.js" ></script>
//
// # To-do list after testing the hack:
//
// - [ ] See if this makes sense
// - [ ] Create proper APIs to retrieve records
// - [ ] Proper UI -- could be kept as server-side rendered
// - [ ] Make it searchable through generic parameters (example: client_id, appplication_id, user_id etc), without full scans
// - [ ] Reconsider storage (badger was a quick win)

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"text/template"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/dgraph-io/badger/v2/options"
	"github.com/go-chi/chi"
	"github.com/go-chi/cors"
	"github.com/zippoxer/bow"
)

func app(version, app string) string {
	return `
window.onload = function() {
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";

	function randomSequence(length) {
		let result = "";
		const charactersLength = characters.length;
		for (let i = 0; i < length; i++) {
			result += characters.charAt(Math.floor(Math.random() * charactersLength));
		}
		return result;
	};

	function getSessionId() {
		let id = window.sessionStorage.getItem('rrweb_id');
		if (id) return id;

		id = randomSequence(64)
		window.sessionStorage.setItem('rrweb_id', id);
		return id;
	}

	function injectScript(src) {
		return new Promise((resolve, reject) => {
			const script = document.createElement('script');
			script.src = src;
			script.addEventListener('load', resolve);
			script.addEventListener('error', e => reject(e.error));
			document.head.appendChild(script);
		});
	}

	injectScript('https://cdn.jsdelivr.net/npm/rrweb@` + version + `/dist/rrweb.min.js')
		.then(() => {` +
		app + `
		}).catch(console.err)
};`
}

const rrwebRecord = `
			let events = [];

			rrweb.record({
				emit(event) {
					events.push(event);
				},
			});

			setInterval(function save() {
				const body = JSON.stringify({
					id: getSessionId(),
					events,
				});
				events = [];
				fetch('{{ .URL }}/record', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
					},
					body,
				});
			}, 5 * 1000);
`

const rrwebPlayer = `
	injectScript('https://cdn.jsdelivr.net/npm/rrweb-player@latest/dist/index.js')
		.then(() => {
			const link = document.createElement("link");
			link.href = "https://cdn.jsdelivr.net/npm/rrweb-player@latest/dist/style.css";
			link.type = "text/css";
			link.rel = "stylesheet";
			document.getElementsByTagName("head")[0].appendChild(link);

			const events = {{ .Events }};

			new rrwebPlayer({
				target: document.body, // customizable root element
				props: {
					events,
				},
			})
		})`

const playerHTML = `
<html>
	<head>
		<meta charset="utf-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0, minimum-scale=1.0, maximum-scale=2.0, user-scalable=yes" />
		<title>Play | rrweb-explorer</title>
		<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.0.0-beta2/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-BmbxuPwQa2lc/FVzBcNJ7UAyJxM6wuqIj61tLrc4wSX0szH/Ev+nYRRuWlolflfl" crossorigin="anonymous">
	</head>
	<body>
		<div class="main container mt-3">
		</div>
		<script type="application/javascript" src="{{ .URL }}/play.js?id={{ .ID }}" ></script>
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
			<h2 class="mb-3">Recordings</h2>
			<ul class="list-group">
			{{ range .Records }}
				<a href="{{ $.URL }}/play?id={{ .ID }}" class="list-group-item list-group-item-action">
					<div class="d-flex w-100 justify-content-between">
						<h5 class="mb-1">Some Title</h5>
						<small class="text-muted">{{ .UpdatedAt.Format "Jan 02, 2006 15:04 UTC"  }}</small>
					</div>
					<p class="mb-1">
					<!-- add meta -->
					</p>
					<small class="text-muted"><!-- add tags --></small>
				</a>
			{{ end }}
			</ul>
		</div>
	</body>
</html>
`

type Record struct {
	ID        string              `json:"id" bow:"key"`
	Events    []interface{}       `json:"events"`
	Meta      []map[string]string `json:"meta"`
	Tags      []string            `json:"tags"`
	UpdatedAt time.Time
}

type RecordRequest struct {
	ID     string              `json:"id"`
	Events []interface{}       `json:"events"`
	Meta   []map[string]string `json:"meta"`
	Tags   []string            `json:"tags"`
}

const rrWebVersion = "0.9.14"
const url = "http://localhost:3000"

func main() {
	tmplPlayer, err := template.New("player").Parse(app(rrWebVersion, rrwebPlayer))
	if err != nil {
		log.Fatal(err)
		return
	}

	tmplRecorder, err := template.New("recorder").Parse(app(rrWebVersion, rrwebRecord))
	if err != nil {
		log.Fatal(err)
		return
	}

	tmplPlayerHTML, err := template.New("player_html").Parse(playerHTML)
	if err != nil {
		log.Fatal(err)
		return
	}

	tmplListHTML, err := template.New("list_html").Parse(listHTML)
	if err != nil {
		log.Fatal(err)
		return
	}

	// Open database under directory "test".
	db, err := bow.Open("badgerdb", bow.SetBadgerOptions(
		badger.DefaultOptions("badgerdb").
			WithTableLoadingMode(options.FileIO).
			WithValueLogLoadingMode(options.FileIO).
			WithNumVersionsToKeep(1).
			WithNumLevelZeroTables(1).
			WithNumLevelZeroTablesStall(2),
	))
	if err != nil {
		log.Fatal(err)
		return
	}
	defer db.Close()

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
	}))

	r.Post("/record", func(w http.ResponseWriter, r *http.Request) {
		var req Record
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var rec Record
		if err := db.Bucket("records").Get(req.ID, &rec); err != nil {
			if !errors.Is(err, bow.ErrNotFound) {
				log.Println(err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		rec.ID = req.ID
		rec.Events = append(rec.Events, req.Events...)
		rec.Meta = req.Meta
		rec.Tags = req.Tags
		rec.UpdatedAt = time.Now()

		if err := db.Bucket("records").Put(rec); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Write([]byte("ok"))
	})

	r.Get("/record.js", func(w http.ResponseWriter, r *http.Request) {
		err := tmplRecorder.Execute(w, struct {
			URL string
		}{
			URL: url,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	})

	r.Get("/play.js", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")

		var rec Record
		if err := db.Bucket("records").Get(id, &rec); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		parsedEvents, err := json.Marshal(&rec.Events)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = tmplPlayer.Execute(w, struct {
			URL    string
			Events string
		}{
			URL:    url,
			Events: string(parsedEvents),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	})

	r.Get("/play", func(w http.ResponseWriter, r *http.Request) {
		err = tmplPlayerHTML.Execute(w, struct {
			URL string
			ID  string
		}{
			URL: url,
			ID:  r.URL.Query().Get("id"),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		var records []Record

		var record Record
		iter := db.Bucket("records").Iter()
		defer iter.Close()
		for iter.Next(&record) {
			records = append(records, record)
		}

		err = tmplListHTML.Execute(w, struct {
			URL     string
			Records []Record
		}{
			URL:     url,
			Records: records,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	})

	r.Get("/api/records/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")

		var rec Record
		if err := db.Bucket("records").Get(id, &rec); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := json.NewEncoder(w).Encode(&rec.Events); err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	})

	http.ListenAndServe(":3000", r)
}
