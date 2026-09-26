package app

import (
	"database/sql"
	"strings"
)

// --- reference material --------------------------------------------------
//
// The app keeps reference material now. It used to keep none: the branch sent
// the item out to wherever material was kept and the audit entry was the whole
// record it had ever existed, which made "not actionable, but worth keeping"
// the one answer of Inbox Zero that kept nothing (design.md, "Reference
// item").

// ReferenceFields is a reference item as a form gives it: the material, and
// the area it is about. One struct, for the reason SomedayFields is one —
// filing a thing and editing it later are the same two questions, and two
// answers to them is how they come to differ.
type ReferenceFields struct {
	Text string
	Tags []string
}

func (f *ReferenceFields) validate() error {
	f.Text = strings.TrimSpace(f.Text)
	if f.Text == "" {
		return ErrEmpty
	}
	f.Tags = normTags(f.Tags)
	return nil
}

func (a *App) referenceTx(tx *sql.Tx, id int64) (*ReferenceItem, error) {
	it := &ReferenceItem{}
	var created string
	err := tx.QueryRow(`SELECT id, text, created_at FROM reference_items WHERE id=?`, id).
		Scan(&it.ID, &it.Text, &created)
	if err != nil {
		return nil, err
	}
	it.CreatedAt = parseTS(created)
	it.Tags, err = tagsForTx(tx, "reference", id)
	if err != nil {
		return nil, err
	}
	return it, nil
}

func (a *App) ReferenceItem(id int64) (*ReferenceItem, error) {
	var it *ReferenceItem
	err := a.tx(func(tx *sql.Tx) error {
		var err error
		it, err = a.referenceTx(tx, id)
		return err
	})
	return it, err
}

// ReferenceItems returns the Reference view, newest first: the pile is read to
// find one thing in it, and what was filed last is what is most often looked
// for again (design.md, "Reference"). It filters by name and by tag, which is
// everything a reference item carries.
func (a *App) ReferenceItems(f Filters) ([]*ReferenceItem, error) {
	rows, err := a.db.Query(`SELECT id, text, created_at FROM reference_items ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	var items []*ReferenceItem
	for rows.Next() {
		it := &ReferenceItem{}
		var created string
		if err := rows.Scan(&it.ID, &it.Text, &created); err != nil {
			rows.Close()
			return nil, err
		}
		it.CreatedAt = parseTS(created)
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// the tags are read once the rows are done with, the way SomedayItems
	// reads them: one connection serves this database, so a second query
	// while the first is still open has nothing to run on
	var out []*ReferenceItem
	for _, it := range items {
		if it.Tags, err = a.tagsFor("reference", it.ID); err != nil {
			return nil, err
		}
		if matchName(f.Name, it.Text) && matchTags(f.Tags, it.Tags) {
			out = append(out, it)
		}
	}
	return out, nil
}

// EditReference updates the material and its tags. Both stay editable for the
// reason they do on a someday/maybe item, and more so: material is added to as
// it is found out — the account number today, the portal it is typed into next
// month — and a note that cannot be corrected is one you stop trusting
// (design.md, "Reference item").
func (a *App) EditReference(id int64, f ReferenceFields) error {
	if err := f.validate(); err != nil {
		return err
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.referenceTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE reference_items SET text=? WHERE id=?`, f.Text, id); err != nil {
			return err
		}
		if err := a.setTagsTx(tx, "reference", id, f.Tags); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "reference", id, before)
	})
}

// DeleteReference is the one way out of this pile, and there is nothing for it
// to go back to: material that turns out not to be worth keeping is not a
// decision waiting to be made, so it is deleted here rather than sent to the
// inbox the way an idea is (design.md, "Reference"). The audit entry keeps the
// snapshot, which is what makes the press recoverable.
func (a *App) DeleteReference(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.referenceTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM reference_items WHERE id=?`, id); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type='reference' AND item_id=?`, id); err != nil {
			return err
		}
		return a.audit(tx, EvDeleted, "reference", id, before)
	})
}

// ProcessReference: not actionable, and worth keeping. The material arrives
// written the way it will be read the day it is wanted — reworded if it needed
// it, and tagged with the area it is about, which is the only thing that makes
// a growing pile answerable (design.md, "Inbox Zero").
func (a *App) ProcessReference(id int64, f ReferenceFields) (*ReferenceItem, error) {
	if err := f.validate(); err != nil {
		return nil, err
	}
	var it *ReferenceItem
	err := a.tx(func(tx *sql.Tx) error {
		if err := a.consumeInboxItem(tx, id, EvBecameReference); err != nil {
			return err
		}
		it = &ReferenceItem{Text: f.Text, Tags: f.Tags, CreatedAt: a.now().UTC()}
		res, err := tx.Exec(`INSERT INTO reference_items (text, created_at) VALUES (?,?)`,
			it.Text, ts(it.CreatedAt))
		if err != nil {
			return err
		}
		it.ID, _ = res.LastInsertId()
		if err := a.setTagsTx(tx, "reference", it.ID, it.Tags); err != nil {
			return err
		}
		return a.audit(tx, EvCreated, "reference", it.ID, it)
	})
	if err != nil {
		return nil, err
	}
	return it, nil
}
