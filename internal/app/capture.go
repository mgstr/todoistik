package app

import (
	"database/sql"
	"errors"
	"strings"
)

var ErrEmpty = errors.New("empty text")

// The ways in, each named in the code that captures under it rather than in
// anybody's configuration: every one of these programs is in this repository,
// so the name is a fact about the code and cannot drift into two spellings of
// the same channel — which is the one thing that would make a year of counts
// lie. Each name is written in exactly one place, which is why the four
// programs that talk to the app over HTTP carry their own rather than
// importing these: `mail` (cmd/mailsync), `telegram` (cmd/telegrambot),
// `reminders` (cmd/remindersync, the list move and the completion channel
// alike) and `mcp` (cmd/todoistikmcp). Those four never import the domain, by
// design — they talk to the running app and not to its database.
//
// The four below are the ones the app itself writes. Anything else is a caller
// from outside naming itself, which is answered in SourceAPI's comment.
const (
	SourceApp      = "app"      // the capture dialog, typed by hand
	SourceAPI      = "api"      // POST /api/capture, when the caller named no source of its own
	SourceSchedule = "schedule" // a schedule firing an occurrence
	SourceSomeday  = "someday"  // an idea sent back to the inbox to be decided about again
)

// Capture puts raw text into the inbox. Every way in goes through here:
// typed by hand, the capture API, a schedule firing. Returns accepted=false
// when the text exactly matches an item already sitting in the open inbox —
// the duplicate is dropped, unaudited, per design.md "Duplicate captures".
//
// source says which way in that was. It is recorded as given and never
// refused: capture may fail for an empty text and for nothing else, so a name
// this app has never seen is written down rather than argued with.
func (a *App) Capture(text, source string) (item *InboxItem, accepted bool, err error) {
	text = normalizeCapture(text)
	if text == "" {
		return nil, false, ErrEmpty
	}
	err = a.tx(func(tx *sql.Tx) error {
		var accepted2 bool
		item, accepted2, err = a.captureTx(tx, text, source)
		accepted = accepted2
		return err
	})
	return item, accepted, err
}

func (a *App) captureTx(tx *sql.Tx, text, source string) (*InboxItem, bool, error) {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM inbox_items WHERE text = ?`, text).Scan(&n); err != nil {
		return nil, false, err
	}
	if n > 0 {
		return nil, false, nil
	}
	item := &InboxItem{Text: text, Source: NormalizeSource(source), CreatedAt: a.now().UTC()}
	res, err := tx.Exec(`INSERT INTO inbox_items (text, source, created_at) VALUES (?,?,?)`, item.Text, item.Source, ts(item.CreatedAt))
	if err != nil {
		return nil, false, err
	}
	item.ID, _ = res.LastInsertId()
	if err := a.audit(tx, EvCreated, "inbox", item.ID, item); err != nil {
		return nil, false, err
	}
	return item, true, nil
}

// normalizeCapture makes one string out of the several ways the same words can
// arrive: a browser posts a textarea with CRLF line endings, a script posts LF,
// and either may come with space around it. Duplicate collapse compares the
// whole text exactly (design.md, "Duplicate captures"), so the same capture
// typed in the dialog and posted by curl has to be the same string or the
// second one would not collapse into the first.
func normalizeCapture(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimSpace(text)
}

// NormalizeSource folds a source name to the one spelling it is counted under.
// `Telegram`, `telegram ` and `TELEGRAM` are one channel, and a name that only
// differs by case would otherwise show up as two rows in the aggregation the
// field exists for.
//
// It shortens rather than refuses, for the reason Capture never refuses one:
// a script that sends a sentence where a name belongs still gets its capture
// in, and the mistake is visible as a name nobody recognises rather than as a
// thought that never arrived. Anything that is not a letter, a digit, a dash
// or an underscore is dropped, so a name cannot arrive carrying notation or a
// line break and be read as something else later.
func NormalizeSource(source string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(source)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		}
		if b.Len() >= sourceMax {
			break
		}
	}
	return b.String()
}

// sourceMax is where a name stops being a name. Every one this app writes is
// under ten characters; the limit is there for the caller that sends something
// else entirely, so that what lands in the column is still readable as a row
// in a list of channels.
const sourceMax = 32

// SplitCapture divides a capture into the line the inbox shows and the body
// carried under it, unshown, until the item is processed (design.md, "Inbox
// item"). The ordinary capture is one line and has no body at all.
//
// Everything that reads a capture reads the line and never the body: the
// notation the branches are seeded from, and the completion request grammar.
// A body comes from wherever the capture came from and was not written to be
// read — a link to a mail is full of `#`, the address it came from is an `@` —
// so keeping it away from the readers is a rule here rather than a bet on the
// remembered lists not holding those names.
func SplitCapture(text string) (line, body string) {
	line, body, _ = strings.Cut(text, "\n")
	return strings.TrimSpace(line), strings.Trim(body, "\n")
}

// Inbox returns the open inbox, oldest first. No filters, by design.
func (a *App) Inbox() ([]*InboxItem, error) {
	rows, err := a.db.Query(`SELECT id, text, source, created_at FROM inbox_items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*InboxItem
	for rows.Next() {
		it := &InboxItem{}
		var at string
		if err := rows.Scan(&it.ID, &it.Text, &it.Source, &at); err != nil {
			return nil, err
		}
		it.CreatedAt = parseTS(at)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (a *App) inboxItemTx(tx *sql.Tx, id int64) (*InboxItem, error) {
	it := &InboxItem{}
	var at string
	err := tx.QueryRow(`SELECT id, text, source, created_at FROM inbox_items WHERE id = ?`, id).Scan(&it.ID, &it.Text, &it.Source, &at)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = parseTS(at)
	return it, nil
}

// InboxItem returns one open inbox item.
func (a *App) InboxItem(id int64) (*InboxItem, error) {
	it := &InboxItem{}
	var at string
	err := a.db.QueryRow(`SELECT id, text, source, created_at FROM inbox_items WHERE id = ?`, id).Scan(&it.ID, &it.Text, &it.Source, &at)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = parseTS(at)
	return it, nil
}

// EditInboxText edits a captured text in place (audited like any edit).
func (a *App) EditInboxText(id int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrEmpty
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.inboxItemTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE inbox_items SET text = ? WHERE id = ?`, text, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "inbox", id, before)
	})
}

// removeInboxItem deletes an inbox item as part of processing it. The audit
// event says what became of it; the snapshot is the item as captured.
func (a *App) removeInboxItem(tx *sql.Tx, id int64, event string) (*InboxItem, error) {
	it, err := a.inboxItemTx(tx, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM inbox_items WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return it, a.audit(tx, event, "inbox", id, it)
}
