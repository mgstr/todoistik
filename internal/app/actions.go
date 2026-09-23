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
	// SnoozeActionID is the sibling this action waits on, 0 for none. The
	// meta line resolves a title or an id into it before it gets here; what
	// this layer checks is the part notation cannot see — that the sibling is
	// in the same project, still open, and not already waiting on this one.
	SnoozeActionID int64
	Tags           []string
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
	if f.SnoozeUntil != "" && f.SnoozeActionID != 0 {
		return errors.New("an action waits on a date or on another action, not on both")
	}
	f.Context = strings.TrimPrefix(strings.TrimSpace(f.Context), "@")
	if f.Context == "" {
		f.ContextParam = ""
	}
	return nil
}

// CreateAction creates an action. projectID 0 means standalone. Every action
// is a next action of whatever it belongs to, so becameNextActionAt is always
// stamped; an action that cannot be started yet carries a snooze, which is a
// claim about when rather than about whether (design.md, "Time fields"). A
// non-empty assignedTo makes it a waiting-for action (stamped as delegation
// date).
func (a *App) CreateAction(projectID int64, f ActionFields) (*Action, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	now := a.now().UTC()
	act := &Action{
		ProjectID: projectID, Title: f.Title,
		Context: f.Context, ContextParam: f.ContextParam,
		Duration: f.Duration, NeedsFocus: f.NeedsFocus,
		Description: f.Description, AssignedTo: strings.TrimSpace(f.AssignedTo),
		DueDate: f.DueDate, SnoozeUntil: f.SnoozeUntil, SnoozeActionID: f.SnoozeActionID,
		Tags: normTags(f.Tags), CreatedAt: now, LastReviewedAt: now,
		BecameNextAt: &now,
	}
	err := a.tx(func(tx *sql.Tx) error {
		// a finished project takes no new work: the commitment it named is
		// met, and an action added to it would be work nobody is tracking
		if projectID != 0 {
			if _, err := a.openProjectRowTx(tx, projectID); err != nil {
				return err
			}
		}
		// 0 is the new action's own id: it does not have one yet, and it
		// cannot be waited on by anything, so no chain can run back to it
		if err := a.checkBlockerTx(tx, 0, projectID, act.SnoozeActionID); err != nil {
			return err
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
		 assigned_to, due_date, created_at, last_reviewed_at, became_next_at, snooze_until,
		 snooze_action_id, completed_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		pid, act.Title, act.Context, act.ContextParam, string(act.Duration), act.NeedsFocus,
		act.Description, act.AssignedTo, act.DueDate, ts(act.CreatedAt), ts(act.LastReviewedAt),
		tsPtr(act.BecameNextAt), act.SnoozeUntil, nullID(act.SnoozeActionID), tsPtr(act.CompletedAt))
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

// The blocker's title comes back with the row rather than being looked up
// afterwards, because every caller that shows an action shows the snooze, and
// a snooze that named an id would be a badge saying nothing.
const actionCols = `a.id, a.project_id, a.title, a.context, a.context_param, a.duration,
	a.needs_focus, a.description, a.assigned_to, a.due_date, a.created_at,
	a.last_reviewed_at, a.became_next_at, a.snooze_until, a.snooze_action_id,
	COALESCE((SELECT b.title FROM actions b WHERE b.id = a.snooze_action_id), ''), a.completed_at`

func scanAction(row rowScanner) (*Action, error) {
	act := &Action{}
	var pid, blocker sql.NullInt64
	var created, reviewed, dur string
	var next, completed sql.NullString
	if err := row.Scan(&act.ID, &pid, &act.Title, &act.Context, &act.ContextParam, &dur,
		&act.NeedsFocus, &act.Description, &act.AssignedTo, &act.DueDate, &created,
		&reviewed, &next, &act.SnoozeUntil, &blocker, &act.SnoozeActionTitle, &completed); err != nil {
		return nil, err
	}
	act.ProjectID = pid.Int64
	act.SnoozeActionID = blocker.Int64
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
		if err := a.checkBlockerTx(tx, id, before.ProjectID, f.SnoozeActionID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE actions SET title=?, context=?, context_param=?, duration=?,
			needs_focus=?, description=?, assigned_to=?, due_date=?, snooze_until=?,
			snooze_action_id=?, became_next_at=? WHERE id=?`,
			f.Title, f.Context, f.ContextParam, string(f.Duration), f.NeedsFocus,
			f.Description, strings.TrimSpace(f.AssignedTo), f.DueDate, f.SnoozeUntil,
			nullID(f.SnoozeActionID), tsPtr(next), id); err != nil {
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
	// finishing this is what the actions waiting on it were waiting for, so
	// they wake here — the same moment, in the same transaction, because an
	// action that stayed asleep past its reason would be missing from "Next
	// actions" with nothing left to explain why.
	//
	// Bringing this one back does not put them to sleep again, and that is
	// deliberate: the waiting is a claim about an ordering, and the ordering
	// was settled the moment the blocker was finished. Re-snoozing them would
	// silently pull work out of the working view on the strength of an undo
	// (design.md, "Time fields").
	if at != nil {
		if _, err := tx.Exec(`UPDATE actions SET snooze_action_id=NULL WHERE snooze_action_id=?`, id); err != nil {
			return err
		}
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

// SnoozeAction sets or clears snoozeUntil ("" clears). Either way it clears
// the other half of the snooze: an action waits on one thing, so naming a date
// is also saying it is no longer waiting on a sibling, and clearing the snooze
// means it is not waiting at all.
func (a *App) SnoozeAction(id int64, until string) error {
	if until != "" && !ValidDate(until) {
		return fmt.Errorf("bad date %q", until)
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openActionTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE actions SET snooze_until=?, snooze_action_id=NULL WHERE id=?`, until, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "action", id, before)
	})
}

// Detach makes a project action standalone. Everything carries over except
// what the project was holding: a snooze on a sibling is an ordering inside a
// plan this action is leaving, so it goes, in both directions — what this one
// waited on, and what was waiting on it. Neither is a sibling any more, and a
// link pointing out of the project would be a plan that reads wrong from both
// ends (design.md, "Reshaping items").
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
	if _, err := tx.Exec(`UPDATE actions SET project_id=NULL, became_next_at=?, snooze_action_id=NULL
		WHERE id=?`, tsPtr(next), id); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE actions SET snooze_action_id=NULL WHERE snooze_action_id=?`, id); err != nil {
		return err
	}
	return a.audit(tx, EvDetached, "action", id, before)
}

// checkBlockerTx refuses a snooze that names an action it must not. The
// notation layer has already turned a title or an id into one of this
// project's open actions; what is left is the part that needs the graph.
//
// A cycle is the one that matters. Every other rule in the app tolerates a
// contradiction and shouts about it — a project may be stalled, a project may
// lose its definition of done — but a ring of actions each waiting on the next
// is different in kind: nothing in it can ever be started, the project holds
// open actions so it does not read as stalled, and the whole thing goes quiet.
// That is the silent death the stalled check exists to catch, so it is refused
// at the moment it would be written instead (design.md, "Time fields").
//
// Refusing it is also what makes the rest cheap: with no cycles the waiting
// graph is a forest, so every chain ends at an action waiting on nothing,
// which is what guarantees a project with open actions always has one that can
// be started. ActionTree leans on the same fact.
//
// self is 0 for an action being created, which nothing can be waiting on yet.
func (a *App) checkBlockerTx(tx *sql.Tx, self, projectID, blocker int64) error {
	if blocker == 0 {
		return nil
	}
	if blocker == self {
		return errSnoozeSelf
	}
	if projectID == 0 {
		return errors.New("a standalone action waits on a date, not on another action — it has no plan to be part of")
	}
	var bProject sql.NullInt64
	var completed sql.NullString
	var title string
	if err := tx.QueryRow(`SELECT project_id, title, completed_at FROM actions WHERE id=?`, blocker).
		Scan(&bProject, &title, &completed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no action #%d to wait on", blocker)
		}
		return err
	}
	if bProject.Int64 != projectID {
		return fmt.Errorf("%q is in another project — an action waits on one of its own siblings", title)
	}
	if completed.Valid {
		return fmt.Errorf("%q is already done, so waiting on it would be waiting on nothing", title)
	}
	// walk up from the blocker: if the chain reaches this action, the link
	// about to be written would close a ring. The forest invariant bounds the
	// walk, and the counter is there for a database that somehow lost it.
	at := blocker
	for i := 0; at != 0 && i <= 1000; i++ {
		if at == self {
			return fmt.Errorf("%q is already waiting on this one, directly or further down the chain", title)
		}
		var nextUp sql.NullInt64
		if err := tx.QueryRow(`SELECT snooze_action_id FROM actions WHERE id=?`, at).Scan(&nextUp); err != nil {
			return err
		}
		at = nextUp.Int64
	}
	return nil
}

func nullID(id int64) any {
	if id == 0 {
		return nil
	}
	return id
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
