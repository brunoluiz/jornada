# JS Client APIs

Once imported, the recorder will be available at `window.recorder`.

## Methods

### `window.recorder.setUser`

- Input: `(user: { id: string, name: string: email: string })`
- Output: window.recorder

Sets user information. If server is running on anonymised mode, this data will not be recorded

### `window.recorder.setMeta`

- Input: `(meta: { [key: string]: string })`
- Output: window.recorder

Sets session meta data. Useful to identify the session somehow.

### `window.recorder.setClientId`

- Input: `(clientId: string)`
- Output: window.recorder

Sets which clientId (example: site1 and site2). Can be used in the future as a filter in the explorer.

### `window.recorder.sync`

Forces the recorder to send data to the server

### `window.recorder.close`

Kills the recorder session
