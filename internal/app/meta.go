package app

import (
	"database/sql"
	"fmt"
	"strings"
)

// --- tag and context bookkeeping (the remembered lists) ------------------

func ensureTagTx(tx *sql.Tx, tag string) error {
	if tag == TodayTag {
		return nil // built in, not on the editable list
	}
	_, err := tx.Exec(`INSERT INTO tags (name) VALUES (?) ON CONFLICT DO NOTHING`, tag)
	return err
}

func (a *App) setTagsTx(tx *sql.Tx, itemType string, id int64, tags []string) error {
	if _, err := tx.Exec(`DELETE FROM item_tags WHERE item_type=? AND item_id=? AND tag != ?`, itemType, id, TodayTag); err != nil {
		return err
	}
	for _, t := range tags {
		if err := ensureTagTx(tx, t); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO item_tags (item_type, item_id, tag) VALUES (?,?,?) ON CONFLICT DO NOTHING`, itemType, id, t); err != nil {
			return err
		}
	}
	return nil
}

func tagsForTx(tx *sql.Tx, itemType string, id int64) ([]string, error) {
	rows, err := tx.Query(`SELECT tag FROM item_tags WHERE item_type=? AND item_id=? ORDER BY tag`, itemType, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (a *App) rememberContextTx(tx *sql.Tx, context, param string) error {
	if context == "" {
		return nil
	}
	if _, err := tx.Exec(`INSERT INTO contexts (name) VALUES (?) ON CONFLICT DO NOTHING`, context); err != nil {
		return err
	}
	if param != "" {
		if _, err := tx.Exec(`INSERT INTO context_params (context, value) VALUES (?,?) ON CONFLICT DO NOTHING`, context, param); err != nil {
			return err
		}
	}
	return nil
}

// Tags returns the remembered tag list (without the built-in #today).
func (a *App) Tags() ([]string, error) {
	return a.stringList(`SELECT name FROM tags ORDER BY name`)
}

// Contexts returns the remembered context names.
func (a *App) Contexts() ([]string, error) {
	return a.stringList(`SELECT name FROM contexts ORDER BY name`)
}

// ContextParams returns the remembered parameter values for one context.
func (a *App) ContextParams(context string) ([]string, error) {
	rows, err := a.db.Query(`SELECT value FROM context_params WHERE context=? ORDER BY value`, context)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (a *App) stringList(query string) ([]string, error) {
	rows, err := a.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// RemoveTag removes a tag from the remembered list. Refused while any item
// still carries it — removing it would be editing those items behind their
// back (design.md, "Tags").
func (a *App) RemoveTag(tag string) error {
	tag = normTag(tag)
	return a.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM item_tags WHERE tag=?`, tag).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("#%s is still carried by %d item(s)", tag, n)
		}
		_, err := tx.Exec(`DELETE FROM tags WHERE name=?`, tag)
		return err
	})
}

// RemoveContext removes a context name (and its parameter values) from the
// remembered lists. Refused while any action still carries it.
func (a *App) RemoveContext(name string) error {
	name = strings.TrimPrefix(strings.TrimSpace(name), "@")
	return a.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM actions WHERE context=?`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("@%s is still carried by %d action(s)", name, n)
		}
		if _, err := tx.Exec(`DELETE FROM context_params WHERE context=?`, name); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM contexts WHERE name=?`, name)
		return err
	})
}

// RemoveContextParam removes one remembered parameter value.
func (a *App) RemoveContextParam(context, value string) error {
	return a.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM actions WHERE context=? AND context_param=?`, context, value).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("@%s(%s) is still carried by %d action(s)", context, value, n)
		}
		_, err := tx.Exec(`DELETE FROM context_params WHERE context=? AND value=?`, context, value)
		return err
	})
}
