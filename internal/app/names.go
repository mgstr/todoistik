package app

import "sort"

// The remembered lists as the Settings page needs them: every name, what
// carries it, and whether it is one of the app's own.
//
// A count is shown against every name including the built-in ones, and for
// those it is not bookkeeping — it is the only place the app says how much of
// your work is short, how much needs focus, how much is parked. The list you
// keep the vocabulary on is a reasonable place to see the vocabulary being
// used.

// NameUse is one name on a remembered list.
type NameUse struct {
	Name    string
	Count   int
	Builtin bool      // a field wearing a name: it can never be removed
	Params  []NameUse // context parameter values, for contexts
}

// Removable reports whether Settings may offer to remove this name. Built-in
// names never; the rest only while nothing carries them, since removing one
// that is in use would be editing those items behind their back (design.md,
// "Contexts").
func (n NameUse) Removable() bool { return !n.Builtin && n.Count == 0 }

// TagList is every tag: the ones that are really fields first, in the order
// they are written, then the remembered list alphabetically.
func (a *App) TagList() ([]NameUse, error) {
	carried, err := a.countBy(`SELECT tag, COUNT(*) FROM item_tags GROUP BY tag`)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, q := range []struct{ name, query string }{
		{string(DurShort), `SELECT COUNT(*) FROM actions WHERE duration='short'`},
		{string(DurMedium), `SELECT COUNT(*) FROM actions WHERE duration='medium'`},
		{string(DurLong), `SELECT COUNT(*) FROM actions WHERE duration='long'`},
		{FocusTag, `SELECT COUNT(*) FROM actions WHERE needs_focus=1`},
		{ParkedTag, `SELECT COUNT(*) FROM actions WHERE project_id IS NOT NULL AND became_next_at IS NULL AND completed_at IS NULL`},
	} {
		n, err := a.count(q.query)
		if err != nil {
			return nil, err
		}
		counts[q.name] = n
	}
	out := []NameUse{}
	for _, n := range StructuralTags {
		c := counts[n]
		if n == TodayTag {
			c = carried[n]
		}
		out = append(out, NameUse{Name: n, Count: c, Builtin: true})
	}
	names, err := a.Tags()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, NameUse{Name: n, Count: carried[n]})
	}
	return out, nil
}

// ContextList is every context, with its remembered parameter values under it.
// @waitingFor leads, being the one context that is a field.
func (a *App) ContextList() ([]NameUse, error) {
	counts, err := a.countBy(`SELECT context, COUNT(*) FROM actions WHERE context != '' GROUP BY context`)
	if err != nil {
		return nil, err
	}
	waiting, err := a.count(`SELECT COUNT(*) FROM actions WHERE assigned_to != '' AND completed_at IS NULL`)
	if err != nil {
		return nil, err
	}
	params, err := a.paramCounts()
	if err != nil {
		return nil, err
	}
	out := []NameUse{{Name: WaitingForContext, Count: waiting, Builtin: true}}
	names, err := a.Contexts()
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, NameUse{Name: n, Count: counts[n], Params: params[n]})
	}
	return out, nil
}

// paramCounts pairs every remembered parameter value with how many actions
// carry it, so an unused one can be told from one in use without opening a
// view. Values with no actions are still listed: they are on the remembered
// list, and that is what this page edits.
func (a *App) paramCounts() (map[string][]NameUse, error) {
	used := map[string]int{}
	rows, err := a.db.Query(`SELECT context, context_param, COUNT(*) FROM actions
		WHERE context != '' AND context_param != '' GROUP BY context, context_param`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ctx, val string
		var n int
		if err := rows.Scan(&ctx, &val, &n); err != nil {
			rows.Close()
			return nil, err
		}
		used[ctx+"\x00"+val] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := map[string][]NameUse{}
	rows, err = a.db.Query(`SELECT context, value FROM context_params ORDER BY context, value`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var ctx, val string
		if err := rows.Scan(&ctx, &val); err != nil {
			return nil, err
		}
		out[ctx] = append(out[ctx], NameUse{Name: val, Count: used[ctx+"\x00"+val]})
	}
	return out, rows.Err()
}

func (a *App) count(query string) (int, error) {
	var n int
	err := a.db.QueryRow(query).Scan(&n)
	return n, err
}

func (a *App) countBy(query string) (map[string]int, error) {
	rows, err := a.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err := rows.Scan(&name, &n); err != nil {
			return nil, err
		}
		out[name] = n
	}
	return out, rows.Err()
}
