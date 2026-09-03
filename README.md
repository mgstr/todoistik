# todoistik

A single-user GTD app. [design.md](design.md) says what it does and why;
[implementation.md](implementation.md) says what it is built out of.

## Run

```sh
go build -o todoistik .
TODOISTIK_TOKEN=$(openssl rand -hex 24) ./todoistik -addr 0.0.0.0:8390 -db ~/todoistik.db -tz Europe/Tallinn
```

Flags (each also readable from the environment):

| flag     | env               | default          |                                            |
|----------|-------------------|------------------|--------------------------------------------|
| `-addr`  | `TODOISTIK_ADDR`  | `127.0.0.1:8390` | listen address                             |
| `-db`    | `TODOISTIK_DB`    | `todoistik.db`   | SQLite database file                       |
| `-token` | `TODOISTIK_TOKEN` | *(empty)*        | bearer token; empty disables auth — then keep it bound to localhost |
| `-tz`    | `TODOISTIK_TZ`    | `Local`          | the one timezone that defines "today"      |

Open the address in a browser and enter the token once. Press `?` for the key map.

## APIs

One way in, one way out — both under the same bearer token:

```sh
# capture (the only way in): raw text or {"text": "..."}
curl -X POST -H "Authorization: Bearer $TOK" -d "Buy new winter tyres" http://host:8390/api/capture
# → {"status":"accepted", ...} or {"status":"duplicate"}

# read (the only way out): any view, with the caller's own filters
curl -H "Authorization: Bearer $TOK" "http://host:8390/api/view/next?tag=car&context=online"
```

Views: `inbox someday projects tasks next today waiting calendar archive scheduler review`.
Filter parameters (each view accepts the ones its screen offers): `name`,
`tag` (repeatable), `context` (repeatable), `duration` (repeatable), `focus`
(`exclude`/`only`), `due` (`today`/`tomorrow`/`thisweek`/`nextweek`),
`completed` (`today`/`yesterday`/`thisweek`/`lastweek`), `sort` (`age`/`title`), `desc`.

## Development

```sh
go test ./...
```
