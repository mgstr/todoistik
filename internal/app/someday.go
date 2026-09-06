package app

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// --- someday/maybe items -------------------------------------------------

func (a *App) somedayTx(tx *sql.Tx, id int64) (*SomedayItem, error) {
	it := &SomedayItem{}
	var created, reviewed string
	err := tx.QueryRow(`SELECT id, text, created_at, last_reviewed_at, snooze_until FROM someday_items WHERE id=?`, id).
		Scan(&it.ID, &it.Text, &created, &reviewed, &it.SnoozeUntil)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = parseTS(created)
	it.LastReviewedAt = parseTS(reviewed)
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
// is the age of the idea). Snoozed items are included, marked by the caller.
func (a *App) SomedayItems(nameFilter string) ([]*SomedayItem, error) {
	rows, err := a.db.Query(`SELECT id, text, created_at, last_reviewed_at, snooze_until FROM someday_items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*SomedayItem
	for rows.Next() {
		it := &SomedayItem{}
		var created, reviewed string
		if err := rows.Scan(&it.ID, &it.Text, &created, &reviewed, &it.SnoozeUntil); err != nil {
			return nil, err
		}
		it.CreatedAt = parseTS(created)
		it.LastReviewedAt = parseTS(reviewed)
		if matchName(nameFilter, it.Text) {
			out = append(out, it)
		}
	}
	return out, rows.Err()
}

// EditSomeday updates the idea's text and snooze; the text may be edited to
// formulate the idea more clearly while it stays raw.
func (a *App) EditSomeday(id int64, text, snoozeUntil string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrEmpty
	}
	if snoozeUntil != "" && !ValidDate(snoozeUntil) {
		return fmt.Errorf("bad snooze date %q", snoozeUntil)
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.somedayTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE someday_items SET text=?, snooze_until=? WHERE id=?`, text, snoozeUntil, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "someday", id, before)
	})
}

// TrashSomeday deletes a someday/maybe item (audited, recoverable).
func (a *App) TrashSomeday(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		_, err := a.removeSomedayTx(tx, id, EvTrashed)
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
	return before, a.audit(tx, event, "someday", id, before)
}

// --- Inbox Zero branches -------------------------------------------------
// Each branch consumes the source item (inbox or someday) and produces the
// decided outcome in one transaction. src is "inbox" or "someday".

func (a *App) consumeSource(tx *sql.Tx, src string, id int64, event string) error {
	switch src {
	case "inbox":
		_, err := a.removeInboxItem(tx, id, event)
		return err
	case "someday":
		_, err := a.removeSomedayTx(tx, id, event)
		return err
	default:
		return fmt.Errorf("unknown source %q", src)
	}
}

// ProcessTrash: the item is deleted, recorded in the audit log.
func (a *App) ProcessTrash(src string, id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.consumeSource(tx, src, id, EvTrashed) })
}

// ProcessReference: sent out of the app to wherever reference material is
// kept; the app stores none. The audit entry is the record it existed.
func (a *App) ProcessReference(src string, id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.consumeSource(tx, src, id, EvReference) })
}

// ProcessTwoMinute: done right now, under two minutes — completed in the
// audit log without ever becoming an action.
func (a *App) ProcessTwoMinute(src string, id int64) error {
	return a.tx(func(tx *sql.Tx) error { return a.consumeSource(tx, src, id, EvTwoMinute) })
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
func (a *App) ProcessAction(src string, id int64, f ActionFields, projectID int64, parked bool) (*Action, error) {
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
		if err := a.consumeSource(tx, src, id, EvDeleted); err != nil {
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
func (a *App) ProcessProject(src string, id int64, f ProjectFields, actions []ActionFields) (*Project, error) {
	if err := a.tx(func(tx *sql.Tx) error {
		return a.consumeSource(tx, src, id, EvDeleted)
	}); err != nil {
		return nil, err
	}
	return a.CreateProject(f, actions)
}

// ProcessSomeday: worth looking at some time, but not now. From the inbox
// only — a someday item deciding to stay is KeepIncubating instead.
func (a *App) ProcessSomeday(id int64, text, snoozeUntil string) (*SomedayItem, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrEmpty
	}
	if snoozeUntil != "" && !ValidDate(snoozeUntil) {
		return nil, fmt.Errorf("bad snooze date %q", snoozeUntil)
	}
	var it *SomedayItem
	err := a.tx(func(tx *sql.Tx) error {
		if err := a.consumeSource(tx, "inbox", id, EvDeleted); err != nil {
			return err
		}
		now := a.now().UTC()
		it = &SomedayItem{Text: text, CreatedAt: now, LastReviewedAt: now, SnoozeUntil: snoozeUntil}
		res, err := tx.Exec(`INSERT INTO someday_items (text, created_at, last_reviewed_at, snooze_until) VALUES (?,?,?,?)`,
			it.Text, ts(it.CreatedAt), ts(it.LastReviewedAt), it.SnoozeUntil)
		if err != nil {
			return err
		}
		it.ID, _ = res.LastInsertId()
		return a.audit(tx, EvCreated, "someday", it.ID, it)
	})
	if err != nil {
		return nil, err
	}
	return it, nil
}

// KeepIncubating: still interesting, still not now — a new snooze, nothing
// else changes. Only when processing a someday/maybe item.
func (a *App) KeepIncubating(id int64, snoozeUntil string) error {
	if snoozeUntil != "" && !ValidDate(snoozeUntil) {
		return fmt.Errorf("bad snooze date %q", snoozeUntil)
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.somedayTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE someday_items SET snooze_until=? WHERE id=?`, snoozeUntil, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "someday", id, before)
	})
}
