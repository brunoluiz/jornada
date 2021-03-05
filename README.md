<h1 align="center">
  Jornada
</h1>

<p align="center">
  The simplest option for recording and replaying user journeys 🎯
</p>

If you have a live website, users will be interacting with it. But recording these sessions for further analysis can be challenging.
**Jornada** makes this easy, enabling both session record and replay, allowing teams to have insights from user interactions. Some use cases:

- Debug reported issues
- User behaviour and experience analysis

> ☢️ WARNING: although this version is way better than the [original prototype hack code][1], this project is still under development and
> is not recommended for developers with severe hacky code alergy. ☢️

![](./docs/demo.gif)


## Installation

### MacOS

Use `brew` to install it

```
brew install brunoluiz/tap/jornada
```

### Linux and Windows

[Check the releases section](https://github.com/brunoluiz/jornada/releases) for more information details 

### Docker

The tool is available as a Docker image as well. Please refer to [Docker Hub page](https://hub.docker.com/r/brunoluiz/jornada/tags) to pick a release

## Usage

**⚠️ Be aware that, by default, the data will be always anonymised. Even if the client sends user data, the server will not save any PII (personal 
identifiable information). This includes user ids, names and e-mails. If your application is granted to gather certain personal data, 
run the application with `--non-anonymised-mode` flag.**

### Server

Use one of the distributions above to fetch a binary. Before running, bear in mind the following configurations:

```
   --public-url value       Public URL where the service is exposed. The service might be running on :3000, but the public access can be proxied through 80 (default: "http://localhost:3000") [$PUBLIC_URL]
   --non-anonymised-mode    If set, it will allow user details to be recorded (default: false) [$NON_ANONYMISED_MODE]
   --address value          Service address -- change to 127.0.0.1 if developing on Mac (avoids network warnings) (default: "0.0.0.0") [$ADDRESS]
   --port value             Service port (default: "3000") [$PORT]
   --allowed-origins value  CORS allowed origins (default: "*") [$ALLOWED_ORIGINS]
   --db-dsn value           DSN for SQL database (see github.com/mattn/go-sqlite3 for more options) (default: "sqlite:///tmp/jornada.db?cache=shared&mode=rwc&_journal_mode=WAL") [$DB_DSN]
   --events-dsn value       Events storage path (BadgerDB) (default: "badger:///tmp/jornada.events") [$EVENTS_DSN]
   --log-level value        Log level (default: "info") [$LOG_LEVEL]
```

### Client

Add the following snippet to your app (at the end of `<body>`) and then head to `http://localhost:3000` to see recorded sessions

```js
<script type="application/javascript" src="http://localhost:3000/record.js" ></script>
<script type="application/javascript">
  // Once record.js is imported, the setter functions can be called at any point in your application
  window.recorder
    .setUser({
      id: 'USER_ID',
      email: 'test@test.com',
      name: 'Bruno Luiz Silva'
    })
    .setMeta({ foo: 'bar' })
    .setClientId('client-id')
</script>
```

## Development

### Architecture & Documentation

- Refer to [`docs/js_client.md`](./docs/js_client.md) for more informations about the JS client.
- Refer to [`docs/architecture.md`](./docs/architecture.md) for more informations about the service implementation.
- Refer to [`docs/search.md`](./docs/search.md) for more informations about how to run searches.

### Running Jornada from source

If you want to contribute with Jornada, you might need to run from the source. The following steps are required:

- Install `go` and `gcc` tooling
- Get SQLite `go get github.com/mattn/go-sqlite3`
- `go run ./cmd/jornada`
- By default, it will be served on `http://localhost:3000`

## To-do

- [x] Set-up rrweb JS recorder
- [x] Create basic templates for UI
- [x] Create easy to use/query storage for sessions
- [x] Create events storage
- [x] Create GDRP safe-mode
- [x] Automatic release set-up w/ CGO
- [x] Support filter and search (based on meta or client data)
- [x] Support for metrics
- [x] Paginate results
- [ ] Support database automatic clean-ups, based on configurations
- [ ] Nice error pages
- [ ] SQL hooks for query logging and timing (or tracing of some sort)
- [ ] Tweak SQLite
- [ ] Test this with big traffic to understand how SQLite and BadgerDB will behave
- [ ] Create some test suite
- [ ] Support for other SQL engines
- [ ] Support for player streaming/live mode (less memory consumption)
- [ ] Support for notes and session marking
- [ ] Support for bookmarking (could be through GA or something similar)
- [ ] Create OpenAPI schemas


[1]: https://gist.github.com/brunoluiz/96f111071f3a483ced13f57514707595
