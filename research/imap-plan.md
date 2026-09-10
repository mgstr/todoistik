# Plan: replace go-imap with our own IMAP client

A working document for a later session. Not part of the repo's docs — delete it
when the work is done, or it becomes a fourth list nobody maintains.

## Why

`cmd/mailsync` is the only thing in the tree that links a third-party library
beyond the database driver, and what it links is a **beta**:
`github.com/emersion/go-imap/v2 v2.0.0-beta.8`, plus `go-message` and `go-sasl`
which `imapclient` drags along. A beta that breaks its API before 2.0.0 means
rewriting `cmd/mailsync/imap.go` anyway — so the argument is to own those lines
now rather than on someone else's schedule.

Nothing that runs all day is affected either way: `go list -deps .` and
`go list -deps ./cmd/remindersync` both find zero emersion packages today. This
is about `mailsync` alone.

## Decision taken

**Hand-roll the client; keep go-imap as a test-only dependency.**

The shipped `mailsync` binary then links no third-party IMAP code at all, while
`run_test.go` keeps driving a whole run against go-imap's in-memory server —
which is what pins the assertion that matters most (after a run the label is
empty *and All Mail still holds the mail*). Writing a fake server as well as a
client doubles the new code and tests the fake rather than the truth.

`go.mod` will still name go-imap. What changes is that
`go list -deps ./cmd/mailsync | grep emersion` returns nothing, and that is the
line to put in the acceptance check.

*Alternative, if a test-only dependency is unacceptable:* drop go-imap entirely
and write a scripted fake server for the tests (a `net.Listen` that replays
canned responses). Adds ~150 lines of test scaffolding and tests our own idea of
what a server says. Not recommended.

## What exists now

```
cmd/mailsync/
  main.go          flags, run(): capture everything, then unlabel in one pass
  imap.go          ← the only file that mentions go-imap
  capture.go       subject/sender/link — pure, no protocol, DO NOT TOUCH
  capture_test.go  pure unit tests — should keep passing untouched
  run_test.go      whole-run test against go-imap's imapserver
```

`imap.go` is the entire surface to replace. Its exported-to-the-package shape:

```go
var dialTLS = imapclient.DialTLS        // the test seam

type mailbox struct{ c *imapclient.Client; allMail string; holds uint32 }

func dial(server, user, password, label string) (*mailbox, error)
func (m *mailbox) close()
func (m *mailbox) messages() ([]message, error)
func (m *mailbox) unlabel(uids []imap.UID) error
```

`message` lives in `capture.go` and currently embeds library types:

```go
type message struct {
    UID       imap.UID
    Subject   string
    From      []imap.Address
    MessageID string
}
```

**Keep this shape.** If `dial`/`messages`/`unlabel` keep their signatures,
`main.go` does not change at all, and `capture.go` changes only in what types
`message` is made of.

## The one insight that makes this small

Do **not** parse `ENVELOPE`. It is a nested parenthesised structure with NILs,
quoted strings and literals inside it, and reading it means writing an
S-expression parser. Ask for raw headers instead:

```
a4 UID FETCH 1:* (UID BODY.PEEK[HEADER.FIELDS (SUBJECT FROM MESSAGE-ID)])
```

The response carries one literal of plain header text per message, and stdlib
does the rest:

- `net/textproto.Reader.ReadMIMEHeader` → the three fields
- `net/mail.ParseAddress` → `Marju Tamm <marju@example.com>` into name + address
- `mime.WordDecoder` → `=?UTF-8?B?...?=` (already used in `capture.go`)

`BODY.PEEK` rather than `BODY` is load-bearing: `BODY` sets `\Seen` and would
mark your mail read as a side effect of capturing it.

This lets `message.From` become `*mail.Address` (stdlib) instead of
`[]imap.Address`, which also simplifies `senderLine` — drop the group-marker
case, since `ParseAddress` never produces one.

## Protocol surface actually needed

Five commands. Tag every command (`a1`, `a2`, …) and read until the line
starting with that tag.

```
S: * OK Gimap ready ...
C: a1 LOGIN "you@gmail.com" "sixteencharpwd"
S: a1 OK ...                          (or "a1 NO [AUTHENTICATIONFAILED] ...")

C: a2 LIST "" "*" RETURN (SPECIAL-USE)
S: * LIST (\HasNoChildren) "/" "todoistik"
S: * LIST (\HasNoChildren \All) "/" "[Gmail]/All Mail"
S: a2 OK ...

C: a3 SELECT "todoistik"
S: * 5 EXISTS                          ← this is mailbox.holds
S: * OK [UIDVALIDITY 1] ...
S: a3 OK [READ-WRITE] ...

C: a4 UID FETCH 1:* (UID BODY.PEEK[HEADER.FIELDS (SUBJECT FROM MESSAGE-ID)])
S: * 1 FETCH (UID 12 BODY[HEADER.FIELDS (SUBJECT FROM MESSAGE-ID)] {118}
S: <118 bytes of raw header text>
S: )
S: a4 OK ...

C: a5 UID MOVE 12,15 "[Gmail]/All Mail"
S: * OK [COPYUID ...]
S: * 1 EXPUNGE
S: a5 OK ...

C: a6 LOGOUT
```

Notes that will otherwise cost an afternoon each:

- **Literals can appear anywhere a string can.** `{118}` followed by CRLF and
  exactly 118 bytes, which may themselves contain CRLF. The reader must handle
  a literal wherever it reads a string, not only where you expect one.
- **Untagged responses are interleaved freely.** `* 5 EXISTS`, `* FLAGS (...)`,
  `* OK [UIDVALIDITY ...]`, `* BYE`. Skip anything not recognised rather than
  erroring — a server may volunteer status at any point.
- **`RETURN (SPECIAL-USE)` needs LIST-EXTENDED.** Gmail has it. If the tagged
  response comes back `BAD`, retry plain `LIST "" "*"` — Gmail returns the
  `\All` attribute either way. Never match All Mail by name: it is localised
  (`[Gmail]/Kogu meil`), and matching by name is the bug the current test is
  built to catch.
- **`UID FETCH 1:*`** is UIDs one through highest, i.e. everything. Guard on
  `holds == 0` first, as now — an empty mailbox and `1:*` is a needless error.
- **`UID MOVE`, never COPY + STORE \Deleted + EXPUNGE.** Deleting asks Gmail's
  own expunge setting what deleting means, and the wrong setting turns
  "unlabel" into "throw away". Gmail advertises MOVE.
- **Quote what you send**, escaping `\` and `"`. App passwords are `[a-z]{16}`
  so quoting always suffices; sending literals from the client (and handling
  the `+ ` continuation) is not needed.
- **Deadlines.** Set a read/write deadline per command; a hung socket in a cron
  job is a job that never ends.

## Modified UTF-7 (RFC 3501 §5.1.3)

Mailbox names on the wire are ASCII. A label called `Arved` is fine; `Töö` or
`Дела` is not. Both directions are needed — encode for `SELECT`/`MOVE`, decode
for `LIST` output (the error message names the labels that exist).

- printable ASCII `0x20`–`0x7e` passes through, except `&`, which becomes `&-`
- anything else: modified BASE64 of the UTF-16BE bytes, between `&` and `-`,
  with `,` in place of `/` in the alphabet, and **no `=` padding**

`Töö` → `T&APYA9g-`. Round-trip tests on Estonian and Russian names are cheap
and catch every mistake this makes.

## Proposed layout

```
cmd/mailsync/
  imap.go          dial/messages/unlabel — same signatures, our own client under them
  imapconn.go      the connection: tagged commands, untagged responses, literals
  imapconn_test.go parser tests against recorded byte streams
  utf7.go          modified UTF-7, both directions
  utf7_test.go     round trips, Estonian and Russian
```

Keep it in `package main`. A `internal/imap` package would invite reuse there
is none of — one caller, one server, five commands.

## Order of work

Each step ends buildable and testable.

1. **`utf7.go` + tests.** Pure, self-contained, no protocol. Do it first because
   it is the only piece with a spec you can check against by hand.
2. **`imapconn.go` + tests.** A `conn` wrapping `net.Conn` with:
   `cmd(format, args...) (untagged [][]byte, err error)` — write a tagged
   command, collect untagged lines until the tagged reply, splice literals in.
   Test it against recorded byte streams: a literal spanning CRLF, an
   interleaved `* 5 EXISTS`, a `NO` reply, a quoted string containing `)`.
3. **`imap.go` rewritten** on top of it, same four signatures. `message.From`
   becomes `*mail.Address`; adjust `senderLine` in `capture.go` and its tests.
4. **Point the seam at the new dialler.** `var dialConn = func(addr string)
   (net.Conn, error) { return tls.Dial("tcp", addr, nil) }`, and `run_test.go`
   replaces it with `net.Dial`. The in-memory server in the test already runs
   with `InsecureAuth: true`, so nothing there changes but that one line.
5. **`go mod tidy`.** go-imap should end up in the indirect/test position;
   `go-message` and `go-sasl` should disappear entirely.

## Tests

Everything currently green must stay green — in particular `run_test.go`
unchanged apart from the seam. Add:

- utf7 round trips, both directions, ASCII and non-ASCII
- a literal that contains CRLF inside it
- an untagged `* 5 EXISTS` arriving in the middle of a FETCH response
- a tagged `NO` producing an error naming what the server said
- a `LIST` reply whose mailbox name is a literal rather than a quoted string

The acceptance check that matters is unchanged and must still pass:

```
TestARunCapturesTheLabelAndTakesItOff
  → the label is empty AND All Mail still holds the mail
```

## Docs (repo rule — same unit of work, separate commit, first)

Read `CLAUDE.md` and README's "Working on this codebase" before starting.

- **implementation.md, "Mail into the inbox"** — the bullets about go-imap and
  about the test seam are the ones that move. New decisions to record with
  their reasons: headers rather than `ENVELOPE`; `BODY.PEEK` rather than
  `BODY`; literals handled everywhere; UTF-7 in both directions; go-imap
  surviving as the test server only.
- **Also fix, while there:** that section currently claims go-imap is "the one
  direct dependency in the tree". It is not, and was not quite when written —
  the SQLite driver is direct too. The honest phrasing is "the one direct
  dependency beyond the database driver", and after this work it is gone from
  the shipped binary entirely.
- **README** — source layout gains `imapconn.go` and `utf7.go`. The usage
  section does not change: no flag, no behaviour and no output moves.
- **design.md — unchanged, and say so explicitly in the commit.** Nothing about
  what the app does moves; this is entirely how `mailsync` is built.

## Commits

Branch off `develop` first (the repo merges feature branches with `--no-ff`;
see `capture-body` and `mailsync`). Then:

1. `Implementation: mailsync speaks IMAP itself` — docs only
2. `Modified UTF-7, both directions` — utf7.go + tests
3. `A tagged command, its untagged answers, and its literals` — imapconn.go + tests
4. `mailsync drops go-imap from everything it ships` — imap.go, capture.go, go.mod

Message style: a declarative sentence, then prose saying *why*, wrapped ~76
columns. Trailers as the existing history has them.

## Acceptance

```sh
go build ./... && go vet ./... && gofmt -l . && go test ./...
go list -deps ./cmd/mailsync | grep emersion    # must print nothing
go list -m -f '{{if not .Indirect}}{{.Path}}{{end}}' all   # driver + todoistik only
```

Then, by hand against the real account, which no test here can reach:

```sh
./mailsync -label todoistik -user you@gmail.com -dry-run     # prints, touches nothing
./mailsync -label todoistik -user you@gmail.com              # one throwaway mail first
```

and check in Gmail afterwards that the mail is **still there**, unread as it
was, simply without the label.

## Risks

- **Literals in unexpected places** is the failure mode most likely to ship
  undetected, because the in-memory test server may quote where Gmail sends a
  literal. Write at least one test that forces a literal for a mailbox name.
- **The test server is not Gmail.** It answers a stricter, simpler dialect.
  Green tests are necessary and not sufficient; the by-hand run above is the
  real acceptance.
- **Nothing must ever delete mail.** If in doubt at any point, fail the run and
  leave the label on. A mail left labelled is captured again next run and
  collapses as a duplicate; a mail deleted is gone.

## Out of scope

- The app-password whitespace fix (`abcd efgh ijkl mnop` pasted from Google
  fails to log in, because `readPassword` trims only surrounding space). Worth
  doing, unrelated to this, one narrow rule: strip spaces only when the value is
  exactly four groups of four letters.
- `/mailsync` is missing from `.gitignore` beside `/todoistik` and
  `/remindersync`. One line, unrelated.
