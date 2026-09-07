# todoistik

A single-user GTD app. [design.md](design.md) says what it does and why;
[implementation.md](implementation.md) says what it is built out of;
[todo.md](todo.md) collects what is wanted after the MVP and has not been
designed yet.

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
| `-config`| `TODOISTIK_CONFIG`| `todoistik.conf` | settings file; missing is fine, wrong is fatal |

Open the address in a browser and enter the token once. Press `?` for the key map.

## Settings

Everything else is a screen or an item. The settings file holds only the
choices that are neither, one `key = value` per line, `#` for comments, read
once at startup — see implementation.md, "Settings file".

```sh
cat > todoistik.conf <<'EOF'
doing.show_nav = false     # keep the nav rail in doing mode (default false)
doing.show_keybar = true   # keep the key bar too (default true)
doing.show_timer = false   # minutes since the action went on screen (default false)
EOF
```

No file means the defaults. A file with an unknown key, a line without an `=`,
or a value that is not `true`/`false` refuses to start and says which line.

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

## Source layout

```
main.go                   entrypoint: flags, opens the DB, starts the server

internal/conf/
  conf.go                 the settings file: key = value, read once at startup
  conf_test.go

internal/cron/
  cron.go                 the day-granular cron dialect (design.md, "Schedule")
  cron_test.go

internal/app/             the domain — everything design.md describes, independent of HTTP
  types.go                the item structs (Action, Project, Schedule, ...) and their small derived methods (IsNext, ComputeStalled, ...)
  db.go                   SQLite open/schema/migrate, audit-log plumbing, the app_state key-value store
  capture.go              the inbox: Capture (with duplicate collapse), Inbox, edit/remove
  actions.go              action CRUD, tags, detach, SetNext/park, snooze
  projects.go             project CRUD, completion rules, Promote
  someday.go              someday/maybe items, and every Inbox Zero branch (ProcessTrash, ProcessAction, ...)
  schedules.go            schedule CRUD, firing, DayStart (the lazy day boundary: #today clearing + firing)
  review.go               weekly review counts and MarkReviewed
  views.go                the read-side queries: NextActions, Tasks, WaitingFor, Calendar, Archive, Projects, plus the shared filter/sort helpers
  nav.go                  per-view item counts, for the nav badges
  meta.go                 the remembered tag/context lists
  app_test.go             behavior tests for the load-bearing rules — not CRUD plumbing

internal/web/              HTTP and HTML — thin: talks to internal/app, never touches SQL directly
  server.go                routes, bearer-token/cookie auth, template funcs, the DayStart-on-every-request hook
  api.go                   the capture and read APIs (JSON)
  ui.go                    every UI page handler — one per view/action, one HTTP verb+path each
  static/                  style.css, app.js (the keyboard layer), vendored htmx.min.js
  templates/                one .html per page; _layout.html holds the shared nav, the ? help overlay, and reusable partials (actionrow, actionformfields, filterloud)
```

## Working on this codebase

For an agent (or a person) picking this up cold:

- **design.md is the spec, implementation.md is the build.** design.md says what the app does and why, with no mention of Go, SQLite or HTTP. implementation.md says what it's built out of. Code should never contradict either — a mismatch is a bug (fix the code), unless the docs themselves are wrong or silent on the point, in which case fix the docs first and say so.
- **Never silently reinterpret a design decision.** A change that would contradict something design.md or implementation.md already says is a design conversation, not a code change — flag it and get a decision before touching code. This repo's commit history is the record of exactly that conversation; read it for the pattern (and the *why*, which the docs' own prose carries — the docs are written to justify every rule, not just state it).
- **Keep the docs in sync with the code, in the same unit of work.** Any change to behavior updates design.md; any change to a technical or UI decision updates implementation.md. Not as an afterthought, and not batched into some later cleanup pass.
- **internal/app has no knowledge of HTTP.** It's called from internal/web today and could be called from a CLI or a test just as easily. Domain rules belong there, not in internal/web/ui.go.
- **internal/web is thin.** A handler parses the request, calls one or two internal/app methods, renders a template or redirects. If a handler needs to know a domain rule (is this action stalled? is this project a valid completion target?), that logic belongs in internal/app, not in the handler or the template.
- **Every SQL query lives in internal/app.** internal/web never imports database/sql.
- **The audit log always gets a snapshot.** Any create/edit/delete going through a.tx(...) should call a.audit(...) with enough of the item to recover it — see design.md, "Audit entry" and "Editing items".
- **Small, verifiable steps.** Build, vet and test before calling a change done (commands below). The tests in internal/app/app_test.go exist to pin down rules that are easy to get subtly wrong — duplicate collapse, schedule firing and back-firing, stalled derivation, delegation restamping, completion validations. Extend them when you touch that logic; don't just eyeball it.
- **Small commits, one topic each.** Prefer a docs-only commit separate from the code commit that implements it, matching this repo's existing history, over one commit that mixes design discussion with implementation.

## Development

```sh
go build ./...   # compile everything
go vet ./...     # static checks
go test ./...    # the domain test suite (internal/app, internal/cron)
gofmt -l .       # should print nothing; gofmt -w . to fix
```
