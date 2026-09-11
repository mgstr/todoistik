package app

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrTitle = errors.New("a title is required")

// ErrCompleted is what every write refuses with when the item it names is
// already completed. A finished item is frozen: the Archive is the record of
// what was done, and a record that can be rewritten in place is not one
// (design.md, "Completion"). It is a rule of the domain rather than of the
// screens, so that a second tab, a stale page or the next way in cannot walk
// around it.
//
// Bringing an item back is the one write a completed item accepts, and it is
// deliberately not guarded: clearing completedAt is what unfreezes it, and
// after that it is an ordinary item again.
var ErrCompleted = errors.New("that item is completed — bring it back first to change it")

// ActionFields is everything settable on an action from a form.
type ActionFields struct {
	Title        string
	Context      string
	ContextParam string
	Duration     Duration
	NeedsFocus   bool
	Description  string
	AssignedTo   string
	DueDate      string
	SnoozeUntil  string
	Tags         []string
}

func (f *ActionFields) validate() error {
	f.Title = strings.TrimSpace(f.Title)
	if f.Title == "" {
		return ErrTitle
	}
	if !f.Duration.Valid() {
		return fmt.Errorf("bad duration %q", f.Duration)
	}
	if f.DueDate != "" && !ValidDate(f.DueDate) {
		return fmt.Errorf("bad due date %q", f.DueDate)
	}
	if f.SnoozeUntil != "" && !ValidDate(f.SnoozeUntil) {
		return fmt.Errorf("bad snooze date %q", f.SnoozeUntil)
	}
	f.Context = strings.TrimPrefix(strings.TrimSpace(f.Context), "@")
	if f.Context == "" {
		f.ContextParam = ""
	}
	return nil
}

// CreateAction creates an action. projectID 0 means standalone, and a
// standalone action is always a next action, so becameNextActionAt is
// stamped. An action added to a project is a next action too, by default —
// parking is the deliberate act (design.md, "Editing items"). A non-empty
// assignedTo makes it a waiting-for action (stamped as delegation date).
func (a *App) CreateAction(projectID int64, f ActionFields, parked bool) (*Action, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	if parked && projectID == 0 {
		return nil, errors.New("only an action inside a project can be parked")
	}
	now := a.now().UTC()
	act := &Action{
		ProjectID: projectID, Title: f.Title,
		Context: f.Context, ContextParam: f.ContextParam,
		Duration: f.Duration, NeedsFocus: f.NeedsFocus,
		Description: f.Description, AssignedTo: strings.TrimSpace(f.AssignedTo),
		DueDate: f.DueDate, SnoozeUntil: f.SnoozeUntil, Tags: normTags(f.Tags),
		CreatedAt: now, LastReviewedAt: now,
	}
	if !parked {
		act.BecameNextAt = &now
	}
	err := a.tx(func(tx *sql.Tx) error {
		// a finished project takes no new work: the commitment it named is
		// met, and an action added to it would be work nobody is tracking
		if projectID != 0 {
			if _, err := a.openProjectRowTx(tx, projectID); err != nil {
				return err
			}
		}
		return a.insertActionTx(tx, act)
	})
	if err != nil {
		return nil, err
	}
	return act, nil
}

func (a *App) insertActionTx(tx *sql.Tx, act *Action) error {
	var pid any
	if act.ProjectID != 0 {
		pid = act.ProjectID
	}
	res, err := tx.Exec(`INSERT INTO actions
		(project_id, title, context, context_param, duration, needs_focus, description,
		 assigned_to, due_date, created_at, last_reviewed_at, became_next_at, snooze_until, completed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		pid, act.Title, act.Context, act.ContextParam, string(act.Duration), act.NeedsFocus,
		act.Description, act.AssignedTo, act.DueDate, ts(act.CreatedAt), ts(act.LastReviewedAt),
		tsPtr(act.BecameNextAt), act.SnoozeUntil, tsPtr(act.CompletedAt))
	if err != nil {
		return err
	}
	act.ID, _ = res.LastInsertId()
	if err := a.setTagsTx(tx, "action", act.ID, act.Tags); err != nil {
		return err
	}
	if err := a.rememberContextTx(tx, act.Context, act.ContextParam); err != nil {
		return err
	}
	return a.audit(tx, EvCreated, "action", act.ID, act)
}

const actionCols = `a.id, a.project_id, a.title, a.context, a.context_param, a.duration,
	a.needs_focus, a.description, a.assigned_to, a.due_date, a.created_at,
	a.last_reviewed_at, a.became_next_at, a.snooze_until, a.completed_at`

func scanAction(row rowScanner) (*Action, error) {
	act := &Action{}
	var pid sql.NullInt64
	var created, reviewed, dur string
	var next, completed sql.NullString
	if err := row.Scan(&act.ID, &pid, &act.Title, &act.Context, &act.ContextParam, &dur,
		&act.NeedsFocus, &act.Description, &act.AssignedTo, &act.DueDate, &created,
		&reviewed, &next, &act.SnoozeUntil, &completed); err != nil {
		return nil, err
	}
	act.ProjectID = pid.Int64
	act.Duration = Duration(dur)
	act.CreatedAt = parseTS(created)
	act.LastReviewedAt = parseTS(reviewed)
	act.BecameNextAt = parseTSPtr(next)
	act.CompletedAt = parseTSPtr(completed)
	return act, nil
}

func (a *App) actionTx(tx *sql.Tx, id int64) (*Action, error) {
	act, err := scanAction(tx.QueryRow(`SELECT `+actionCols+` FROM actions a WHERE a.id=?`, id))
	if err != nil {
		return nil, err
	}
	act.Tags, err = tagsForTx(tx, "action", id)
	return act, err
}

// openActionTx loads an action that is about to be written to, and refuses if
// it is completed — the one place the freeze is enforced for actions, so that
// every write either goes through it or is the one that lifts the freeze.
func (a *App) openActionTx(tx *sql.Tx, id int64) (*Action, error) {
	act, err := a.actionTx(tx, id)
	if err != nil {
		return nil, err
	}
	if act.CompletedAt != nil {
		return nil, ErrCompleted
	}
	return act, nil
}

// Action loads one action with its tags (and project title, when any).
func (a *App) Action(id int64) (*Action, error) {
	var act *Action
	err := a.tx(func(tx *sql.Tx) error {
		var err error
		act, err = a.actionTx(tx, id)
		if err != nil {
			return err
		}
		if act.ProjectID != 0 {
			return tx.QueryRow(`SELECT title FROM projects WHERE id=?`, act.ProjectID).Scan(&act.ProjectTitle)
		}
		return nil
	})
	return act, err
}

// UpdateAction applies edited fields. Changing "assigned to" restamps
// becameNextActionAt in both directions: a new clock starts whenever the
// waiting changes hands (design.md, "Time fields").
func (a *App) UpdateAction(id int64, f ActionFields) error {
	if err := f.validate(); err != nil {
		return err
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openActionTx(tx, id)
		if err != nil {
			return err
		}
		next := before.BecameNextAt
		if strings.TrimSpace(f.AssignedTo) != before.AssignedTo {
			now := a.now().UTC()
			next = &now
		}
		if _, err := tx.Exec(`UPDATE actions SET title=?, context=?, context_param=?, duration=?,
			needs_focus=?, description=?, assigned_to=?, due_date=?, snooze_until=?, became_next_at=?
			WHERE id=?`,
			f.Title, f.Context, f.ContextParam, string(f.Duration), f.NeedsFocus,
			f.Description, strings.TrimSpace(f.AssignedTo), f.DueDate, f.SnoozeUntil, tsPtr(next), id); err != nil {
			return err
		}
		if err := a.setTagsTx(tx, "action", id, normTags(f.Tags)); err != nil {
			return err
		}
		if err := a.rememberContextTx(tx, f.Context, f.ContextParam); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "action", id, before)
	})
}

// CompleteAction marks an action done.
func (a *App) CompleteAction(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		// completing what is already complete would restamp the moment the
		// work was finished, which is the one thing the Archive is for
		if _, err := a.openActionTx(tx, id); err != nil {
			return err
		}
		return a.completeActionTx(tx, id, a.now())
	})
}

// UncompleteAction brings back something completed by mistake.
func (a *App) UncompleteAction(id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.setActionCompleted(tx, id, nil) })
}

// completeActionTx completes an action at a given moment.
//
// The moment is a parameter rather than always being now, because there is one
// path where the two differ: confirming a completion request stamps the time
// the work was actually finished elsewhere, which may be days before the
// answer (design.md, "Completion"). Every other caller passes the moment of
// the click, so the ordinary path is this path and there is no second way to
// complete an action.
func (a *App) completeActionTx(tx *sql.Tx, id int64, at time.Time) error {
	return a.setActionCompleted(tx, id, &at)
}

func (a *App) setActionCompleted(tx *sql.Tx, id int64, at *time.Time) error {
	before, err := a.actionTx(tx, id)
	if err != nil {
		return err
	}
	var stamp any
	event := EvUncompleted
	if at != nil {
		stamp = ts(*at)
		event = EvCompleted
	}
	if _, err := tx.Exec(`UPDATE actions SET completed_at=? WHERE id=?`, stamp, id); err != nil {
		return err
	}
	return a.audit(tx, event, "action", id, before)
}

// DeleteAction resolves an action by deleting it — audited and recoverable.
func (a *App) DeleteAction(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openActionTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM actions WHERE id=?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type='action' AND item_id=?`, id); err != nil {
			return err
		}
		return a.audit(tx, EvDeleted, "action", id, before)
	})
}

// SetNext marks an action as next (true) or parks it (false). Parking is
// only meaningful inside a project; a standalone action is always next.
func (a *App) SetNext(id int64, next bool) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openActionTx(tx, id)
		if err != nil {
			return err
		}
		if !next && before.ProjectID == 0 {
			return errors.New("a standalone action is always a next action; snooze it instead")
		}
		var at any
		if next {
			at = ts(a.now())
			if before.BecameNextAt != nil {
				return nil // already next; keep the original clock
			}
		}
		if _, err := tx.Exec(`UPDATE actions SET became_next_at=? WHERE id=?`, at, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "action", id, before)
	})
}

// SnoozeAction sets or clears snoozeUntil ("" clears).
func (a *App) SnoozeAction(id int64, until string) error {
	if until != "" && !ValidDate(until) {
		return fmt.Errorf("bad date %q", until)
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openActionTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE actions SET snooze_until=? WHERE id=?`, until, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "action", id, before)
	})
}

// Detach makes a project action standalone. A standalone action is always a
// next action, so a parked one gets becameNextActionAt stamped with the
// detach time. Everything else carries over (design.md, "Reshaping items").
func (a *App) Detach(id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.detachTx(tx, id) })
}

func (a *App) detachTx(tx *sql.Tx, id int64) error {
	before, err := a.openActionTx(tx, id)
	if err != nil {
		return err
	}
	if before.ProjectID == 0 {
		return errors.New("already standalone")
	}
	next := before.BecameNextAt
	if next == nil {
		now := a.now().UTC()
		next = &now
	}
	if _, err := tx.Exec(`UPDATE actions SET project_id=NULL, became_next_at=? WHERE id=?`, tsPtr(next), id); err != nil {
		return err
	}
	return a.audit(tx, EvDetached, "action", id, before)
}

// ToggleTag adds or removes one tag on an action or project. #today is the
// one tag whose changes are never audited (design.md, "#today").
func (a *App) ToggleTag(itemType string, id int64, tag string) error {
	tag = normTag(tag)
	if tag == "" {
		return ErrEmpty
	}
	if itemType != "action" && itemType != "project" {
		return fmt.Errorf("no tags on %q", itemType)
	}
	return a.tx(func(tx *sql.Tx) error {
		// the freeze reaches the cheapest edit in the app as well: a tag is
		// what an item is about, and a finished item was about what it was
		// about when it was finished
		if err := refuseCompletedTx(tx, itemType, id); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM item_tags WHERE item_type=? AND item_id=? AND tag=?`,
			itemType, id, tag).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type=? AND item_id=? AND tag=?`, itemType, id, tag); err != nil {
				return err
			}
		} else {
			if err := ensureTagTx(tx, tag); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO item_tags (item_type, item_id, tag) VALUES (?,?,?)`, itemType, id, tag); err != nil {
				return err
			}
		}
		if tag == TodayTag {
			return nil
		}
		return a.audit(tx, EvEdited, itemType, id, map[string]any{"toggledTag": tag})
	})
}

// refuseCompletedTx is the freeze for the writes that touch an item without
// loading it — the tag toggle, which knows only a type and an id.
func refuseCompletedTx(tx *sql.Tx, itemType string, id int64) error {
	table := "actions"
	if itemType == "project" {
		table = "projects"
	}
	var completed sql.NullString
	if err := tx.QueryRow(`SELECT completed_at FROM `+table+` WHERE id=?`, id).Scan(&completed); err != nil {
		return err
	}
	if completed.Valid {
		return ErrCompleted
	}
	return nil
}

func normTag(t string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), "#"))
}

func normTags(tags []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range tags {
		t = normTag(t)
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
