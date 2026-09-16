package app

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
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

// AddTag puts a name on the remembered tag list. Adding is deliberate and
// separate from using: design.md keeps names off the free-text path so that
// `#car` and `#Car` cannot drift into two tags (see "Tags"). Idempotent.
func (a *App) AddTag(tag string) error {
	tag = normTag(tag)
	if tag == "" {
		return errors.New("a tag needs a name")
	}
	if !plainName(tag) {
		return fmt.Errorf("#%s: a tag is letters, digits, - and _", tag)
	}
	for _, s := range StructuralTags {
		if tag == s {
			return fmt.Errorf("#%s is built in", tag)
		}
	}
	return a.tx(func(tx *sql.Tx) error { return ensureTagTx(tx, tag) })
}

// AddContext puts a name on the remembered context list, and optionally one
// parameter value with it.
func (a *App) AddContext(name, param string) error {
	name = strings.TrimPrefix(strings.TrimSpace(name), "@")
	if name == "" {
		return errors.New("a context needs a name")
	}
	if !plainName(name) {
		return fmt.Errorf("@%s: a context is letters, digits, - and _", name)
	}
	if name == WaitingForContext {
		return fmt.Errorf("@%s is built in", WaitingForContext)
	}
	return a.tx(func(tx *sql.Tx) error {
		return a.rememberContextTx(tx, name, strings.TrimSpace(param))
	})
}

// nameTokenRe is one name as the Settings line writes it: #name, @name or
// @name(parameter), and nothing else on the line.
var nameTokenRe = regexp.MustCompile(`^([@#])([^\s()]+)(\(([^()]*)\))?$`)

// CreateName learns the one name a line spells. It is the Settings screen's
// Create, and it takes the notation rather than a kind and a name because the
// sigil is the only thing on that screen saying which list is meant.
//
// Three refusals are its own. A name that differs from one already on the list
// only by case is the drift the list exists to stop (design.md, "Contexts"). A
// tag has no parameter. And a parameter needs its context to exist first: the
// kind of place is the deliberate name, and learning it as a side effect of one
// value under it would add two names where one was typed.
func (a *App) CreateName(line string) error {
	line = strings.TrimSpace(line)
	m := nameTokenRe.FindStringSubmatch(line)
	if m == nil {
		return fmt.Errorf("%q is not one name: #name, @name or @name(parameter)", line)
	}
	sigil, name, hasParam, param := m[1], m[2], m[3] != "", strings.TrimSpace(m[4])
	if sigil == "#" {
		if hasParam {
			return fmt.Errorf("#%s: a tag has no parameter", name)
		}
		return a.AddTag(name)
	}
	contexts, err := a.Contexts()
	if err != nil {
		return err
	}
	have := ""
	for _, c := range contexts {
		if strings.EqualFold(c, name) {
			have = c
		}
	}
	if have != "" && have != name {
		return fmt.Errorf("@%s is already on the list as @%s", name, have)
	}
	if !hasParam {
		return a.AddContext(name, "")
	}
	if param == "" {
		return fmt.Errorf("@%s(): a parameter needs a value", name)
	}
	if have == "" {
		return fmt.Errorf("@%s is not a context yet", name)
	}
	params, err := a.ContextParams(name)
	if err != nil {
		return err
	}
	for _, p := range params {
		if strings.EqualFold(p, param) && p != param {
			return fmt.Errorf("@%s(%s) is already on the list as @%s(%s)", name, param, name, p)
		}
	}
	return a.AddContext(name, param)
}

// plainName keeps a name to what the notation can carry back out of a written
// description: a space or a bracket in it would not survive the round trip.
func plainName(s string) bool {
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return s != ""
}

// RemoveTag removes a tag from the remembered list. Refused while any item
// still carries it — removing it would be editing those items behind their
// back (design.md, "Tags").
func (a *App) RemoveTag(tag string) error {
	tag = normTag(tag)
	// A structural name is a field wearing a tag's notation, and nothing
	// carries it in item_tags — so the in-use check below would pass it and
	// the delete would quietly do nothing. Refuse it out loud: the Settings
	// page disables the control, but a disabled control is a hint, not a lock.
	for _, s := range StructuralTags {
		if tag == s {
			return fmt.Errorf("#%s is built in — it is a field, not a label", tag)
		}
	}
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
	if name == WaitingForContext {
		return fmt.Errorf("@%s is built in — it is a field, not a label", WaitingForContext)
	}
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
