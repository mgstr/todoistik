package app

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrOpenActions = errors.New("the project still has open actions; resolve each one first (complete, delete or detach)")
	ErrNoDOD       = errors.New("a project with no definition of done can not be completed")
)

type ProjectFields struct {
	Title       string
	DOD         string
	Description string
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
		Title: f.Title, DOD: f.DOD, Description: f.Description,
		Tags: normTags(f.Tags), SnoozeUntil: f.SnoozeUntil,
		CreatedAt: now, LastReviewedAt: now,
	}
	err := a.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO projects (title, dod, description, created_at, last_reviewed_at, snooze_until)
			VALUES (?,?,?,?,?,?)`,
			p.Title, p.DOD, p.Description, ts(p.CreatedAt), ts(p.LastReviewedAt), p.SnoozeUntil)
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
	err := tx.QueryRow(`SELECT id, title, dod, description, created_at, last_reviewed_at, snooze_until, completed_at
		FROM projects WHERE id=?`, id).
		Scan(&p.ID, &p.Title, &p.DOD, &p.Description, &created, &reviewed, &p.SnoozeUntil, &completed)
	if err != nil {
		return nil, err
	}
	p.CreatedAt = parseTS(created)
	p.LastReviewedAt = parseTS(reviewed)
	p.CompletedAt = parseTSPtr(completed)
	var terr error
	p.Tags, terr = tagsForTx(tx, "project", id)
	return p, terr
}

// Project loads one project with all its actions, stalled state derived.
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
	rows, err := tx.Query(`SELECT `+actionCols+` FROM actions a WHERE a.project_id=? ORDER BY a.completed_at IS NOT NULL, a.id`, projectID)
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
		before, err := a.projectRowTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE projects SET title=?, dod=?, description=?, snooze_until=? WHERE id=?`,
			f.Title, f.DOD, f.Description, f.SnoozeUntil, id); err != nil {
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
		before, err := a.projectRowTx(tx, id)
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
		before, err := a.projectRowTx(tx, id)
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
		src, err = a.actionTx(tx, actionID)
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

// ProjectStateAfterComplete is what the completion flow needs to decide what
// to ask (design.md, "Completing a next action").
type ProjectStateAfterComplete struct {
	Project     *Project  `json:"project"`
	OpenActions []*Action `json:"openActions"`
	HasNext     bool      `json:"hasNext"`
}

// ProjectState reports the project's situation for the completion prompt.
func (a *App) ProjectState(projectID int64) (*ProjectStateAfterComplete, error) {
	p, err := a.Project(projectID)
	if err != nil {
		return nil, err
	}
	st := &ProjectStateAfterComplete{Project: p, OpenActions: p.OpenActions()}
	for _, act := range st.OpenActions {
		if act.IsNext() {
			st.HasNext = true
		}
	}
	return st, nil
}
