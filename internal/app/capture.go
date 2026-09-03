package app

import (
	"database/sql"
	"errors"
	"strings"
)

var ErrEmpty = errors.New("empty text")

// Capture puts raw text into the inbox. Every way in goes through here:
// typed by hand, the capture API, a schedule firing. Returns accepted=false
// when the text exactly matches an item already sitting in the open inbox —
// the duplicate is dropped, unaudited, per design.md "Duplicate captures".
func (a *App) Capture(text string) (item *InboxItem, accepted bool, err error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, false, ErrEmpty
	}
	err = a.tx(func(tx *sql.Tx) error {
		var accepted2 bool
		item, accepted2, err = a.captureTx(tx, text)
		accepted = accepted2
		return err
	})
	return item, accepted, err
}

func (a *App) captureTx(tx *sql.Tx, text string) (*InboxItem, bool, error) {
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM inbox_items WHERE text = ?`, text).Scan(&n); err != nil {
		return nil, false, err
	}
	if n > 0 {
		return nil, false, nil
	}
	item := &InboxItem{Text: text, CreatedAt: a.now().UTC()}
	res, err := tx.Exec(`INSERT INTO inbox_items (text, created_at) VALUES (?,?)`, item.Text, ts(item.CreatedAt))
	if err != nil {
		return nil, false, err
	}
	item.ID, _ = res.LastInsertId()
	if err := a.audit(tx, EvCreated, "inbox", item.ID, item); err != nil {
		return nil, false, err
	}
	return item, true, nil
}

// Inbox returns the open inbox, oldest first. No filters, by design.
func (a *App) Inbox() ([]*InboxItem, error) {
	rows, err := a.db.Query(`SELECT id, text, created_at FROM inbox_items ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*InboxItem
	for rows.Next() {
		it := &InboxItem{}
		var at string
		if err := rows.Scan(&it.ID, &it.Text, &at); err != nil {
			return nil, err
		}
		it.CreatedAt = parseTS(at)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (a *App) inboxItemTx(tx *sql.Tx, id int64) (*InboxItem, error) {
	it := &InboxItem{}
	var at string
	err := tx.QueryRow(`SELECT id, text, created_at FROM inbox_items WHERE id = ?`, id).Scan(&it.ID, &it.Text, &at)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = parseTS(at)
	return it, nil
}

// InboxItem returns one open inbox item.
func (a *App) InboxItem(id int64) (*InboxItem, error) {
	it := &InboxItem{}
	var at string
	err := a.db.QueryRow(`SELECT id, text, created_at FROM inbox_items WHERE id = ?`, id).Scan(&it.ID, &it.Text, &at)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = parseTS(at)
	return it, nil
}

// EditInboxText edits a captured text in place (audited like any edit).
func (a *App) EditInboxText(id int64, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrEmpty
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.inboxItemTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE inbox_items SET text = ? WHERE id = ?`, text, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "inbox", id, before)
	})
}

// removeInboxItem deletes an inbox item as part of processing it. The audit
// event says what became of it; the snapshot is the item as captured.
func (a *App) removeInboxItem(tx *sql.Tx, id int64, event string) (*InboxItem, error) {
	it, err := a.inboxItemTx(tx, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(`DELETE FROM inbox_items WHERE id = ?`, id); err != nil {
		return nil, err
	}
	return it, a.audit(tx, event, "inbox", id, it)
}
