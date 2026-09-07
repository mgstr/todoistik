package app

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
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
}

func (v *Vocabulary) knownContext(name string) bool {
	return name == WaitingForContext || (v != nil && v.Contexts[name])
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
	v := &Vocabulary{Contexts: map[string]bool{}, Tags: map[string]bool{}}
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
		if !ValidDate(val) {
			err = orFirst(err, fmt.Errorf("%s:%s is not a date — write it as %s:2026-09-20", key, val, key))
			return m
		}
		switch key {
		case "due":
			if f.DueDate != "" {
				err = orFirst(err, fmt.Errorf("two due dates: %s and %s", f.DueDate, val))
				return m
			}
			f.DueDate = val
		case "snooze":
			if f.SnoozeUntil != "" {
				err = orFirst(err, fmt.Errorf("two snooze dates: %s and %s", f.SnoozeUntil, val))
				return m
			}
			f.SnoozeUntil = val
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
