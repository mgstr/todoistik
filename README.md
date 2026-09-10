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

## Backups

The app takes a snapshot of the database every hour, into a `<database>.backups`
directory beside it — `~/todoistik.db.backups/todoistik-2026-09-09T15.db`, one
file per hour, named for the hour in the configured timezone. It keeps
`backup.days` × 24 of them (2 days, 48 files, by default) and deletes the
oldest as new ones arrive. `backup.days = 0` turns them off.

A snapshot is a complete database written with `VACUUM INTO`, not a copy of
the file: in WAL mode the newest writes live in the `-wal` file beside the
database, so copying the database alone copies an older moment. Opening a
snapshot, or putting it back in place of the database, needs nothing else.

## Settings

Everything else is a screen or an item. The settings file holds only the
choices that are neither, one `key = value` per line, `#` for comments, read
once at startup — see implementation.md, "Settings file".

```sh
cat > todoistik.conf <<'EOF'
zen.views = doing, processing  # screens that open with every panel off (default: these two)
zen.show_timer = false         # start the doing screen with the timer showing (default false; ctrl-t flips it)
zen.timer_format = auto        # minutes, then H:MM past the hour. Or a pattern: H:MM, HH:MM, M
backup.days = 2                # days of hourly database snapshots to keep (default 2; 0 keeps none)
review.someday_days = 30       # days before a someday/maybe item is back on the weekly review (default 30)
EOF
```

No file means the defaults. A file with an unknown key, a line without an `=`,
a value that is not `true`/`false`, or a screen name the app does not have
refuses to start and says which line. `zen.views =` with nothing after it is a
legal answer and means no screen opens that way.

Which panels are on the rest of the time — the title bar, the navigation rail
and the key bar — is not in this file: it is screen state, set with `ctrl-v`
and remembered in the database like the filter sets.

## APIs

One way in, one way out — both under the same bearer token:

```sh
# capture (the only way in): raw text or {"text": "..."}; several lines are one
# item, of which the first is the one the inbox shows
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

`q` is the same set written as one line, the way it is typed on the screen —
`?q=@home %23car %23short milk` is `@home` and `#car` and `#short` and a title
containing "milk". Given `q`, the parameters above are not also read (`sort`
and `desc` are, since the line cannot say them).

## Reminders, both ways

`remindersync` carries items between a macOS Reminders list and todoistik. It
is a separate binary, and the direction is named on the command line, because
the two do opposite things to the same list:

```sh
go build -o remindersync ./cmd/remindersync
./remindersync move -list "Inbox" -token "$TOK"                        # a list into the inbox
./remindersync sync -list "geocaching" -q "#gc #sync" -token "$TOK"    # a view onto a list
./remindersync loop -token "$TOK" ~/remindersync.conf                  # both, every few minutes
```

`move` and `sync` take the same four flags, and `sync` two more:

| flag       | env               | default                 |                                                    |
|------------|-------------------|-------------------------|----------------------------------------------------|
| `-list`    |                   | *(required)*            | the Reminders list to work on                      |
| `-url`     | `TODOISTIK_URL`   | `http://127.0.0.1:8390` | the running app                                    |
| `-token`   | `TODOISTIK_TOKEN` | *(empty)*               | bearer token; empty for a server started without one |
| `-dry-run` |                   | *(off)*                 | print what would happen; change nothing on either side |
| `-view`    |                   | `next`                  | *(sync)* which view to mirror                      |
| `-q`       |                   | *(empty)*               | *(sync)* the view's filter line, as typed on the screen |

### move — a Reminders list into the inbox

One capture per reminder, and the reminder deleted once the app has said it has
the text. It goes in through the capture API like anything else.

Each reminder becomes one capture carrying everything it held — title and due
date on the first line, the note under it — because the reminder is deleted
straight after. Completed reminders are
left alone, nothing is written as a `#tag` or an `@context`, and a reminder the
app calls a duplicate is deleted too, since the identical line is already in
the inbox.

### sync — a view onto a Reminders list

The other direction, for reading away from the desk: whatever a view holds
appears on a Reminders list, which is on the phone and the watch without
anything having to be typed twice. `-q` is the filter line the view is filtered
with on screen, so the list can be one filter's worth of it —
`-q "#gc #sync"` is the list of what is tagged both.

**The list is the view.** Each reminder carries a marker, `(::15)`, which is
the action's id and how it is recognised — so renaming an action in todoistik
retitles its reminder instead of stranding it. Then:

- an item the list does not hold is added, with its description as the note and
  its due date as the reminder's own
- a reminder carrying no marker, or a marker no item in the view holds, is
  **deleted** — including one typed straight into Reminders. Point this at a
  list kept for it, not at one you also write to by hand
- a reminder already there is not rewritten into a copy of the item. Only what
  todoistik knows is written: a due date or note the item does not carry leaves
  the reminder's own alone, and a reminder due at 14:30 on the right day keeps
  its time, because an action's due date is a day and nothing finer

Running it again changes nothing — the second run prints `0 created, 0 updated,
0 deleted`. The list has to exist; a name that is not there is an error naming
the lists that are, rather than a new list nobody asked for.

**Ticking a reminder off does not complete the action** — it asks you to. A
ticked reminder files one capture and is then deleted:

```
Completion request from reminders ::15 ::2026-09-08T14:30 залогировать кеш 8 сентября
```

Processing that line asks one question instead of the usual six: complete
action 15, stamped with the time you ticked it, or ignore the request. Ignoring
leaves the action open, so the next run puts a fresh open reminder back on the
phone. If the action has since been deleted, promoted into a project or
completed at the desk, the screen says which and the request is thrown away.

That keeps one way into the app and one place where work is confirmed: the
phone says "this looks done", and you answer at the desk. See design.md,
"Completion requests" and "Inbox Zero".

### loop — every direction, every few minutes

```sh
./remindersync loop -token "$TOK" -period 5 ~/remindersync.conf
```

| flag       | env               | default                 |                                                    |
|------------|-------------------|-------------------------|----------------------------------------------------|
| `-period`  |                   | `5`                     | minutes between passes, counted from the end of one to the start of the next |
| `-url`     | `TODOISTIK_URL`   | `http://127.0.0.1:8390` | the default for every line that does not name its own |
| `-token`   | `TODOISTIK_TOKEN` | *(empty)*               | the same                                            |
| `-dry-run` |                   | *(off)*                 | print what every run would do; change nothing        |

The config file holds **one run per line**, and a line is the words you would
type at the prompt — the program's own name at the front is optional:

```sh
cat > ~/remindersync.conf <<'EOF'
# the phone's inbox, into todoistik
move -list Inbox

# geocaching, out to the watch
./remindersync sync -list geocaching -q "#gc #sync"

# a second list, from a different filter
sync -list "к покупке" -q "@grocery"
EOF
```

Blank lines and `#` comments are ignored, and quotes hold a value together, so
`-q "#gc #sync"` is written here exactly as it is typed at the prompt. `-url`
and `-token` given to `loop` stand in for every line; a line naming its own
wins. A file with an unknown direction, a line with nothing to work on, or a
quote that is never closed refuses to start and says which line — the same rule
the app's settings file keeps.

The runs go one at a time, in the order the file names them, and the wait
starts when the pass ends: Reminders answers one caller at a time, and two
passes at once would queue behind each other with no idea the other was there.
A run that fails is printed and the pass carries on; the next pass tries it
again. `ctrl-c` stops after the run it is in.

**It says what it will do once, and then only what it did.** After the opening
block, a pass where nothing moved prints nothing at all — so anything in the
log is something that happened:

```
2 run(s) every 5 minute(s), from /Users/andres/remindersync.conf:
  move -list Inbox
  sync -list geocaching -q #gc #sync

=== 2026-09-10 20:39:40  move -list Inbox
captured: проверка
1 captured, 0 duplicate, 0 left on the list
```

Failures go to stderr, one self-contained line each, so the two streams can be
redirected separately:

```
2026-09-10 20:42:31 sync -list geocaching -q #gc #sync: cannot reach http://127.0.0.1:8390
```

Nothing echoes a token, in either stream.

Both one-shot directions print a line per item and exit non-zero if they had to
leave anything behind; `loop` runs until it is stopped. See implementation.md,
"Reminders, both ways".

The first run asks macOS for permission to control Reminders; without it every
run fails with `-1743`, granted back under System Settings → Privacy & Security
→ Automation.

## Mail into the inbox

`mailsync` empties a Gmail label into the inbox, one capture per message. Put
the label on a mail wherever you read it; the run captures it and takes the
label off:

```sh
go build -o mailsync ./cmd/mailsync
MAILSYNC_PASSWORD=$(cat ~/.mailsync-app-password) \
  ./mailsync -label todoistik -user you@gmail.com -token "$TOK"
```

| flag             | env                 | default                 |                                                     |
|------------------|---------------------|-------------------------|-----------------------------------------------------|
| `-label`         |                     | *(required)*            | the Gmail label to empty                            |
| `-user`          | `MAILSYNC_USER`     | *(required)*            | the account, `you@gmail.com`                        |
| `-password-file` |                     | *(empty)*               | a file holding the app password; otherwise `MAILSYNC_PASSWORD` |
| `-server`        | `MAILSYNC_SERVER`   | `imap.gmail.com:993`    | the IMAP server                                     |
| `-account`       |                     | `0`                     | which signed-in Google account the links open in (`/mail/u/N/`) |
| `-url`           | `TODOISTIK_URL`     | `http://127.0.0.1:8390` | the running app                                     |
| `-token`         | `TODOISTIK_TOKEN`   | *(empty)*               | bearer token; empty for a server started without one |
| `-dry-run`       |                     | *(off)*                 | print what would be captured; change nothing on either side |

**The password is an app password, never your Google password**, and it is
never a flag — a flag is on the process list for anything on the machine to
read. Make one under Google Account → Security → 2-Step Verification → App
passwords, and give it to the run in `MAILSYNC_PASSWORD` or in a file named by
`-password-file`.

Each message becomes one capture: the **subject is the first line**, and who it
is from and a link straight back to the mail are the body under it — unshown in
the inbox list, and read on the processing screen, where they go into the
action's description. Nothing is written as a `#tag` or an `@context`.

**The label is the queue.** A mail the app has captured has its label removed,
by moving it to All Mail: the mail itself is not deleted, not marked read and
not moved out of the account. A message the app calls a duplicate loses its
label too, since the identical capture is already in the inbox. A run that
fails part way leaves the label on, and the next run captures it again, is told
it is a duplicate, and takes the label off then.

It prints a line per message and exits non-zero if it had to leave anything
behind. See implementation.md, "Mail into the inbox".

## Source layout

```
main.go                   entrypoint: flags, opens the DB, starts the server

cmd/remindersync/         both directions between a macOS Reminders list and the app, through its APIs
  main.go                 which direction, and the flags both of them take
  move.go                 a Reminders list into the inbox, through the capture API
  sync.go                 a view onto a Reminders list, through the read API
  loop.go                 every direction in a config file, on a period
  reminders.go            the osascript layer: one script per operation, one visit each
  main_test.go
  sync_test.go
  loop_test.go

cmd/mailsync/             a Gmail label into the inbox, through the capture API
  main.go                 the flags, and the run: capture everything, then take the labels off
  imap.go                 the IMAP layer: the label's messages, and the move that unlabels one
  capture.go              the capture a message makes — subject, sender, and the link back
  capture_test.go
  run_test.go             a whole run, against go-imap's in-memory server and a stub capture API

internal/apiclient/
  client.go               the app as seen from outside: capture on the way in, a view on the way out

internal/request/
  request.go              the completion-request line: the grammar the utility writes and the app reads
  request_test.go

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
  query.go                the filter line: `@home #car milk` read into a filter set and written back out
  nav.go                  per-view item counts, for the nav badges
  meta.go                 the remembered tag/context lists
  requests.go             completion requests: what one names, and confirming it
  app_test.go             behavior tests for the load-bearing rules — not CRUD plumbing

internal/web/              HTTP and HTML — thin: talks to internal/app, never touches SQL directly
  server.go                routes, bearer-token/cookie auth, template funcs, the DayStart-on-every-request hook
  api.go                   the capture and read APIs (JSON)
  ui.go                    every UI page handler — one per view/action, one HTTP verb+path each
  panels.go                which panels a screen wears, zen mode, and zen.views
  panels_test.go
  static/                  style.css, app.js (the keyboard layer), vendored htmx.min.js
  templates/                one .html per page (process_completion.html is the request's own screen); _layout.html holds the shared nav, the title bar, the panel chooser, the ? help overlay, and reusable partials (actionrow, actionformfields, filterloud)
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
