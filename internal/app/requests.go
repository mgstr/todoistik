package app

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"todoistik/internal/request"
)

// A completion request is a captured line saying that an item was finished
// somewhere else (design.md, "Completion requests"). The grammar of the line
// is in internal/request, which the reporting program shares; what one *means*
// is here, because it means an action, and only this package knows whether
// that action is still there.
//
// Nothing about a request is stored. It is an inbox item like any other until
// it is answered, and answering it either completes the action it names or
// throws the line away — there is no third state to keep, which is what design
// .md means by "not done" being a state rather than a decision.

// RequestVerdict is what a request turns out to be about, and it decides what
// the screen may offer. Only Open has two answers; the rest have one, so a
// button that cannot be pressed honestly is never shown.
type RequestVerdict string

const (
	RequestOpen     RequestVerdict = "open"     // the action is there, and open
	RequestDone     RequestVerdict = "done"     // completed here already
	RequestPromoted RequestVerdict = "promoted" // it became a project; the identity ended there
	RequestGone     RequestVerdict = "gone"     // no such action, and nothing says why
)

// CompletionRequest is one request, read and resolved: what the line said, and
// what the app finds at the other end of it.
type CompletionRequest struct {
	Source  string         `json:"source"`
	Name    string         `json:"name"` // what the item was called over there
	At      time.Time      `json:"at"`   // when it was finished over there
	Verdict RequestVerdict `json:"verdict"`
	Action  *Action        `json:"action,omitempty"` // for Open and Done
}

// ReadCompletionRequest reads a captured line as a request and resolves what
// it is about, or reports that the line is not a request at all.
//
// The action is looked up by id, which needs no view: the request may well have
// been filed by something that could only see one of them (implementation.md,
// "The completion channel").
func (a *App) ReadCompletionRequest(text string) (*CompletionRequest, bool, error) {
	line, ok := request.Parse(text)
	if !ok {
		return nil, false, nil
	}
	req := &CompletionRequest{
		Source: line.Source,
		Name:   line.Name,
		At:     line.In(a.loc),
	}
	act, err := a.Action(line.ItemID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		verdict, verr := a.disappeared(line.ItemID)
		if verr != nil {
			return nil, true, verr
		}
		req.Verdict = verdict
	case err != nil:
		return nil, true, err
	case reused(act, req.At):
		verdict, verr := a.disappeared(line.ItemID)
		if verr != nil {
			return nil, true, verr
		}
		req.Verdict = verdict
	case act.CompletedAt != nil:
		req.Verdict, req.Action = RequestDone, act
	default:
		req.Verdict, req.Action = RequestOpen, act
	}
	return req, true, nil
}

// reused reports that the item wearing this id is not the item the request is
// about, and is why an item that exists can still be the wrong one.
//
// An id is the row's own, handed out as one past the highest in the table, and
// nothing reserves the ones given back: delete the newest action, or promote it
// into a project, and the next action created takes the same number. A request
// filed before that swap would otherwise name an action with nothing to do with
// the work that was done.
//
// The creation date settles it exactly, with nothing to store: the item a
// request is about existed before the work on it was finished, so an item
// created after that moment is a different item wearing the same id. The only
// way to fool it is to tick a reminder in the same minute its action was
// created, which is not a sequence that happens.
func reused(act *Action, at time.Time) bool { return act.CreatedAt.After(at) }

// disappeared says why an id names nothing usable: promoted is the one
// disappearance that is not a loss and looks exactly like one, and the audit
// log is the only record that tells them apart (design.md, "Audit entry").
func (a *App) disappeared(actionID int64) (RequestVerdict, error) {
	promoted, err := a.wasPromoted(actionID)
	if err != nil {
		return RequestGone, err
	}
	if promoted {
		return RequestPromoted, nil
	}
	return RequestGone, nil
}

// wasPromoted answers whether the action that is gone left by being promoted
// into a project. The audit log is the only record of it: promotion deletes the
// action, and its entry is what makes the difference recoverable at all
// (design.md, "Audit entry").
func (a *App) wasPromoted(actionID int64) (bool, error) {
	var n int
	err := a.db.QueryRow(
		`SELECT count(*) FROM audit_log WHERE event = ? AND item_type = 'action' AND item_id = ?`,
		EvPromoted, actionID).Scan(&n)
	return n > 0, err
}

// ErrRequestNotOpen is the answer to confirming a request whose action is no
// longer there to complete. The screen never offers the button in that case,
// so this is for the answer that arrives anyway — a second tab, a back button,
// a form posted after the action was completed at the desk.
var ErrRequestNotOpen = errors.New("that item can no longer be completed from this request")

// ProcessCompletion answers a completion request with yes: the action it names
// is completed, stamped with the moment the work was actually finished, and the
// request leaves the inbox.
//
// The line is re-read here rather than trusted from the caller, so that the
// only action this can ever complete is the one the captured line names.
func (a *App) ProcessCompletion(inboxID int64) (*Action, error) {
	var act *Action
	err := a.tx(func(tx *sql.Tx) error {
		it, err := a.inboxItemTx(tx, inboxID)
		if err != nil {
			return err
		}
		line, ok := request.Parse(it.Text)
		if !ok {
			return fmt.Errorf("%w: that inbox item is not a completion request", ErrRequestNotOpen)
		}
		before, err := a.actionTx(tx, line.ItemID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrRequestNotOpen
			}
			return err
		}
		if before.CompletedAt != nil {
			return ErrRequestNotOpen
		}
		if reused(before, line.In(a.loc)) {
			return ErrRequestNotOpen
		}
		// the ordinary completion, at the moment named: one path, one audit
		// entry, everything that hangs off completing an action unchanged
		if err := a.completeActionTx(tx, line.ItemID, line.In(a.loc)); err != nil {
			return err
		}
		if _, err := a.removeInboxItem(tx, inboxID, EvConfirmed); err != nil {
			return err
		}
		act, err = a.actionTx(tx, line.ItemID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return act, nil
}
