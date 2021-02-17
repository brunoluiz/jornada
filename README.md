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

![](./docs/demo.gif)

> ☢️ WARNING: although this version is better than the [original prototype hack code][1], this project is still not recommended for developers 
> with accute hacky code alergy. It might still contain some hacky solutions, and some choices might make people scream (SQLite). ☢️

## Installation

### MacOS

Use `brew` to install it

```
brew install brunoluiz/tap/mermaid-server
```

### Linux and Windows

[Check the releases section](https://github.com/brunoluiz/jornada/releases) for more information details 

### Docker

The tool is available as a Docker image as well. Please refer to [Docker Hub page](https://hub.docker.com/r/brunoluiz/jornada/tags) to pick a release

## Usage in your frontend application

To use this, add the following snippet to your app (at the end of `<body>`) and then head to http://localhost:3000 to see recorded sessions

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

### Running locally

- Install `go` and `gcc` tooling
- Get SQLite `go get github.com/mattn/go-sqlite3`
- `go run ./cmd/jornada`
- By default, it will be served on `http://localhost:3000`

### Architecture

Refer to [`ARCHITECTURE.md`](./ARCHITECTURE.md) for more informations about the service implementation.

## To-do

- [x] Set-up rrweb JS recorder
- [x] Create basic templates for UI
- [x] Create easy to use/query storage for sessions
- [x] Create events storage
- [x] Create GDRP safe-mode
- [ ] Automatic release set-up w/ CGO
- [ ] Automatic payload sanitisation (rrweb doesn't sanitise passwords by default)
- [ ] Tweak SQLite
- [x] Support for metrics
- [ ] Test this with big traffic to understand how SQLite and BadgerDB will behave
- [ ] Create some test suite
- [ ] Support for other SQL engines
- [ ] Support filter and search (based on meta or client data)
- [ ] Support for player streaming/live mode (less memory consumption)
- [ ] Support for notes and session marking
- [ ] Support for bookmarking (could be through GA or something similar)
- [ ] Create OpenAPI schemas


[1]: https://gist.github.com/brunoluiz/96f111071f3a483ced13f57514707595
