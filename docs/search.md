# Search parameters

Search uses is enabled by SQL-like DSL. The following fields are available for querying:

- `updated_at`
- `client_id`
- `device`
- `os.name`
- `os.version`
- `browser.name`
- `browser.version`
- `meta.{{ use your own key }}`

Operations `=`, `>=`, `<=` and conditionals `AND` and `OR` can be used to filter your data. Below there are some query examples which you can try out:

- `os.name = 'Mac' AND os.version >= '10.5'`
- `os.version > '10.10' AND os.version <= '10.16'`
- `browser.name = 'Firefox' OR browser.name = 'Chrome'`
