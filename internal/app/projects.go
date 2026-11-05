package app

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	ErrOpenActions = errors.New("the project still has open actions")
	ErrNoDOD       = errors.New("a project with no definition of done can not be completed")
)

type ProjectFields struct {
	Title       string
	DOD         string
	SnoozeUntil string
	Tags        []string
}

// CreateProject creates a project with its initial actions. At creation the
// DOD and at least one action are required — Inbox Zero is where those
// answers are cheapest (design.md, "Editing items").
func (a *App) CreateProject(f ProjectFields, actions []ActionFields) (*Project, error) {
	f.Title = strings.TrimSpace(f.Title)
	if f.Title == "" {
		return nil, ErrTitle
	}
	if strings.TrimSpace(f.DOD) == "" {
		return nil, errors.New("a definition of done is required")
	}
	if len(actions) == 0 {
		return nil, errors.New("at least one action is required, which becomes the next action")
	}
	if f.SnoozeUntil != "" && !ValidDate(f.SnoozeUntil) {
		return nil, fmt.Errorf("bad snooze date %q", f.SnoozeUntil)
	}
	now := a.now().UTC()
	p := &Project{
		Title: f.Title, DOD: f.DOD,
		Tags: normTags(f.Tags), SnoozeUntil: f.SnoozeUntil,
		CreatedAt: now, LastReviewedAt: now,
	}
	err := a.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO projects (title, dod, created_at, last_reviewed_at, snooze_until)
			VALUES (?,?,?,?,?)`,
			p.Title, p.DOD, ts(p.CreatedAt), ts(p.LastReviewedAt), p.SnoozeUntil)
		if err != nil {
			return err
		}
		p.ID, _ = res.LastInsertId()
		if err := a.setTagsTx(tx, "project", p.ID, p.Tags); err != nil {
			return err
		}
		if err := a.audit(tx, EvCreated, "project", p.ID, p); err != nil {
			return err
		}
		for _, af := range actions {
			if err := af.validate(); err != nil {
				return err
			}
			act := &Action{
				ProjectID: p.ID, Title: af.Title,
				Context: af.Context, ContextParam: af.ContextParam,
				Duration: af.Duration, NeedsFocus: af.NeedsFocus,
				Description: af.Description, AssignedTo: strings.TrimSpace(af.AssignedTo),
				DueDate: af.DueDate, SnoozeUntil: af.SnoozeUntil, Tags: normTags(af.Tags),
				CreatedAt: now, LastReviewedAt: now, BecameNextAt: &now,
			}
			if err := a.insertActionTx(tx, act); err != nil {
				return err
			}
			p.Actions = append(p.Actions, act)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (a *App) projectRowTx(tx *sql.Tx, id int64) (*Project, error) {
	p := &Project{}
	var created, reviewed string
	var completed sql.NullString
	var nextID sql.NullInt64
	err := tx.QueryRow(`SELECT id, title, dod, created_at, last_reviewed_at, snooze_until,
		next_action_id, completed_at FROM projects WHERE id=?`, id).
		Scan(&p.ID, &p.Title, &p.DOD, &created, &reviewed, &p.SnoozeUntil, &nextID, &completed)
	if err != nil {
		return nil, err
	}
	p.NextActionID = nextID.Int64
	p.CreatedAt = parseTS(created)
	p.LastReviewedAt = parseTS(reviewed)
	p.CompletedAt = parseTSPtr(completed)
	var terr error
	p.Tags, terr = tagsForTx(tx, "project", id)
	return p, terr
}

// Project loads one project with all its actions, stalled state derived.
// openProjectRowTx loads a project that is about to be written to, and refuses
// if it is completed — the freeze, for projects, in one place (see
// ErrCompleted).
func (a *App) openProjectRowTx(tx *sql.Tx, id int64) (*Project, error) {
	p, err := a.projectRowTx(tx, id)
	if err != nil {
		return nil, err
	}
	if p.CompletedAt != nil {
		return nil, ErrCompleted
	}
	return p, nil
}

func (a *App) Project(id int64) (*Project, error) {
	var p *Project
	err := a.tx(func(tx *sql.Tx) error {
		var err error
		p, err = a.projectRowTx(tx, id)
		if err != nil {
			return err
		}
		p.Actions, err = a.projectActionsTx(tx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	p.Stalled = p.ComputeStalled(a.Today())
	return p, nil
}

func (a *App) projectActionsTx(tx *sql.Tx, projectID int64) ([]*Action, error) {
	rows, err := tx.Query(`SELECT `+actionCols+` FROM actions a WHERE a.project_id=? ORDER BY a.completed_at IS NOT NULL, a.position, a.id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Action
	for rows.Next() {
		act, err := scanAction(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, act)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, act := range out {
		if act.Tags, err = tagsForTx(tx, "action", act.ID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// UpdateProject edits title, DOD, description, snooze and tags. Clearing the
// DOD is never blocked — it puts the project in an error state instead.
func (a *App) UpdateProject(id int64, f ProjectFields) error {
	f.Title = strings.TrimSpace(f.Title)
	if f.Title == "" {
		return ErrTitle
	}
	if f.SnoozeUntil != "" && !ValidDate(f.SnoozeUntil) {
		return fmt.Errorf("bad snooze date %q", f.SnoozeUntil)
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openProjectRowTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE projects SET title=?, dod=?, snooze_until=? WHERE id=?`,
			f.Title, f.DOD, f.SnoozeUntil, id); err != nil {
			return err
		}
		if err := a.setTagsTx(tx, "project", id, normTags(f.Tags)); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "project", id, before)
	})
}

// CompleteProject completes a project: only with no open actions and a DOD —
// completing is the moment the DOD is confirmed met (design.md, "Completion").
func (a *App) CompleteProject(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openProjectRowTx(tx, id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(before.DOD) == "" {
			return ErrNoDOD
		}
		var open int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM actions WHERE project_id=? AND completed_at IS NULL`, id).Scan(&open); err != nil {
			return err
		}
		if open > 0 {
			return ErrOpenActions
		}
		if _, err := tx.Exec(`UPDATE projects SET completed_at=? WHERE id=?`, ts(a.now()), id); err != nil {
			return err
		}
		return a.audit(tx, EvCompleted, "project", id, before)
	})
}

// UncompleteProject clears completedAt, bringing the project back.
func (a *App) UncompleteProject(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.projectRowTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE projects SET completed_at=NULL WHERE id=?`, id); err != nil {
			return err
		}
		return a.audit(tx, EvUncompleted, "project", id, before)
	})
}

// DeleteProject deletes a project. Like completion it requires every open
// action resolved first; completed actions are kept in the audit snapshot.
func (a *App) DeleteProject(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.openProjectRowTx(tx, id)
		if err != nil {
			return err
		}
		var open int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM actions WHERE project_id=? AND completed_at IS NULL`, id).Scan(&open); err != nil {
			return err
		}
		if open > 0 {
			return ErrOpenActions
		}
		before.Actions, err = a.projectActionsTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type='action' AND item_id IN (SELECT id FROM actions WHERE project_id=?)`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM actions WHERE project_id=?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type='project' AND item_id=?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM projects WHERE id=?`, id); err != nil {
			return err
		}
		return a.audit(tx, EvDeleted, "project", id, before)
	})
}

// Promote turns a standalone action into a project, running the same
// validations as the Inbox Zero project branch, fields prefilled by the
// caller from the action. The source action is consumed (audited).
func (a *App) Promote(actionID int64, f ProjectFields, actions []ActionFields) (*Project, error) {
	var src *Action
	err := a.tx(func(tx *sql.Tx) error {
		var err error
		src, err = a.openActionTx(tx, actionID)
		if err != nil {
			return err
		}
		if src.ProjectID != 0 {
			return errors.New("detach the action from its project first, then promote")
		}
		if _, err := tx.Exec(`DELETE FROM actions WHERE id=?`, actionID); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type='action' AND item_id=?`, actionID); err != nil {
			return err
		}
		return a.audit(tx, EvPromoted, "action", actionID, src)
	})
	if err != nil {
		return nil, err
	}
	return a.CreateProject(f, actions)
}

// IsFinishable reports a project with nothing open left: the one state in which
// completing it is accepted, and the one thing the completion flow has to ask
// about after an action is ticked off (design.md, "Completing a next action").
//
// It is the question `CompleteProject` answers `ErrOpenActions` to, asked before
// the press rather than after — so a screen can leave the Done off instead of
// offering one the app refuses. It is deliberately not "has no next action":
// those came apart when a project stopped counting every open action as one, and
// a project waiting on a snooze has no next action and is not finishable either.
func (a *App) IsFinishable(projectID int64) (bool, error) {
	p, err := a.Project(projectID)
	if err != nil {
		return false, err
	}
	return len(p.OpenActions()) == 0, nil
}

// MakeNext points the project at one of its actions: this is the one that is
// next, whatever the plan's order would otherwise have said (design.md,
// "Project").
//
// It writes the pointing and nothing else. The previous next action loses
// nothing — it is not restamped, not reordered and not moved in the plan —
// because being next is the project's claim about which step is on you now and
// not a property of the action it lands on. Which is also why there is no
// unpointing: a project with a workable action always has a next one, and
// changing your mind is pointing somewhere else.
func (a *App) MakeNext(actionID int64) error {
	return a.tx(func(tx *sql.Tx) error {
		act, err := a.openActionTx(tx, actionID)
		if err != nil {
			return err
		}
		if act.ProjectID == 0 {
			return errors.New("a task has no project to be the next action of")
		}
		before, err := a.openProjectRowTx(tx, act.ProjectID)
		if err != nil {
			return err
		}
		// the same rule the derivation reads, so the app can never be pointed
		// at an action the page would refuse to show: a snoozed action is not
		// workable yet, and "next" said about it would be a claim the snooze
		// contradicts (design.md, "Time fields")
		if !canBeNext(act, a.Today()) {
			return errors.New("a snoozed action can not be the next action; wake it first")
		}
		if _, err := tx.Exec(`UPDATE projects SET next_action_id=? WHERE id=?`, actionID, act.ProjectID); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "project", act.ProjectID, before)
	})
}

// ErrStalePlan: the order asked for is not an order of this project's open
// actions. The page it was dragged on was drawn before something changed — an
// action added, completed or deleted somewhere else — and writing the order
// anyway would put rows nobody saw into places nobody chose.
var ErrStalePlan = errors.New("the plan has changed since this page was drawn — open the project again")

// MoveAction moves an open action one place up (delta -1) or down (delta 1)
// among its project's open actions: the key that carries the row with the
// cursor (design.md, "The order of a plan is yours").
//
// Completed actions are stepped over, because they are not in the plan any
// more — they are in the record under it, ordered by when they were done — and
// a move that swapped places with one would be a press that changed nothing on
// the screen. At either end of the plan the move is not an error but nothing:
// there is nowhere to go, and the key says so by not being offered.
func (a *App) MoveAction(actionID int64, delta int) error {
	return a.tx(func(tx *sql.Tx) error {
		act, err := a.openActionTx(tx, actionID)
		if err != nil {
			return err
		}
		if act.ProjectID == 0 {
			return errors.New("a task is in no plan to be moved in")
		}
		before, err := a.planSnapshotTx(tx, act.ProjectID)
		if err != nil {
			return err
		}
		plan := before.OpenActions()
		at := -1
		for i, o := range plan {
			if o.ID == actionID {
				at = i
			}
		}
		to := at + delta
		if at < 0 || to < 0 || to >= len(plan) {
			return nil
		}
		order := make([]int64, len(plan))
		for i, o := range plan {
			order[i] = o.ID
		}
		order[at], order[to] = order[to], order[at]
		return a.writeOrderTx(tx, before, order)
	})
}

// ReorderActions puts a project's open actions in the order given, which has
// to name every one of them exactly once: it is what a drag hands back, the
// whole list as it now stands. Anything else is a page drawn before the plan
// changed, and is refused rather than half applied.
func (a *App) ReorderActions(projectID int64, order []int64) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.planSnapshotTx(tx, projectID)
		if err != nil {
			return err
		}
		plan := before.OpenActions()
		if len(order) != len(plan) {
			return ErrStalePlan
		}
		have := map[int64]bool{}
		for _, o := range plan {
			have[o.ID] = true
		}
		for _, id := range order {
			if !have[id] {
				return ErrStalePlan
			}
			delete(have, id)
		}
		return a.writeOrderTx(tx, before, order)
	})
}

// planSnapshotTx is the open project with its actions, which is both what a
// reorder works from and what its audit entry keeps: the order as it was.
func (a *App) planSnapshotTx(tx *sql.Tx, projectID int64) (*Project, error) {
	p, err := a.openProjectRowTx(tx, projectID)
	if err != nil {
		return nil, err
	}
	if p.Actions, err = a.projectActionsTx(tx, projectID); err != nil {
		return nil, err
	}
	return p, nil
}

// writeOrderTx gives the open actions named in order the positions they
// already hold between them, smallest first. Reusing the same numbers rather
// than counting from 1 leaves every completed action exactly where it was,
// which matters for the one way back into the plan: an action brought back
// from done returns to the place it was done from. An order that is the one
// already standing writes nothing and is not audited, so it is no undo step.
func (a *App) writeOrderTx(tx *sql.Tx, before *Project, order []int64) error {
	plan := before.OpenActions()
	slots := make([]int64, len(plan))
	for i, o := range plan {
		slots[i] = o.Position
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	// two rows can share a position only in a database written by hand; the
	// slots are spread apart so that the order written is the order read
	for i := 1; i < len(slots); i++ {
		if slots[i] <= slots[i-1] {
			slots[i] = slots[i-1] + 1
		}
	}
	changed := false
	for i, id := range order {
		if plan[i].ID != id || plan[i].Position != slots[i] {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	for i, id := range order {
		if _, err := tx.Exec(`UPDATE actions SET position=? WHERE id=?`, slots[i], id); err != nil {
			return err
		}
	}
	return a.audit(tx, EvReordered, "project", before.ID, before)
}
