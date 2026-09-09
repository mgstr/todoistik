package app

import (
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
// detach stamping a parked action, a delegation restamping the clock. If the
// text owned them, every one of those would have to rewrite prose.

// Names that always parse as tokens, whatever is on the remembered lists.
// Each one stands for a column, which is why none of them can be removed in
// Settings: deleting one would not remove a label, it would remove a field.
const (
	WaitingForContext = "waitingFor"
	FocusTag          = "focus"
	ParkedTag         = "parked"
)

// StructuralTags are the tags that are not tags: each is a field wearing a
// tag's notation. TodayTag is here too — it was already built in.
var StructuralTags = []string{
	string(DurShort), string(DurMedium), string(DurLong),
	FocusTag, ParkedTag, TodayTag,
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
	for _, s := range StructuralTags {
		if name == s {
			return true
		}
	}
	return v != nil && v.Tags[name]
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

// A token starts a word: preceded by the start of the text or by whitespace.
// That alone already excludes an email address, and the vocabulary check
// excludes the rest.
var tokenRe = regexp.MustCompile(`(^|\s)([@#])([\p{L}\p{N}_-]+)(\(([^)]*)\))?`)

// The two dates are written `due:2026-09-20` and `snooze:2026-09-20`. A third
// notation rather than a context or a tag, because they are neither: a date is
// not a name off a remembered list, and spelling it out keeps it readable
// without a fourth sigil to learn.
var dateRe = regexp.MustCompile(`(^|\s)(due|snooze):(\S+)`)

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
	Parked       bool
	Today        bool
	DueDate      string
	SnoozeUntil  string
	Tags         []string
}

// ParseMeta reads a meta line into the fields it spells. inProject says
// whether the action has a home, because parking is only meaningful inside one
// (design.md, "Standalone actions").
//
// Anything the notation does not account for is refused rather than kept.
// While this was one box with the description, an unknown `@name` or `#name`
// stayed prose and that was the whole anti-drift rule; on a line that holds
// nothing but names there is no prose for it to stay as, so the choice is
// between saying so and swallowing it. A name that is not on the remembered
// lists is usually a name that was never added, which is worth being told.
func ParseMeta(text string, v *Vocabulary, inProject bool) (MetaFields, error) {
	f, left, err := parseTokens(text, v, inProject)
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
func parseTokens(text string, v *Vocabulary, inProject bool) (MetaFields, string, error) {
	var f MetaFields
	var seenDuration, seenContext, seenWaiting bool
	var err error

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
			case ParkedTag:
				if !inProject {
					err = orFirst(err, errParkedStandalone)
					return keep()
				}
				f.Parked = true
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

var errParkedStandalone = fmt.Errorf("#%s only means something inside a project — a standalone action is always a next action", ParkedTag)

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
	f.Parked = act.ProjectID != 0 && act.BecameNextAt == nil
	f.DueDate, f.SnoozeUntil = act.DueDate, act.SnoozeUntil
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
	if f.Parked {
		tokens = append(tokens, "#"+ParkedTag)
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
	// inProject so that #parked parses instead of erroring in an action's
	// words; it is refused just below, in a project's
	f, left, err := parseTokens(text, v, true)
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
	case f.Parked:
		return "#" + ParkedTag
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
// The snooze is refused here even though the item has one: it is the date box
// beside this line, which is the control design.md gives the branch, and a
// second way to write the same field is a second place for it to disagree.
func ParseSomedayMeta(text string, v *Vocabulary) ([]string, error) {
	// inProject so that #parked parses rather than erroring in an action's
	// words; it is refused just below, in a someday item's
	f, left, err := parseTokens(text, v, true)
	if err != nil {
		return nil, err
	}
	if left != "" {
		return nil, fmt.Errorf("%q is not notation — an unknown #tag, or words that belong in the idea itself", left)
	}
	if f.SnoozeUntil != "" {
		return nil, fmt.Errorf("a someday/maybe item's snooze is the date beside this line, not notation")
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
