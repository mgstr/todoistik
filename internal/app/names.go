package app

import (
	"sort"
	"strings"
)

// The remembered lists as the Settings page needs them: every name, what
// carries it, and whether it is one of the app's own.
//
// A count is shown against every name including the built-in ones, and for
// those it is not bookkeeping — it is the only place the app says how much of
// your work is short, and how much of it needs quiet. The list you
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

// NameCloud is the remembered lists as the Settings screen shows them through
// its filter line: what is left of each, and how many names are on the screen
// out of how many there are. A parameter counts as a name of its own, because
// it is removed on its own, and so does a verb.
type NameCloud struct {
	Tags, Contexts, Verbs []NameUse
	Shown, Total          int
}

// NarrowNames narrows the three lists by a filter line. Every word has to
// match, the rule the name filter uses everywhere else (see matchName). A word
// with # asks about tags only and one with @ about contexts only, so the
// notation that writes a name also says which list it is looked for on; a bare
// word asks all three, which is what a verb is written as anywhere else.
// `@person(mar` asks about the parameters under a context.
//
// A context whose own name does not match is still shown when one of its
// parameters does, with only those parameters under it: a parameter read
// without its context is a bare word, and `Marju` means nothing until it is
// `@person(Marju)` (design.md, "Contexts").
func NarrowNames(tags, contexts, verbs []NameUse, line string) NameCloud {
	var words []nameWord
	for _, w := range strings.Fields(strings.ToLower(line)) {
		words = append(words, parseNameWord(w))
	}
	all := func(match func(nameWord) bool) bool {
		for _, w := range words {
			if !match(w) {
				return false
			}
		}
		return true
	}
	var c NameCloud
	for _, t := range tags {
		c.Total++
		if all(func(w nameWord) bool { return w.tag(t.Name) }) {
			c.Tags = append(c.Tags, t)
		}
	}
	for _, ctx := range contexts {
		c.Total += 1 + len(ctx.Params)
		own := all(func(w nameWord) bool { return w.context(ctx.Name) })
		var params []NameUse
		for _, p := range ctx.Params {
			if all(func(w nameWord) bool { return w.paramOf(ctx.Name, p.Name) }) {
				params = append(params, p)
			}
		}
		if !own && len(params) == 0 {
			continue
		}
		shown := ctx
		shown.Params = params
		c.Contexts = append(c.Contexts, shown)
	}
	for _, v := range verbs {
		c.Total++
		if all(func(w nameWord) bool { return w.verb(v.Name) }) {
			c.Verbs = append(c.Verbs, v)
		}
	}
	c.Shown = len(c.Tags) + len(c.Verbs)
	for _, ctx := range c.Contexts {
		c.Shown += 1 + len(ctx.Params)
	}
	return c
}

// nameWord is one word of the Settings filter line, taken apart.
type nameWord struct {
	sigil    string // "#", "@", or "" for either list
	name     string // what the name has to contain
	param    string // what the parameter has to contain, after a (
	hasParam bool
}

func parseNameWord(w string) nameWord {
	var nw nameWord
	if strings.HasPrefix(w, "#") || strings.HasPrefix(w, "@") {
		nw.sigil, w = w[:1], w[1:]
	}
	if i := strings.Index(w, "("); i >= 0 {
		nw.hasParam = true
		nw.param = strings.TrimSuffix(w[i+1:], ")")
		w = w[:i]
	}
	nw.name = w
	return nw
}

func (w nameWord) tag(name string) bool {
	return w.sigil != "@" && !w.hasParam && strings.Contains(strings.ToLower(name), w.name)
}

func (w nameWord) context(name string) bool {
	return w.sigil != "#" && !w.hasParam && strings.Contains(strings.ToLower(name), w.name)
}

// A verb is written with no sigil anywhere in the app — it is a word in a
// title — so only a bare word on the line asks about one. `@` and `#` are
// asking the other two lists, and a verb has nothing to say to either.
func (w nameWord) verb(word string) bool {
	return w.sigil == "" && !w.hasParam && strings.Contains(word, w.name)
}

// A parameter is found by its own text or by the context it sits under, so
// `person` shows every person and `mar` shows Marju under @person.
func (w nameWord) paramOf(ctx, value string) bool {
	if w.sigil == "#" {
		return false
	}
	ctx, value = strings.ToLower(ctx), strings.ToLower(value)
	if w.hasParam {
		return strings.Contains(ctx, w.name) && strings.Contains(value, w.param)
	}
	return strings.Contains(ctx, w.name) || strings.Contains(value, w.name)
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
