# Architecture

## Recording

Jornada defines a simple JS client-side API to be used by developers, using [`rrweb`](https://www.rrweb.io/) to record events.
This API create a session within the service and keep it up-to-date when changes occur (example: user details after log-in).

The following happens once the recorder is instantiated:

- Call `POST /api/v1/sessions` to create a session (receives `session id`)
- Call `POST /api/v1/sessions` for sub-sequent sessions updates, but attaching the `session id` on the body
- Every 5 seconds, dump recorded events to the service through `POST /api/v1/sessions/{id}/events`

[![](https://mermaid.ink/img/eyJjb2RlIjoic2VxdWVuY2VEaWFncmFtXG4gICAgYXBwLT4-YXBpOiBjcmVhdGUgc2Vzc2lvblxuICAgIGFwaS0tPj5hcHA6IG9rIHcvIHNlc3Npb25faWRcblxuICAgIGxvb3BcbiAgICAgICAgYXBwLT4-YXBpOiBzZW5kcyBldmVudHNcbiAgICAgICAgYXBwLT4-YXBpOiBzZW5kcyBzZXNzaW9uIHVwZGF0ZXNcbiAgICBlbmRcbiIsIm1lcm1haWQiOnsidGhlbWUiOiJuZXV0cmFsIn0sInVwZGF0ZUVkaXRvciI6ZmFsc2V9)](https://mermaid-js.github.io/mermaid-live-editor/#/edit/eyJjb2RlIjoic2VxdWVuY2VEaWFncmFtXG4gICAgYXBwLT4-YXBpOiBjcmVhdGUgc2Vzc2lvblxuICAgIGFwaS0tPj5hcHA6IG9rIHcvIHNlc3Npb25faWRcblxuICAgIGxvb3BcbiAgICAgICAgYXBwLT4-YXBpOiBzZW5kcyBldmVudHNcbiAgICAgICAgYXBwLT4-YXBpOiBzZW5kcyBzZXNzaW9uIHVwZGF0ZXNcbiAgICBlbmRcbiIsIm1lcm1haWQiOnsidGhlbWUiOiJuZXV0cmFsIn0sInVwZGF0ZUVkaXRvciI6ZmFsc2V9)

The available JS API can be seen in [./internal/http/view/view.go](here). In the future, this will be extracted as a npm package.

## Storage

Two storages are used in Jornada:

1. [./internal/repo/sessions_sql.go]( SQLite ) : used to save session and user details. The way the recorder is set-up, there will be just a few writes per session in this
storage. SQLite seems to be the simplest operational choice, due to the low throughput it will probably have. Adding support to other 
SQL engines shouldn't be hard though.
2. [./internal/repo/events_badger.go](BadgerDB): a Golang LSM key-value storage. It is used to save the event stream from `rrweb`.

## Reference

### Project structure

```
/cmd/*: project binaries

/docs
  /diagrams: mermaid diagrams

/internal/
  /server: http routes and server setup
    /view: explorer UI views

  /repo: repositories packages

  /storage: packages to initialise and manage project's storage
    /badgerdb: badgerdb v2 storage package
    /sqldb: sql storage package (for now only SQLite)
```

### Endpoints

- `GET  /`: redirects to /sessions
- `GET  /sessions`: loads recorded sessions
- `GET  /sessions/{id}`: load session details and player
- `POST /api/v1/sessions`: start a new session, returning an ID to be used by the recorder
- `GET  /api/v1/sessions/{id}`: retrieve session by ID (api used by the player JS)
- `POST /api/v1/sessions/{id}/events`: record session events (rrweb)
- `GET  /record.js`: used in the target application to send data to the server
