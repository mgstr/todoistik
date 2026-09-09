package app

import (
	"database/sql"
	"errors"
	"strings"
)

// --- someday/maybe items -------------------------------------------------

// SomedayFields is a someday/maybe item as a form gives it: the idea, and the
// area it belongs to. One struct, so that filing an item and editing one later
// cannot drift into two answers to the same two questions (design.md,
// "Someday/maybe item").
type SomedayFields struct {
	Text string
	Tags []string
}

func (f *SomedayFields) validate() error {
	f.Text = strings.TrimSpace(f.Text)
	if f.Text == "" {
		return ErrEmpty
	}
	f.Tags = normTags(f.Tags)
	return nil
}

func (a *App) somedayTx(tx *sql.Tx, id int64) (*SomedayItem, error) {
	it := &SomedayItem{}
	var created, reviewed string
	err := tx.QueryRow(`SELECT id, text, created_at, last_reviewed_at FROM someday_items WHERE id=?`, id).
		Scan(&it.ID, &it.Text, &created, &reviewed)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = parseTS(created)
	it.LastReviewedAt = parseTS(reviewed)
	it.Tags, err = tagsForTx(tx, "someday", id)
	if err != nil {
		return nil, err
	}
	return it, nil
}

func (a *App) SomedayItem(id int64) (*SomedayItem, error) {
	var it *SomedayItem
	err := a.tx(func(tx *sql.Tx) error {
		var err error
		it, err = a.somedayTx(tx, id)
		return err
	})
	return it, err
}

// SomedayItems returns the someday/maybe view, oldest first (the age shown
// is the age of the idea). It filters by name and by tag, the two things a
// someday item has to be narrowed by (design.md, "Someday/Maybe").
func (a *App) SomedayItems(f Filters) ([]*SomedayItem, error) {
	rows, err := a.db.Query(`SELECT id, text, created_at, last_reviewed_at FROM someday_items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var items []*SomedayItem
	for rows.Next() {
		it := &SomedayItem{}
		var created, reviewed string
		if err := rows.Scan(&it.ID, &it.Text, &created, &reviewed); err != nil {
			rows.Close()
			return nil, err
		}
		it.CreatedAt = parseTS(created)
		it.LastReviewedAt = parseTS(reviewed)
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// the tags are read after the rows are done with, the way the projects
	// query reads them: one connection serves this database, so a second
	// query while the first is still open has nothing to run on
	var out []*SomedayItem
	for _, it := range items {
		if it.Tags, err = a.tagsFor("someday", it.ID); err != nil {
			return nil, err
		}
		if matchName(f.Name, it.Text) && matchTags(f.Tags, it.Tags) {
			out = append(out, it)
		}
	}
	return out, nil
}

// EditSomeday updates the idea and its tags. The text may be reworded to
// formulate the idea more clearly and the tags may be changed while it stays
// unclarified: neither says what will be done about it, which is the
// clarifying a someday item is spared (design.md, "Someday/maybe item").
func (a *App) EditSomeday(id int64, f SomedayFields) error {
	if err := f.validate(); err != nil {
		return err
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.somedayTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE someday_items SET text=? WHERE id=?`, f.Text, id); err != nil {
			return err
		}
		if err := a.setTagsTx(tx, "someday", id, f.Tags); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "someday", id, before)
	})
}

// ReturnToInbox sends an idea back to the inbox, to be decided about now
// rather than later. The item is consumed and its text captured again, so it
// is one undecided thing in the one place undecided things live and has to be
// answered like any other (design.md, "Reshaping items").
//
// The tags ride along written into the text — "Restore the bicycle #hobby" —
// because an inbox item has no tags of its own and the area is a decision
// already made about this idea. Dropping it would make the trip back cost
// something, which is the one thing that would stop it being taken.
//
// A text that is already sitting in the inbox collapses into it, by the same
// rule every other way in obeys (design.md, "Duplicate captures") — the idea
// is in the inbox either way, which is what was asked for.
func (a *App) ReturnToInbox(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		it, err := a.removeSomedayTx(tx, id, EvReturned)
		if err != nil {
			return err
		}
		text := it.Text
		for _, t := range it.Tags {
			text += " #" + t
		}
		_, _, err = a.captureTx(tx, text)
		return err
	})
}

func (a *App) removeSomedayTx(tx *sql.Tx, id int64, event string) (*SomedayItem, error) {
	before, err := a.somedayTx(tx, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM someday_items WHERE id=?`, id); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type='someday' AND item_id=?`, id); err != nil {
		return nil, err
	}
	return before, a.audit(tx, event, "someday", id, before)
}

// --- Inbox Zero branches -------------------------------------------------
// Each branch consumes the inbox item and produces the decided outcome in one
// transaction. The inbox is the only source: an idea that has become worth
// moving on goes back to the inbox first (design.md, "Reshaping items"), so
// that every decision is made in one place, on one kind of item.

func (a *App) consumeSource(tx *sql.Tx, id int64, event string) error {
	_, err := a.removeInboxItem(tx, id, event)
	return err
}

// ProcessTrash: the item is deleted, recorded in the audit log.
func (a *App) ProcessTrash(id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.consumeSource(tx, id, EvTrashed) })
}

// ProcessReference: sent out of the app to wherever reference material is
// kept; the app stores none. The audit entry is the record it existed.
func (a *App) ProcessReference(id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.consumeSource(tx, id, EvReference) })
}

// ProcessTwoMinute: done right now, under two minutes — completed in the
// audit log without ever becoming an action.
func (a *App) ProcessTwoMinute(id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.consumeSource(tx, id, EvTwoMinute) })
}

// ProcessAction: the item becomes an action, standalone when projectID is 0
// and filed under that project otherwise (design.md, "Inbox Zero", the Action
// branch). Deciding it is worth doing is what makes it next, so it is stamped
// as a next action either way unless it is deliberately parked — which only
// an action inside a project can be.
//
// A stalled or a snoozed project is a valid target: filing a next action into
// a stalled project is exactly what resolves the stall, and a project's snooze
// is about not being bugged, not about being closed to new work. A completed
// one is not — it is finished, and reopening it is not a decision to make
// while emptying the inbox.
func (a *App) ProcessAction(id int64, f ActionFields, projectID int64, parked bool) (*Action, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	if parked && projectID == 0 {
		return nil, errors.New("only an action inside a project can be parked")
	}
	if projectID != 0 {
		p, err := a.Project(projectID)
		if err != nil {
			return nil, err
		}
		if p.CompletedAt != nil {
			return nil, errors.New("that project is completed; it can not take a new action")
		}
	}
	var act *Action
	err := a.tx(func(tx *sql.Tx) error {
		if err := a.consumeSource(tx, id, EvDeleted); err != nil {
			return err
		}
		now := a.now().UTC()
		act = &Action{
			ProjectID: projectID,
			Title:     f.Title, Context: f.Context, ContextParam: f.ContextParam,
			Duration: f.Duration, NeedsFocus: f.NeedsFocus,
			Description: f.Description, AssignedTo: strings.TrimSpace(f.AssignedTo),
			DueDate: f.DueDate, SnoozeUntil: f.SnoozeUntil, Tags: normTags(f.Tags),
			CreatedAt: now, LastReviewedAt: now,
		}
		if !parked {
			act.BecameNextAt = &now
		}
		return a.insertActionTx(tx, act)
	})
	if err != nil {
		return nil, err
	}
	return act, nil
}

// ProcessProject: the item becomes a project (title, DOD and at least one
// action required — the Inbox Zero project branch).
func (a *App) ProcessProject(id int64, f ProjectFields, actions []ActionFields) (*Project, error) {
	if err := a.tx(func(tx *sql.Tx) error {
		return a.consumeSource(tx, id, EvDeleted)
	}); err != nil {
		return nil, err
	}
	return a.CreateProject(f, actions)
}

// ProcessSomeday: worth looking at some time, but not now. The idea arrives
// written the way it will be read a month from now: reworded if it needed it,
// and tagged with the area it belongs to, which is the decision this branch
// was already making (design.md, "Inbox Zero").
func (a *App) ProcessSomeday(id int64, f SomedayFields) (*SomedayItem, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	var it *SomedayItem
	err := a.tx(func(tx *sql.Tx) error {
		if err := a.consumeSource(tx, id, EvDeleted); err != nil {
			return err
		}
		now := a.now().UTC()
		it = &SomedayItem{Text: f.Text, Tags: f.Tags, CreatedAt: now, LastReviewedAt: now}
		res, err := tx.Exec(`INSERT INTO someday_items (text, created_at, last_reviewed_at) VALUES (?,?,?)`,
			it.Text, ts(it.CreatedAt), ts(it.LastReviewedAt))
		if err != nil {
			return err
		}
		it.ID, _ = res.LastInsertId()
		if err := a.setTagsTx(tx, "someday", it.ID, it.Tags); err != nil {
			return err
		}
		return a.audit(tx, EvCreated, "someday", it.ID, it)
	})
	if err != nil {
		return nil, err
	}
	return it, nil
}
