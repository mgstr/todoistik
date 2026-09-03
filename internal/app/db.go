package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type App struct {
	db  *sql.DB
	loc *time.Location // the one configured timezone that defines "today"
	now func() time.Time
}

func Open(path string, loc *time.Location) (*App, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// modernc/sqlite serializes writes poorly across many conns; one is plenty
	// for a single-user app and removes SQLITE_BUSY from the picture.
	db.SetMaxOpenConns(1)
	a := &App{db: db, loc: loc, now: time.Now}
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
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS someday_items (
	id INTEGER PRIMARY KEY,
	text TEXT NOT NULL,
	created_at TEXT NOT NULL,
	last_reviewed_at TEXT NOT NULL,
	snooze_until TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS projects (
	id INTEGER PRIMARY KEY,
	title TEXT NOT NULL,
	dod TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	last_reviewed_at TEXT NOT NULL,
	snooze_until TEXT NOT NULL DEFAULT '',
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
`

func (a *App) migrate() error {
	_, err := a.db.Exec(schema)
	return err
}

// Today returns the current day in the app's configured timezone.
func (a *App) Today() string { return a.now().In(a.loc).Format(DateFormat) }

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
