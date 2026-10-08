package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Undo takes back the last thing done at the keyboard (design.md, "Undo").
//
// What it is built out of is the database writing down its own way back. Every
// table an item lives in carries three triggers, and each one records the
// statement that would reverse the write that fired it: a DELETE for an
// insert, an INSERT of the whole row for a delete, an UPDATE of the columns
// that changed for an update. Replayed newest first, a step's statements put
// every row it touched back exactly as it was — the tags, the plan's order,
// the pointer at the next action — without one line here knowing what a
// project is. The alternative was an inverse written by hand for each of the
// thirty-odd writes in this package, and the first one added without its
// inverse would have been an undo that half worked (implementation.md,
// "Undo").

// undoTables are the tables whose writes are recorded: every item, what items
// carry, and the remembered lists a write may add a name to. The audit log is
// not here — it is added to and never taken back — and neither is app_state,
// which holds how the screen is set and nothing that was done.
var undoTables = []string{
	"inbox_items", "someday_items", "reference_items", "projects", "actions",
	"item_tags", "schedules", "tags", "contexts", "context_params", "verbs",
}

// undoDepth is how many steps are kept. Far more than are ever walked back in
// one sitting; the number exists so that the log is bounded by something other
// than how long the app has been used.
const undoDepth = 100

// EvUndo is what the audit log says about an undo: one entry, after the
// entries of the step it took back, naming them. Those stay — the log is added
// to and never rewritten — so it reads as what happened: a thing was done, and
// then it was taken back (design.md, "Undo").
const EvUndo = "undo"

var (
	// ErrNothingToUndo: the stack is empty.
	ErrNothingToUndo = errors.New("nothing to undo")
	// ErrUndoMoved: the step the question was asked about is no longer the
	// last one. Something was done between the asking and the answer — in
	// another window, most likely — and taking back a step other than the one
	// that was read out is the one thing a confirmation exists to prevent.
	ErrUndoMoved = errors.New("something else was done since — ask again")
)

const undoSchema = `
CREATE TABLE IF NOT EXISTS undo_steps (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	at TEXT NOT NULL,
	audits TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS undo_log (
	seq INTEGER PRIMARY KEY,
	step INTEGER NOT NULL DEFAULT 0,
	sql TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_undo_log_step ON undo_log(step);
`

// gesture is one thing a person did: everything written between its start and
// its end is one step, however many transactions the handler took to write it.
type gesture struct {
	step int64 // the step this gesture's writes belong to; 0 until one is made
}

// Gesture runs fn as one thing done at the keyboard. What it writes is
// recorded as a single undo step, provided it wrote an audit entry — the log's
// own test of whether something happened, so that picking an item for today,
// which is never audited, is never a step either.
//
// Writes made outside a gesture are not recorded at all: a capture arriving
// through the API, a schedule firing at the day boundary. Those were not done
// at this keyboard, and `u` taking back a mail that arrived a minute ago would
// be the app losing something nobody decided to lose.
func (a *App) Gesture(fn func()) {
	a.gmu.Lock()
	defer a.gmu.Unlock()
	a.gesture.Store(&gesture{})
	defer func() {
		a.gesture.Store(nil)
		// what the gesture recorded and never made a step of
		a.db.Exec(`DELETE FROM undo_log WHERE step=0`)
	}()
	fn()
}

// Outside runs fn as a write that is nobody's gesture. It takes the lock a
// gesture holds, so that a capture posted while a Save is half-way through its
// transactions cannot land inside that Save's step.
func (a *App) Outside(fn func()) {
	a.gmu.Lock()
	defer a.gmu.Unlock()
	fn()
}

// dropUndoTriggers takes the recording off. Before a migration, because SQLite
// refuses to drop a column a trigger names; and on Close, so that a database
// nobody has open is plain tables, alterable by hand.
func (a *App) dropUndoTriggers() error {
	for _, t := range undoTables {
		for _, op := range []string{"i", "u", "d"} {
			if _, err := a.db.Exec(`DROP TRIGGER IF EXISTS undo_` + t + `_` + op); err != nil {
				return err
			}
		}
	}
	return nil
}

// createUndoTriggers writes the three triggers for every recorded table, from
// the table's own columns as they are now — so a column added by a migration
// is recorded from the next start, with nothing here to keep in step.
func (a *App) createUndoTriggers() error {
	for _, t := range undoTables {
		cols, keys, err := a.tableColumns(t)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			return fmt.Errorf("undo: %s has no primary key", t)
		}
		where := func(row string) string {
			parts := make([]string, len(keys))
			for i, k := range keys {
				parts[i] = `'` + k + `='||quote(` + row + `.` + k + `)`
			}
			return strings.Join(parts, `||' AND '||`)
		}
		// A row keyed by a name is the same row whoever puts it back, so its
		// return may find it already there. A row keyed by an id is one
		// particular item, and finding another in its place has to fail the
		// whole undo rather than quietly lose one of the two.
		insert := "INSERT"
		if len(keys) > 1 || keys[0] != "id" {
			insert = "INSERT OR IGNORE"
		}
		vals := make([]string, len(cols))
		var changed, sets []string
		for i, c := range cols {
			vals[i] = `quote(OLD.` + c + `)`
			if !contains(keys, c) {
				changed = append(changed, `OLD.`+c+` IS NOT NEW.`+c)
				sets = append(sets, `CASE WHEN OLD.`+c+` IS NOT NEW.`+c+
					` THEN '`+c+`='||quote(OLD.`+c+`)||',' ELSE '' END`)
			}
		}
		stmts := []string{
			`CREATE TRIGGER undo_` + t + `_i AFTER INSERT ON ` + t + ` BEGIN
				INSERT INTO undo_log (sql) VALUES ('DELETE FROM ` + t + ` WHERE '||` + where("NEW") + `);
			END`,
			`CREATE TRIGGER undo_` + t + `_d AFTER DELETE ON ` + t + ` BEGIN
				INSERT INTO undo_log (sql) VALUES ('` + insert + ` INTO ` + t + ` (` + strings.Join(cols, ",") +
				`) VALUES ('||` + strings.Join(vals, `||','||`) + `||')');
			END`,
		}
		if len(sets) > 0 {
			// only the columns that moved: a step is taken back without
			// touching what something else has since written to the same row —
			// the day boundary stamping a schedule as fired, most of all
			stmts = append(stmts, `CREATE TRIGGER undo_`+t+`_u AFTER UPDATE ON `+t+
				` WHEN `+strings.Join(changed, " OR ")+` BEGIN
				INSERT INTO undo_log (sql) VALUES ('UPDATE `+t+` SET '||rtrim(`+strings.Join(sets, "||")+
				`, ',')||' WHERE '||`+where("OLD")+`);
			END`)
		}
		for _, s := range stmts {
			if _, err := a.db.Exec(s); err != nil {
				return fmt.Errorf("undo trigger on %s: %w", t, err)
			}
		}
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// tableColumns is a table's columns in order, and the ones that key it.
func (a *App) tableColumns(table string) (cols, keys []string, err error) {
	rows, err := a.db.Query(`SELECT name, pk FROM pragma_table_info(?) ORDER BY cid`, table)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var pk int
		if err := rows.Scan(&name, &pk); err != nil {
			return nil, nil, err
		}
		cols = append(cols, name)
		if pk > 0 {
			keys = append(keys, name)
		}
	}
	return cols, keys, rows.Err()
}

// auditMark is where the audit log ends, read at the start of a transaction
// so that its end can say which entries the transaction wrote.
func auditMark(tx *sql.Tx) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT COALESCE(MAX(id),0) FROM audit_log`).Scan(&id)
	return id, err
}

// settleUndo decides, at the end of a transaction, what becomes of the
// statements its writes recorded. Outside a gesture they are thrown away.
// Inside one they join the gesture's step — made here, the first time the
// gesture writes an audit entry — and until it has written one they wait:
// a gesture that never audits anything never makes a step.
//
// It returns the step to remember on the gesture once the commit has gone
// through; before that the step is only a row that may yet be rolled back.
func (a *App) settleUndo(tx *sql.Tx, g *gesture, mark int64) (int64, error) {
	var pending bool
	if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM undo_log WHERE step=0)`).Scan(&pending); err != nil {
		return 0, err
	}
	if g == nil {
		if pending {
			_, err := tx.Exec(`DELETE FROM undo_log WHERE step=0`)
			return 0, err
		}
		return 0, nil
	}
	rows, err := tx.Query(`SELECT id FROM audit_log WHERE id > ? ORDER BY id`, mark)
	if err != nil {
		return 0, err
	}
	var audits strings.Builder
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		audits.WriteString("," + strconv.FormatInt(id, 10))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	step := g.step
	if step == 0 && audits.Len() == 0 {
		return 0, nil
	}
	if step == 0 {
		res, err := tx.Exec(`INSERT INTO undo_steps (at) VALUES (?)`, ts(a.now()))
		if err != nil {
			return 0, err
		}
		step, _ = res.LastInsertId()
		if _, err := tx.Exec(`DELETE FROM undo_steps WHERE id NOT IN
			(SELECT id FROM undo_steps ORDER BY id DESC LIMIT ?)`, undoDepth); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM undo_log WHERE step > 0
			AND step NOT IN (SELECT id FROM undo_steps)`); err != nil {
			return 0, err
		}
	}
	if pending {
		if _, err := tx.Exec(`UPDATE undo_log SET step=? WHERE step=0`, step); err != nil {
			return 0, err
		}
	}
	if audits.Len() > 0 {
		if _, err := tx.Exec(`UPDATE undo_steps SET audits = audits || ? WHERE id=?`, audits.String(), step); err != nil {
			return 0, err
		}
	}
	return step, nil
}

// UndoStep is one thing that can be taken back: when it was done, and what the
// audit log wrote about it — which is what the question reads out before
// anything is undone.
type UndoStep struct {
	ID      int64
	At      time.Time
	Entries []*UndoEntry
}

// UndoEntry is one audit entry of a step, with the name of the item it is
// about. The name is the snapshot's where the snapshot has one and the item's
// own otherwise: a review mark and a toggled tag snapshot no title, and "edited
// action" with nothing after it says too little to answer yes to.
type UndoEntry struct {
	*AuditEntry
	Name string
}

// Text is what an entry's snapshot calls the item, whole: its text for the
// kinds that have text, its title for the kinds that have one. It is what the
// Audit screen shows and what Recapture puts back in the inbox, so it is every
// line of a capture and not only the first.
func (e *AuditEntry) Text() string {
	var snap struct {
		Text  string `json:"text"`
		Title string `json:"title"`
	}
	if json.Unmarshal([]byte(e.Snapshot), &snap) != nil {
		return ""
	}
	if snap.Text == "" {
		return snap.Title
	}
	return snap.Text
}

// Name is the first line of that: the one line a list shows an item by.
func (e *AuditEntry) Name() string {
	line, _ := SplitCapture(e.Text())
	return line
}

// CanUndo reports whether there is a step to take back.
func (a *App) CanUndo() bool {
	var n int
	a.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM undo_steps)`).Scan(&n)
	return n == 1
}

// LastUndo is the step `u` would take back, or nil when there is none.
func (a *App) LastUndo() (*UndoStep, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	return a.lastUndoTx(tx)
}

func (a *App) lastUndoTx(tx *sql.Tx) (*UndoStep, error) {
	s := &UndoStep{}
	var at, audits string
	err := tx.QueryRow(`SELECT id, at, audits FROM undo_steps ORDER BY id DESC LIMIT 1`).Scan(&s.ID, &at, &audits)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.At = parseTS(at)
	for _, f := range strings.Split(audits, ",") {
		id, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			continue
		}
		e := &AuditEntry{}
		var eat string
		err = tx.QueryRow(`SELECT id, at, event, item_type, item_id, snapshot FROM audit_log WHERE id=?`, id).
			Scan(&e.ID, &eat, &e.Event, &e.ItemType, &e.ItemID, &e.Snapshot)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		e.At = parseTS(eat)
		ue := &UndoEntry{AuditEntry: e, Name: e.Name()}
		if ue.Name == "" {
			ue.Name = itemNameTx(tx, e.ItemType, e.ItemID)
		}
		s.Entries = append(s.Entries, ue)
	}
	return s, nil
}

// itemNames is where each kind of item keeps the words it is known by.
var itemNames = map[string][2]string{
	"inbox":     {"inbox_items", "text"},
	"someday":   {"someday_items", "text"},
	"reference": {"reference_items", "text"},
	"project":   {"projects", "title"},
	"action":    {"actions", "title"},
	"schedule":  {"schedules", "text"},
}

func itemNameTx(tx *sql.Tx, itemType string, id int64) string {
	at, ok := itemNames[itemType]
	if !ok {
		return ""
	}
	var name string
	tx.QueryRow(`SELECT `+at[1]+` FROM `+at[0]+` WHERE id=?`, id).Scan(&name)
	line, _ := SplitCapture(name)
	return line
}

// undone is the snapshot of an EvUndo entry: the entries that were taken back,
// and one line saying so for the Audit screen to show.
type undone struct {
	Text  string       `json:"text"`
	Undid []undoneItem `json:"undid"`
}

type undoneItem struct {
	Event    string `json:"event"`
	ItemType string `json:"itemType"`
	ItemID   int64  `json:"itemId,omitempty"`
	Name     string `json:"name,omitempty"`
}

// Undo takes back the step with this id, which must be the last one: the
// caller names the step it read out, and a stack that has moved since is
// refused rather than answered about a different step.
//
// Everything the step wrote is put back as it was, and one audit entry is
// written saying what was undone. The step is then gone — there is no redo —
// and the one before it is last.
func (a *App) Undo(id int64) (*UndoStep, error) {
	tx, err := a.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	step, err := a.lastUndoTx(tx)
	if err != nil {
		return nil, err
	}
	if step == nil {
		return nil, ErrNothingToUndo
	}
	if step.ID != id {
		return nil, ErrUndoMoved
	}
	// The statements go back in the reverse of the order they were recorded
	// in, which is not an order the foreign keys agree with half-way: a
	// project returns before the action it points at as next. Checked at the
	// commit instead, where the whole of it is back.
	if _, err := tx.Exec(`PRAGMA defer_foreign_keys = ON`); err != nil {
		return nil, err
	}
	rows, err := tx.Query(`SELECT sql FROM undo_log WHERE step=? ORDER BY seq DESC`, id)
	if err != nil {
		return nil, err
	}
	var stmts []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return nil, err
		}
		stmts = append(stmts, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return nil, fmt.Errorf("undo: %w", err)
		}
	}
	if err := tidyAfterUndoTx(tx); err != nil {
		return nil, err
	}
	rec := undone{}
	// The step's own entries stay where they are: the audit log is never
	// rewritten (implementation.md, "Schema changes"), and what happened is
	// that a thing was done and then taken back — two facts, in that order.
	for _, e := range step.Entries {
		rec.Undid = append(rec.Undid, undoneItem{e.Event, e.ItemType, e.ItemID, e.Name})
	}
	var itemType string
	var itemID int64
	if len(step.Entries) > 0 {
		first := step.Entries[0]
		itemType, itemID = first.ItemType, first.ItemID
		rec.Text = first.Event + ": " + first.Name
		if more := len(step.Entries) - 1; more > 0 {
			rec.Text += fmt.Sprintf(" (+%d)", more)
		}
	}
	if err := a.audit(tx, EvUndo, itemType, itemID, rec); err != nil {
		return nil, err
	}
	// the step, and whatever replaying it has just recorded: an undo is not
	// itself a thing to undo
	if _, err := tx.Exec(`DELETE FROM undo_log WHERE step=? OR step=0`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM undo_steps WHERE id=?`, id); err != nil {
		return nil, err
	}
	return step, tx.Commit()
}

// tidyAfterUndoTx puts right the two things a replay cannot know about,
// because they were written outside any step.
//
// A pick for today is never a step, so one made on an item whose creation is
// then undone is left pointing at nothing — and the next item to be given that
// id would be born picked. And a name taken off a remembered list in Settings
// is not a step either, so a tag an undo hands back to an item may no longer
// be on the list; every name an item carries has to be (design.md, "Tags").
func tidyAfterUndoTx(tx *sql.Tx) error {
	for itemType, at := range itemNames {
		if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type=? AND item_id NOT IN (SELECT id FROM `+at[0]+`)`, itemType); err != nil {
			return err
		}
	}
	for _, s := range []string{
		`INSERT OR IGNORE INTO tags (name) SELECT DISTINCT tag FROM item_tags WHERE tag != '` + TodayTag + `'`,
		`INSERT OR IGNORE INTO contexts (name) SELECT DISTINCT context FROM actions WHERE context != ''`,
		`INSERT OR IGNORE INTO context_params (context, value)
			SELECT DISTINCT context, context_param FROM actions WHERE context != '' AND context_param != ''`,
	} {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
