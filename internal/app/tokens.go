package app

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The meta line is the one field an action's metadata is written in:
// `@context`, `@context(parameter)`, `@waitingFor(who)`, `#tag`, the four tags
// that stand for fields, and the two dates. This file is the codec between
// that line and the columns — see design.md, "Writing an action".
//
// It is a line of its own rather than something mixed into the description,
// because the two are read for different reasons: the description is read to
// remember what this action is about, and the meta line is read to see what
// the app thinks it is. Sharing a box meant every glance at one crossed the
// other, and meant the description could not be edited without editing
// notation by accident.
//
// The columns remain the truth. The text is parsed into them on save and
// written back out of them on open, rather than the other way around, because
// the app changes those fields from outside the box: picking for today, a
// completed blocker waking what waited on it, a delegation restamping the
// clock. If the text owned them, every one of those would have to rewrite
// prose — and `snooze:(Buy the frame)` would go on naming an action that was
// finished last week.

// Names that always parse as tokens, whatever is on the remembered lists.
// Each one stands for a column, which is why none of them can be removed in
// Settings: deleting one would not remove a label, it would remove a field.
const (
	WaitingForContext = "waitingFor"
	FocusTag          = "focus"
)

// StructuralTags are the tags that are not tags: each is a field wearing a
// tag's notation. TodayTag is here too — it was already built in.
var StructuralTags = []string{
	string(DurShort), string(DurMedium), string(DurLong),
	FocusTag, TodayTag,
}

// Vocabulary is what makes an `@name` or a `#name` metadata rather than prose:
// the names already on the remembered lists, plus the structural ones. A name
// that is not known stays in the text exactly as written — which is what keeps
// `marju@gmail.com` from becoming a context and `invoice #12345` from becoming
// a tag, and what keeps design.md's rule that names are never typed fresh
// (see "Contexts").
type Vocabulary struct {
	Contexts map[string]bool
	Tags     map[string]bool
	// Today is the day the line is being read on, as a date. A relative date
	// word is a word whose meaning is a day — "friday" names a different one
	// depending on when it was typed — so what it means belongs here with
	// everything else a name means right now, rather than as one more
	// parameter threaded through every parse.
	Today string
	// Siblings are the open actions of the project the line is being written
	// in, and the only actions `snooze:(...)` may name. They are here for the
	// same reason the day is: a title is a name whose meaning is an action,
	// and what a name means right now is what this type carries. Empty for a
	// standalone action, which has no siblings and so can wait on no one.
	Siblings []Sibling
	// Self is the action being edited, which may not wait on itself. 0 while
	// writing a new one, which cannot be named yet either way.
	Self int64
}

// Sibling is one action a snooze may name: its id, and the title that names it.
type Sibling struct {
	ID    int64
	Title string
}

func (v *Vocabulary) knownContext(name string) bool {
	return name == WaitingForContext || (v != nil && v.Contexts[name])
}

func (v *Vocabulary) today() string {
	if v == nil {
		return ""
	}
	return v.Today
}

func (v *Vocabulary) knownTag(name string) bool {
	return structuralTag(name) || (v != nil && v.Tags[name])
}

// Vocabulary reads the remembered lists.
func (a *App) Vocabulary() (*Vocabulary, error) {
	v := &Vocabulary{Contexts: map[string]bool{}, Tags: map[string]bool{}, Today: a.Today()}
	names, err := a.Contexts()
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		v.Contexts[n] = true
	}
	names, err = a.Tags()
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		v.Tags[n] = true
	}
	return v, nil
}

// VocabularyIn is the same lists plus the actions a `snooze:` on this screen
// may name: the open actions of the project being written in, and which of
// them is the one being edited. projectID 0 leaves the list empty, which is
// what makes `snooze:(…)` refuse itself on a standalone action rather than
// needing a rule of its own.
//
// Completed siblings are deliberately not on it. Waiting on something already
// finished is waiting on nothing, and a plan is written out of what is still
// ahead of it.
func (a *App) VocabularyIn(projectID, self int64) (*Vocabulary, error) {
	v, err := a.Vocabulary()
	if err != nil {
		return nil, err
	}
	v.Self = self
	if projectID == 0 {
		return v, nil
	}
	rows, err := a.db.Query(`SELECT id, title FROM actions
		WHERE project_id=? AND completed_at IS NULL ORDER BY id`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sib Sibling
		if err := rows.Scan(&sib.ID, &sib.Title); err != nil {
			return nil, err
		}
		v.Siblings = append(v.Siblings, sib)
	}
	return v, rows.Err()
}

// A token starts a word: preceded by the start of the text or by whitespace.
// That alone already excludes an email address, and the vocabulary check
// excludes the rest.
var tokenRe = regexp.MustCompile(`(^|\s)([@#])([\p{L}\p{N}_-]+)(\(([^)]*)\))?`)

// The two dates are written `due:2026-09-20` and `snooze:2026-09-20`. A third
// notation rather than a context or a tag, because they are neither: a date is
// not a name off a remembered list, and spelling it out keeps it readable
// without a fourth sigil to learn.
var dateRe = regexp.MustCompile(`(^|\s)(due|snooze):(\S+)`)

// A snooze also takes an action instead of a date: `snooze:(Buy the frame)`
// names a sibling by title, `snooze:#42` names one by id. Two spellings of one
// thing, and they are not redundant — the title is what you would write and
// read, and the id is the way out of the one case the title cannot express,
// which is two open siblings called the same.
//
// The brackets are the notation `@waitingFor(marju)` already uses, for the
// same reason: what is inside them is prose rather than a name off a list, and
// a title has spaces in it. Matched before dateRe, which would otherwise take
// `#42` for a date and refuse the line.
var snoozeActionRe = regexp.MustCompile(`(^|\s)snooze:(?:\(([^)]*)\)|#(\d+))`)

// Either date also takes a word that counts off from today: `tomorrow`, a day
// name, or a number of days. The app knows what day it is, and making you work
// out that Friday is the 18th is exactly the friction that ends with the date
// not being written at all (design.md, "Time fields").
var weekdays = map[string]time.Weekday{
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
	"sunday":    time.Sunday,
}

// `3days` is the spelling the panel teaches, `3d` the shorthand it accepts.
// `1day` and `3day` are accepted too: they are unambiguous, and refusing a
// number followed by the word "day" would be the app being pedantic about
// grammar it understood perfectly well.
var daysRe = regexp.MustCompile(`^(\d+)(d|days?)$`)

// resolveDate turns a date token's value into the date it names. An ISO date
// is itself; a word is counted off from today. The word is resolved here, on
// the way in, so that what is stored and what reads back out is always the
// date itself — a line that still said `friday` a week later would be a
// second, drifting opinion about when this actually is.
//
// key is carried in only so that a refusal can name the token it is about.
func resolveDate(key, val, today string) (string, error) {
	if ValidDate(val) {
		return val, nil
	}
	unknown := fmt.Errorf("%s:%s is not a date — write it as %s:2026-09-20, %s:tomorrow, %s:friday or %s:3days",
		key, val, key, key, key, key)
	base, err := time.Parse(DateFormat, today)
	if err != nil {
		return "", unknown
	}
	var days int
	wd, isDay := weekdays[val]
	switch {
	case val == "today":
		days = 0
	case val == "tomorrow":
		days = 1
	case isDay:
		// the next one of that name, never today: "snooze until monday"
		// typed on a Monday means the Monday ahead, not the one you are
		// standing in
		days = (int(wd) - int(base.Weekday()) + 7) % 7
		if days == 0 {
			days = 7
		}
	case daysRe.MatchString(val):
		n, cerr := strconv.Atoi(daysRe.FindStringSubmatch(val)[1])
		if cerr != nil {
			return "", unknown
		}
		days = n
	default:
		return "", unknown
	}
	// a snooze is a claim that this is not worth looking at yet, so a word
	// that lands on today is not one. An explicit date in the past is left
	// alone: that is a claim that went stale, which is the weekly review's to
	// catch (design.md, "Time fields")
	if key == "snooze" && days == 0 {
		return "", fmt.Errorf("snooze:%s names today, which is not a snooze — leave the snooze off instead", val)
	}
	return base.AddDate(0, 0, days).Format(DateFormat), nil
}

// MetaFields is a meta line read as the fields it spells.
type MetaFields struct {
	Context      string
	ContextParam string
	AssignedTo   string
	Duration     Duration
	NeedsFocus   bool
	Today        bool
	DueDate      string
	SnoozeUntil  string
	// SnoozeActionID is the sibling named by `snooze:(...)` or `snooze:#42`,
	// resolved here on the way in for the reason a date word is: what is
	// stored is the thing itself, so that renaming the blocker cannot quietly
	// break what was waiting on it. SnoozeActionTitle is how it reads back out.
	SnoozeActionID    int64
	SnoozeActionTitle string
	Tags              []string
}

// ParseMeta reads a meta line into the fields it spells.
//
// Anything the notation does not account for is refused rather than kept.
// While this was one box with the description, an unknown `@name` or `#name`
// stayed prose and that was the whole anti-drift rule; on a line that holds
// nothing but names there is no prose for it to stay as, so the choice is
// between saying so and swallowing it. A name that is not on the remembered
// lists is usually a name that was never added, which is worth being told.
func ParseMeta(text string, v *Vocabulary) (MetaFields, error) {
	f, left, err := parseTokens(text, v)
	if err != nil {
		return f, err
	}
	if left != "" {
		return f, fmt.Errorf("%q is not notation — an unknown @context or #tag, or prose that belongs in the description", left)
	}
	return f, nil
}

// parseTokens takes what it recognises and hands back what it did not, so that
// ParseMeta can refuse a leftover while the round-trip test can look at one.
func parseTokens(text string, v *Vocabulary) (MetaFields, string, error) {
	var f MetaFields
	var seenDuration, seenContext, seenWaiting bool
	var err error

	// the action form of the snooze goes first: `snooze:#42` would otherwise
	// be read as a date and refused as one
	text = snoozeActionRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := snoozeActionRe.FindStringSubmatch(m)
		lead, name, id := sub[1], sub[2], sub[3]
		if f.SnoozeActionID != 0 {
			err = orFirst(err, errors.New("two snoozes on one action — it waits on one thing, not two"))
			return m
		}
		sib, rerr := v.resolveSibling(name, id)
		if rerr != nil {
			err = orFirst(err, rerr)
			return m
		}
		f.SnoozeActionID, f.SnoozeActionTitle = sib.ID, sib.Title
		return lead
	})

	text = dateRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := dateRe.FindStringSubmatch(m)
		lead, key, val := sub[1], sub[2], sub[3]
		date, derr := resolveDate(key, val, v.today())
		if derr != nil {
			err = orFirst(err, derr)
			return m
		}
		switch key {
		case "due":
			if f.DueDate != "" {
				err = orFirst(err, fmt.Errorf("two due dates: %s and %s", f.DueDate, date))
				return m
			}
			f.DueDate = date
		case "snooze":
			if f.SnoozeUntil != "" {
				err = orFirst(err, fmt.Errorf("two snooze dates: %s and %s", f.SnoozeUntil, date))
				return m
			}
			// a date and an action are two answers to one question. Which of
			// them is meant is exactly what the person is in the middle of
			// deciding, so it is reported rather than guessed at (design.md,
			// "Writing an action")
			if f.SnoozeActionID != 0 {
				err = orFirst(err, fmt.Errorf("snoozed until %s and until %q — an action waits on one thing, not two",
					date, f.SnoozeActionTitle))
				return m
			}
			f.SnoozeUntil = date
		}
		return lead
	})

	prose := tokenRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := tokenRe.FindStringSubmatch(m)
		lead, sigil, name, param := sub[1], sub[2], sub[3], sub[5]
		hasParam := sub[4] != ""

		keep := func() string { return m }
		switch sigil {
		case "@":
			if !v.knownContext(name) {
				return keep()
			}
			if name == WaitingForContext {
				if !hasParam || strings.TrimSpace(param) == "" {
					err = orFirst(err, fmt.Errorf("@%s needs a name in brackets — who or what is it waiting on?", WaitingForContext))
					return keep()
				}
				if seenWaiting {
					err = orFirst(err, fmt.Errorf("@%s given twice", WaitingForContext))
					return keep()
				}
				seenWaiting = true
				f.AssignedTo = strings.TrimSpace(param)
				return lead
			}
			if seenContext {
				err = orFirst(err, fmt.Errorf("more than one context: @%s and @%s. An action has at most one", f.Context, name))
				return keep()
			}
			seenContext = true
			f.Context, f.ContextParam = name, strings.TrimSpace(param)
			return lead
		case "#":
			if !v.knownTag(name) {
				return keep()
			}
			switch d := Duration(name); d {
			case DurShort, DurMedium, DurLong:
				if seenDuration {
					err = orFirst(err, fmt.Errorf("more than one size: #%s and #%s. Pick one", f.Duration, name))
					return keep()
				}
				seenDuration = true
				f.Duration = d
				return lead
			}
			switch name {
			case FocusTag:
				f.NeedsFocus = true
			case TodayTag:
				f.Today = true
			default:
				f.Tags = append(f.Tags, name)
			}
			return lead
		}
		return keep()
	})

	sort.Strings(f.Tags)
	return f, strings.TrimSpace(collapseBlankLines(prose)), err
}

// resolveSibling turns what `snooze:` named into the action it means. Both
// spellings land here, and both are refused by name rather than dropped: a
// snooze that names nothing is a claim about an ordering that does not exist,
// and swallowing it would leave the action looking workable when the person
// had just said it was not.
func (v *Vocabulary) resolveSibling(name, id string) (Sibling, error) {
	if v == nil || len(v.Siblings) == 0 {
		return Sibling{}, errors.New("snooze:(…) names an action in the same project, and this one has no siblings to wait on — a standalone action waits on a date")
	}
	if id != "" {
		n, cerr := strconv.ParseInt(id, 10, 64)
		if cerr != nil {
			return Sibling{}, fmt.Errorf("snooze:#%s is not an action id", id)
		}
		for _, sib := range v.Siblings {
			if sib.ID == n {
				if sib.ID == v.Self {
					return Sibling{}, errSnoozeSelf
				}
				return sib, nil
			}
		}
		return Sibling{}, fmt.Errorf("snooze:#%s is not an open action of this project", id)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Sibling{}, errors.New("snooze:() names nothing — put the action it waits on in the brackets")
	}
	// the same word matching the name filter uses, for the reason filing an
	// action into a project uses it: one way of naming something that already
	// exists (design.md, "Filtering by name")
	var hits []Sibling
	for _, sib := range v.Siblings {
		if sib.ID != v.Self && matchName(name, sib.Title) {
			hits = append(hits, sib)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		if v.Self != 0 && matchName(name, siblingTitle(v.Siblings, v.Self)) {
			return Sibling{}, errSnoozeSelf
		}
		return Sibling{}, fmt.Errorf("snooze:(%s) matches no open action of this project", name)
	default:
		var titles []string
		for _, h := range hits {
			titles = append(titles, fmt.Sprintf("%q (#%d)", h.Title, h.ID))
		}
		return Sibling{}, fmt.Errorf("snooze:(%s) matches %s — name it more exactly, or by id",
			name, strings.Join(titles, " and "))
	}
}

func siblingTitle(sibs []Sibling, id int64) string {
	for _, s := range sibs {
		if s.ID == id {
			return s.Title
		}
	}
	return ""
}

var errSnoozeSelf = errors.New("an action cannot wait on itself")

func orFirst(existing, e error) error {
	if existing != nil {
		return existing
	}
	return e
}

// collapseBlankLines tidies what removing tokens leaves behind: runs of spaces
// inside a line, and the empty lines the meta line becomes once emptied.
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		out = append(out, strings.TrimRight(strings.Join(strings.Fields(ln), " "), " "))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// WriteMeta writes an action's fields back out as the line they are written
// in: every token in a fixed order, so that opening and saving an action twice
// cannot shuffle it.
func WriteMeta(act *Action) string {
	var f MetaFields
	f.Context, f.ContextParam = act.Context, act.ContextParam
	f.AssignedTo = act.AssignedTo
	f.Duration = act.Duration
	f.NeedsFocus = act.NeedsFocus
	f.DueDate, f.SnoozeUntil = act.DueDate, act.SnoozeUntil
	f.SnoozeActionID, f.SnoozeActionTitle = act.SnoozeActionID, act.SnoozeActionTitle
	for _, t := range act.Tags {
		if t == TodayTag {
			f.Today = true
			continue
		}
		f.Tags = append(f.Tags, t)
	}
	return f.String()
}

func (f MetaFields) String() string {
	var tokens []string
	if f.Context != "" {
		tokens = append(tokens, atToken(f.Context, f.ContextParam))
	}
	if f.AssignedTo != "" {
		tokens = append(tokens, atToken(WaitingForContext, f.AssignedTo))
	}
	if f.Duration != DurNone {
		tokens = append(tokens, "#"+string(f.Duration))
	}
	if f.NeedsFocus {
		tokens = append(tokens, "#"+FocusTag)
	}
	if f.Today {
		tokens = append(tokens, "#"+TodayTag)
	}
	tags := append([]string(nil), f.Tags...)
	sort.Strings(tags)
	for _, t := range tags {
		tokens = append(tokens, "#"+t)
	}
	// dates last: they are the only tokens that are not a name, and they read
	// as a tail rather than as one more label
	if f.DueDate != "" {
		tokens = append(tokens, "due:"+f.DueDate)
	}
	if f.SnoozeUntil != "" {
		tokens = append(tokens, "snooze:"+f.SnoozeUntil)
	}
	// always the title, never the id the line may have been typed with: the
	// id is a way in for the ambiguous case and not a way of reading a plan
	if f.SnoozeActionID != 0 {
		tokens = append(tokens, "snooze:("+f.SnoozeActionTitle+")")
	}
	return strings.Join(tokens, " ")
}

func atToken(name, param string) string {
	if param == "" {
		return "@" + name
	}
	return "@" + name + "(" + param + ")"
}

// Meta is WriteMeta as a method, so a template can ask an action for its meta
// line without the web layer assembling it.
func (a *Action) Meta() string { return WriteMeta(a) }

// ProjectMeta is a project's meta line read as the fields it spells. A project
// carries less than an action: no context, no size, no deadline and nobody it
// is waiting on. Those belong to the actions under it — a project is not a
// thing you do, so there is nothing for them to describe (design.md, "Tags").
type ProjectMeta struct {
	Tags        []string
	SnoozeUntil string
}

// ParseProjectMeta reads a project's meta line. It is the same notation read
// by the same parser, narrowed to what a project has: writing an action's
// field here is refused by name rather than ignored, because a size or a
// context written on a project is a mistake about where the thing belongs, and
// silently dropping it would leave that mistake believed.
func ParseProjectMeta(text string, v *Vocabulary) (ProjectMeta, error) {
	f, left, err := parseTokens(text, v)
	if err != nil {
		return ProjectMeta{}, err
	}
	if left != "" {
		return ProjectMeta{}, fmt.Errorf("%q is not notation — an unknown name, or prose that belongs in the definition of done", left)
	}
	if what := nonTagField(f); what != "" {
		return ProjectMeta{}, fmt.Errorf("a project has no %s; that belongs on an action under it", what)
	}
	return ProjectMeta{Tags: f.Tags, SnoozeUntil: f.SnoozeUntil}, nil
}

// nonTagField names the first thing on a parsed line that only an action
// carries, and "" when there is none. Two lines are narrower than an action's
// — a project's and a someday/maybe item's — and both narrow to the same set,
// so they ask the same question here and each says its own sentence about the
// answer. Writing an action's field on either is a mistake about where the
// thing belongs, and dropping it silently would leave that mistake believed.
func nonTagField(f MetaFields) string {
	switch {
	case f.Context != "":
		return "context"
	case f.AssignedTo != "":
		return "@" + WaitingForContext
	case f.Duration != DurNone:
		return "size"
	case f.NeedsFocus:
		return "#" + FocusTag
	case f.Today:
		return "#" + TodayTag
	case f.SnoozeActionID != 0:
		return "snooze on an action"
	case f.DueDate != "":
		return "due date"
	}
	return ""
}

// ParseSomedayMeta reads a someday/maybe item's meta line, which carries tags
// and nothing else. An idea you have decided not to commit to has no context,
// no size and no deadline — it is not something you are doing — but it does
// belong to an area of responsibility, and that is what the monthly walk
// groups it by (design.md, "Someday/maybe item").
//
// A snooze is refused like the rest: the item has no such field. Hiding an
// idea until a date, on a list you already chose to open, is hiding it from
// the one walk that exists to look at it.
func ParseSomedayMeta(text string, v *Vocabulary) ([]string, error) {
	f, left, err := parseTokens(text, v)
	if err != nil {
		return nil, err
	}
	if left != "" {
		return nil, fmt.Errorf("%q is not notation — an unknown #tag, or words that belong in the idea itself", left)
	}
	if f.SnoozeUntil != "" {
		return nil, fmt.Errorf("a someday/maybe item has no snooze; it waits on the list until you decide about it")
	}
	if what := nonTagField(f); what != "" {
		return nil, fmt.Errorf("a someday/maybe item has no %s; it is an idea, not something you are doing", what)
	}
	return f.Tags, nil
}

// WriteSomedayMeta is the other direction, through the same writer for the
// reason WriteProjectMeta goes through it: one notation, not a third dialect.
func WriteSomedayMeta(it *SomedayItem) string {
	return MetaFields{Tags: it.Tags}.String()
}

// Meta is WriteSomedayMeta as a method, so a template can ask the item.
func (it *SomedayItem) Meta() string { return WriteSomedayMeta(it) }

// WriteProjectMeta is the other direction, and it goes through the same
// writer, so a project's line cannot drift into a second dialect of the same
// notation.
func WriteProjectMeta(p *Project) string {
	return MetaFields{Tags: p.Tags, SnoozeUntil: p.SnoozeUntil}.String()
}

// Meta is WriteProjectMeta as a method, for the same reason Action.Meta is.
func (p *Project) Meta() string { return WriteProjectMeta(p) }

// --- reading a captured line ---------------------------------------------
//
// A capture may be typed with notation in it — "Book the tyre change @garage
// #car #short" — because the notation is the shortest way to write down what
// you already know at the moment of capture, and stopping to open the app is
// exactly what capture must never require (design.md, "Inbox item"). It means
// nothing until the item is processed: these two functions are what Inbox Zero
// reads it with, seeding a branch's meta line with what that line can hold and
// leaving the rest of the words as the title.
//
// Nothing is decided by this. It fills in a form that is still answered by
// hand, which is what keeps it on the right side of the rule that nothing
// arrives already formed (design.md, "Deliberate omissions").

// MetaFromText splits a captured line into an action's meta line and what is
// left of the line once the notation is taken out of it.
//
// A line the notation cannot account for is left alone entirely — an empty
// meta line and the text unchanged. Moving half of a misread line into a box
// and leaving half in the title is worse than not reading it at all: what was
// dropped would be invisible, and this is the one moment an item is being
// looked at deliberately.
//
// A `snooze:` naming an action never survives this: a capture belongs to no
// project yet, so there are no siblings for it to name, and the whole line is
// left alone rather than half-read. Which is right — what a new action waits
// on is decided on the form, with the project it is being filed into already
// chosen.
func MetaFromText(text string, v *Vocabulary) (meta, rest string) {
	f, left, err := parseTokens(text, v)
	if err != nil {
		return "", text
	}
	return f.String(), left
}

// TagsFromText is the same reading narrowed to tags, for the two lines that
// hold nothing else: a project's and a someday/maybe item's. Everything else
// stays in the text where it was typed — a context, a size or a deadline
// describes doing something, and neither of those two is something you do
// (design.md, "Writing a project", "Someday/maybe item").
func TagsFromText(text string, v *Vocabulary) (meta, rest string) {
	var tags []string
	rest = tokenRe.ReplaceAllStringFunc(text, func(m string) string {
		sub := tokenRe.FindStringSubmatch(m)
		lead, sigil, name := sub[1], sub[2], sub[3]
		if sigil != "#" || sub[4] != "" || structuralTag(name) || !v.knownTag(name) {
			return m
		}
		tags = append(tags, name)
		return lead
	})
	return MetaFields{Tags: tags}.String(), strings.TrimSpace(collapseBlankLines(rest))
}

// CaptureSeed is what one of the three creating branches of Inbox Zero starts
// its form from, read out of the captured item.
//
// There is no DOD field here, and that is the point: a definition of done is
// the one sentence the project form exists to force out of you, and a body
// pre-filled there would satisfy the check that makes a project a project,
// leaving a definition of done that defines nothing and is read at every
// review from then on (design.md, "Inbox Zero").
type CaptureSeed struct {
	Meta        string // the branch's meta line, read from the first line only
	Title       string // the words left on that line — the Idea box, for someday
	Description string // the body, where the branch has somewhere to keep it
}

// SeedCapture reads a capture into the fields a branch's form starts from.
//
// The first line is read as notation and the body never is (see SplitCapture),
// and the body then goes where that branch keeps material: an action's
// description, the first action of a project — a project has no description of
// its own, and material a project needs belongs to whichever of its actions
// needs it — and, for a someday/maybe item, into the one box its form has,
// since an unclarified idea is a single free-form field and there is nothing to
// split the capture into.
//
// Nothing is decided by any of it. It fills in a form that is still answered by
// hand.
func SeedCapture(branch, text string, v *Vocabulary) CaptureSeed {
	line, body := SplitCapture(text)
	switch branch {
	case "action":
		meta, title := MetaFromText(line, v)
		return CaptureSeed{Meta: meta, Title: title, Description: body}
	case "project":
		meta, title := TagsFromText(line, v)
		return CaptureSeed{Meta: meta, Title: title, Description: body}
	case "someday":
		meta, text := TagsFromText(line, v)
		if body != "" {
			text = strings.TrimSpace(text + "\n" + body)
		}
		return CaptureSeed{Meta: meta, Title: text}
	}
	return CaptureSeed{}
}

// structuralTag reports a #name that stands for a field rather than for an
// area of responsibility.
func structuralTag(name string) bool {
	for _, s := range StructuralTags {
		if name == s {
			return true
		}
	}
	return false
}

// PromotedMeta is the project meta line a promotion starts from: this action's
// tags, and nothing else it carries. design.md, "Promoting an action", sends
// the tags to the project and everything else to its first action, so a
// context or a size must not arrive here — and #today is a pick made this
// morning, not something a new project should inherit.
func (a *Action) PromotedMeta() string {
	var tags []string
	for _, t := range a.Tags {
		if t != TodayTag {
			tags = append(tags, t)
		}
	}
	return WriteProjectMeta(&Project{Tags: tags})
}

// CopyMeta is the meta line a copy of this action starts from: what describes
// the *work* — where it is done, how big it is, whether it needs quiet, and
// which areas it belongs to (design.md, "Copying a finished one").
//
// Four of an action's fields are deliberately not in it, and they are the four
// that describe an occasion rather than a job: a deadline was a date in a
// month that has passed, #today was a morning's pick, and a snooze named
// either that same month or a sibling of a project this copy is not in yet.
// Delegation goes with them — who does it is settled by the form being filled in now, which is
// exactly what design.md says the Action branch decides (see "Inbox Zero").
func (a *Action) CopyMeta() string {
	var tags []string
	for _, t := range a.Tags {
		if t != TodayTag {
			tags = append(tags, t)
		}
	}
	return MetaFields{
		Context:      a.Context,
		ContextParam: a.ContextParam,
		Duration:     a.Duration,
		NeedsFocus:   a.NeedsFocus,
		Tags:         tags,
	}.String()
}

// CopyMeta is the project version: its tags, and not the snooze it happened to
// be wearing. A snooze is "do not bug me until", which is a thing said about a
// moment in time and not about an outcome (design.md, "Time fields").
func (p *Project) CopyMeta() string {
	return WriteProjectMeta(&Project{Tags: p.Tags})
}
