package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type App struct {
	db   *sql.DB
	path string         // the database file, for the snapshots kept beside it
	loc  *time.Location // the one configured timezone that defines "today"
	now  func() time.Time
	// somedayReviewDays: the review period for someday/maybe items, in days
	// (see internal/app/review.go). Open seeds it with the settings file's
	// default so a caller that reads no settings file still gets the rule;
	// main overwrites it with whatever the file says.
	somedayReviewDays int
}

func Open(path string, loc *time.Location) (*App, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// modernc/sqlite serializes writes poorly across many conns; one is plenty
	// for a single-user app and removes SQLITE_BUSY from the picture.
	db.SetMaxOpenConns(1)
	a := &App{db: db, path: path, loc: loc, now: time.Now, somedayReviewDays: 30}
	if err := a.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

func (a *App) Close() error { return a.db.Close() }

const schema = `
CREATE TABLE IF NOT EXISTS inbox_items (
	id INTEGER PRIMARY KEY,
	text TEXT NOT NULL,
	source TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS someday_items (
	id INTEGER PRIMARY KEY,
	text TEXT NOT NULL,
	created_at TEXT NOT NULL,
	last_reviewed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS projects (
	id INTEGER PRIMARY KEY,
	title TEXT NOT NULL,
	dod TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	last_reviewed_at TEXT NOT NULL,
	snooze_until TEXT NOT NULL DEFAULT '',
	next_action_id INTEGER REFERENCES actions(id) ON DELETE SET NULL,
	completed_at TEXT
);
CREATE TABLE IF NOT EXISTS actions (
	id INTEGER PRIMARY KEY,
	project_id INTEGER REFERENCES projects(id),
	title TEXT NOT NULL,
	context TEXT NOT NULL DEFAULT '',
	context_param TEXT NOT NULL DEFAULT '',
	duration TEXT NOT NULL DEFAULT '',
	needs_focus INTEGER NOT NULL DEFAULT 0,
	description TEXT NOT NULL DEFAULT '',
	assigned_to TEXT NOT NULL DEFAULT '',
	due_date TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	last_reviewed_at TEXT NOT NULL,
	became_next_at TEXT,
	snooze_until TEXT NOT NULL DEFAULT '',
	snooze_action_id INTEGER REFERENCES actions(id) ON DELETE SET NULL,
	completed_at TEXT
);
CREATE TABLE IF NOT EXISTS tags (name TEXT PRIMARY KEY) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS item_tags (
	item_type TEXT NOT NULL,
	item_id INTEGER NOT NULL,
	tag TEXT NOT NULL,
	PRIMARY KEY (item_type, item_id, tag)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS contexts (name TEXT PRIMARY KEY) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS verbs (word TEXT PRIMARY KEY) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS context_params (
	context TEXT NOT NULL,
	value TEXT NOT NULL,
	PRIMARY KEY (context, value)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS schedules (
	id INTEGER PRIMARY KEY,
	text TEXT NOT NULL,
	rule TEXT NOT NULL,
	suffix TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	counted_from TEXT NOT NULL,
	last_fired_at TEXT,
	last_reviewed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS audit_log (
	id INTEGER PRIMARY KEY,
	at TEXT NOT NULL,
	event TEXT NOT NULL,
	item_type TEXT NOT NULL,
	item_id INTEGER NOT NULL DEFAULT 0,
	snapshot TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS app_state (key TEXT PRIMARY KEY, value TEXT NOT NULL) WITHOUT ROWID;
-- The one index in this database, and the Dashboard is what earned it. Every
-- other screen queries what is open, which is bounded by how much you have
-- going on; the Dashboard queries the log, which is bounded by how long you
-- have been using the app. Its two duration panels pair each leaving event
-- with the item's own creation as a correlated subquery, so without this the
-- work is rows-that-left x whole-log and the screen cost a quarter of a second
-- on one year of moderate use — twenty times any other page. With it, one
-- millisecond. The column order is the order the subquery narrows in.
CREATE INDEX IF NOT EXISTS idx_audit_item ON audit_log(item_type, item_id, event, at);
`

func (a *App) migrate() error {
	if _, err := a.db.Exec(schema); err != nil {
		return err
	}
	// The schema above only ever creates what is missing, so a database made
	// by an older build needs the rest said out loud. Each step is written to
	// be a no-op the second time it runs.
	has, err := a.hasColumn("projects", "description")
	if err != nil {
		return err
	}
	if has {
		// A project's own description was dropped: its definition of done is
		// the thing worth writing, and a second free-text field beside it was
		// somewhere for the same sentence to be written twice. Material that
		// belongs to a commitment lives on the action it belongs to (design.md,
		// "Deliberate omissions").
		if _, err := a.db.Exec(`ALTER TABLE projects DROP COLUMN description`); err != nil {
			return err
		}
	}
	has, err = a.hasColumn("someday_items", "snooze_until")
	if err != nil {
		return err
	}
	if has {
		// A someday/maybe item stopped having a snooze. Every other way out of
		// that list now goes through the inbox, and a date that only ever meant
		// "do not show me this yet" on a list you already chose to open was
		// hiding ideas from the one walk that exists to look at them
		// (design.md, "Someday/maybe item").
		if _, err := a.db.Exec(`ALTER TABLE someday_items DROP COLUMN snooze_until`); err != nil {
			return err
		}
	}
	// An action may now wait on a sibling instead of on a date, which is the
	// column that link lives in. ON DELETE SET NULL is the rule itself and not
	// a tidying-up convenience: deleting the blocker is one of the two things
	// that wakes what was waiting on it (design.md, "Time fields").
	has, err = a.hasColumn("actions", "snooze_action_id")
	if err != nil {
		return err
	}
	if !has {
		if _, err := a.db.Exec(`ALTER TABLE actions ADD COLUMN snooze_action_id INTEGER
			REFERENCES actions(id) ON DELETE SET NULL`); err != nil {
			return err
		}
	}
	// A project points at the one of its actions that is next (design.md,
	// "Project"). Nullable, and null is the ordinary state rather than a gap
	// to be backfilled: a project that has never been pointed at falls back to
	// the first open, unsnoozed action in its plan, which is exactly what every
	// project did before the column existed. ON DELETE SET NULL for the same
	// reason snooze_action_id has it — deleting the action is one of the ways a
	// project stops having a next action.
	has, err = a.hasColumn("projects", "next_action_id")
	if err != nil {
		return err
	}
	if !has {
		if _, err := a.db.Exec(`ALTER TABLE projects ADD COLUMN next_action_id INTEGER
			REFERENCES actions(id) ON DELETE SET NULL`); err != nil {
			return err
		}
	}
	// #parked is gone, and with it the one meaning an empty became_next_at
	// had. Every action became available to be worked on when it was written;
	// what used to be said by parking is said by waiting on the sibling that
	// comes first.
	//
	// An open action is stamped with the moment the parking was lifted, the
	// way a detach stamps one — becoming available is an event, and it is
	// happening now. A completed one is stamped with its own completion, since
	// its clock is closed and no event is reaching it; COALESCE says both in
	// one expression.
	if _, err := a.db.Exec(`UPDATE actions SET became_next_at = COALESCE(completed_at, ?)
		WHERE became_next_at IS NULL`, ts(a.now())); err != nil {
		return err
	}
	// The duration buckets stopped naming minutes. Two of the four old buckets
	// were both "small enough to just do", which is why they collapse together.
	for _, m := range [][2]string{
		{"<5min", "short"}, {"<15min", "short"}, {"<1h", "medium"}, {">1h", "long"},
	} {
		if _, err := a.db.Exec(`UPDATE actions SET duration=? WHERE duration=?`, m[1], m[0]); err != nil {
			return err
		}
	}
	// An inbox item says which way in it arrived by (design.md, "Inbox item").
	// A database made before the field existed gets the column with '' in every
	// row, and '' is read as "captured before this was recorded": there is no
	// honest name to backfill with, and picking one would put captures in a
	// channel's count that it never made.
	has, err = a.hasColumn("inbox_items", "source")
	if err != nil {
		return err
	}
	if !has {
		if _, err := a.db.Exec(`ALTER TABLE inbox_items ADD COLUMN source TEXT NOT NULL DEFAULT ''`); err != nil {
			return err
		}
	}
	// The verb list starts with something on it, once and once only — a seed
	// rather than a set of defaults, so that a word taken off stays off (see
	// verbs.go).
	return a.seedVerbs()
}

func (a *App) hasColumn(table, column string) (bool, error) {
	rows, err := a.db.Query(`SELECT 1 FROM pragma_table_info(?) WHERE name=?`, table, column)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}

// Today returns the current day in the app's configured timezone.
func (a *App) Today() string { return a.now().In(a.loc).Format(DateFormat) }

// Loc is the one timezone that defines "today" (README, -tz). A timestamp the
// app shows as a day has to be turned into one somewhere, and doing it against
// any other zone would put a completion on the wrong side of midnight.
func (a *App) Loc() *time.Location { return a.loc }

// --- small scan/store helpers -------------------------------------------

const tsFormat = time.RFC3339Nano

func ts(t time.Time) string { return t.UTC().Format(tsFormat) }

func tsPtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return ts(*t)
}

func parseTS(s string) time.Time {
	t, _ := time.Parse(tsFormat, s)
	return t
}

func parseTSPtr(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := parseTS(s.String)
	return &t
}

// ValidDate reports whether s is a well-formed YYYY-MM-DD date.
func ValidDate(s string) bool {
	_, err := time.Parse(DateFormat, s)
	return err == nil
}

// --- audit ---------------------------------------------------------------

// audit writes one entry; the snapshot is the item as it was at the moment
// that matters for recovery (before a delete/edit, as created for a create).
func (a *App) audit(tx *sql.Tx, event, itemType string, itemID int64, item any) error {
	snap := ""
	if item != nil {
		b, err := json.Marshal(item)
		if err != nil {
			return fmt.Errorf("audit snapshot: %w", err)
		}
		snap = string(b)
	}
	_, err := tx.Exec(`INSERT INTO audit_log (at, event, item_type, item_id, snapshot) VALUES (?,?,?,?,?)`,
		ts(a.now()), event, itemType, itemID, snap)
	return err
}

// tx runs fn inside a transaction.
func (a *App) tx(fn func(tx *sql.Tx) error) error {
	tx, err := a.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// SetState stores one piece of app state (e.g. a view's remembered
// filters). Filters are momentary state and live on no item.
func (a *App) SetState(key, value string) error {
	_, err := a.db.Exec(`INSERT INTO app_state (key,value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (a *App) GetState(key string) (string, error) {
	var v string
	err := a.db.QueryRow(`SELECT value FROM app_state WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// AuditLog returns recent audit entries, newest first.
func (a *App) AuditLog(limit int) ([]*AuditEntry, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := a.db.Query(`SELECT id, at, event, item_type, item_id, snapshot FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AuditEntry
	for rows.Next() {
		e := &AuditEntry{}
		var at string
		if err := rows.Scan(&e.ID, &at, &e.Event, &e.ItemType, &e.ItemID, &e.Snapshot); err != nil {
			return nil, err
		}
		e.At = parseTS(at)
		out = append(out, e)
	}
	return out, rows.Err()
}
