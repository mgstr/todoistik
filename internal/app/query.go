package app

import (
	"regexp"
	"strings"
)

// The two windows a filter line can ask for, plus the one it cannot: `snooze:`
// is written the same way on a meta line, so it is recognised here in order to
// be refused rather than swallowed as a word.
var windowRe = regexp.MustCompile(`(^|\s)(due|completed|snooze):(\S+)`)

// The filter query: one line that says what you want to see, in the notation
// an action is already written in (see tokens.go). `@home #car #short milk`
// is "at home, about the car, short, with milk in the title".
//
// It is the same codec argument the meta line makes, pointed at the filter set
// instead of at an action's columns: the Filters remain the truth and the line
// is parsed into them and written back out of them, so what the box shows
// after an apply is what the app is actually filtering by, not what was typed.
// A line that owned the filters would be a second place for them to live.

// QueryProblem is one thing wrong with a query, attached to the token it is
// about. Problems are collected rather than returned as an error because the
// screen resolves them one at a time — create the name, pick a near one, or
// take the token out (implementation.md, "Token boxes").
type QueryProblem struct {
	Token string // exactly as written, "@hoem"
	Name  string // the name inside it, "hoem"
	Kind  string // what to do about it: see the constants below
}

const (
	// ProblemContext / ProblemTag: a name that is on no remembered list. It is
	// either new or mistyped, and only the person typing knows which.
	ProblemContext = "context"
	ProblemTag     = "tag"
	// ProblemSecondContext: an action carries one context, so asking for two
	// is asking for nothing (design.md, "Filtering by context").
	ProblemSecondContext = "second-context"
	// ProblemNotAFilter: a name the app knows that is not something this line
	// can ask for — `#parked`, which is a field and not a tag, and no view
	// this line filters shows parked actions anyway.
	ProblemNotAFilter = "not-a-filter"
	// ProblemWindow: `due:` or `completed:` given something that is not one of
	// the windows those filters have. The windows are words rather than dates
	// because the question is "what is coming at me", and the answer moves
	// with the day (design.md, "Calendar").
	ProblemWindow = "window"
)

// DueWindows and CompletedWindows are what those two filters accept, in the
// order a person would say them.
var (
	DueWindows       = []string{"today", "tomorrow", "thisweek", "nextweek"}
	CompletedWindows = []string{"today", "yesterday", "thisweek", "lastweek"}
)

func hasWord(list []string, w string) bool {
	for _, s := range list {
		if s == w {
			return true
		}
	}
	return false
}

// ParseQuery reads a filter line into the filter set it spells, and lists what
// is wrong with it. A line with problems still parses: everything it got right
// is in the Filters, so the screen can show what would happen while it asks
// about the rest.
func ParseQuery(q string, v *Vocabulary) (Filters, []QueryProblem) {
	var f Filters
	var problems []QueryProblem
	rest := []byte(q)

	// the two windows first, so that `due:today` is one thing and not the word
	// "due:today" left over for the name filter
	for _, m := range windowRe.FindAllStringSubmatchIndex(q, -1) {
		key, val := q[m[4]:m[5]], q[m[6]:m[7]]
		token := strings.TrimSpace(q[m[0]:m[1]])
		for i := m[2]; i < m[1]; i++ {
			rest[i] = ' '
		}
		if key == "snooze" {
			// a snooze is not a filter: nothing asks "show me what is asleep
			// until Tuesday", and whether a snoozed item is shown at all is
			// the view's own answer (design.md, "Time fields")
			problems = append(problems, QueryProblem{Token: token, Name: key, Kind: ProblemNotAFilter})
			continue
		}
		windows := DueWindows
		if key == "completed" {
			windows = CompletedWindows
		}
		if !hasWord(windows, val) {
			problems = append(problems, QueryProblem{Token: token, Name: val, Kind: ProblemWindow})
			continue
		}
		if key == "due" {
			f.Due = val
		} else {
			f.Completed = val
		}
	}

	for _, m := range tokenRe.FindAllStringSubmatchIndex(q, -1) {
		sigil := q[m[4]:m[5]]
		name := q[m[6]:m[7]]
		param := ""
		if m[10] >= 0 {
			param = q[m[10]:m[11]]
		}
		token := strings.TrimSpace(q[m[0]:m[1]])
		// the token is taken out of what is left, so the words that remain are
		// the name filter and nothing else
		for i := m[2]; i < m[1]; i++ {
			rest[i] = ' '
		}

		if sigil == "@" {
			switch {
			case !v.knownContext(name):
				problems = append(problems, QueryProblem{Token: token, Name: name, Kind: ProblemContext})
			case len(f.Contexts) > 0:
				problems = append(problems, QueryProblem{Token: token, Name: name, Kind: ProblemSecondContext})
			case param != "":
				f.Contexts = append(f.Contexts, name+"("+param+")")
			default:
				f.Contexts = append(f.Contexts, name)
			}
			continue
		}

		switch name {
		case string(DurShort), string(DurMedium), string(DurLong):
			f.Durations = append(f.Durations, Duration(name))
		case FocusTag:
			f.Focus = "only"
		case ParkedTag:
			problems = append(problems, QueryProblem{Token: token, Name: name, Kind: ProblemNotAFilter})
		default:
			if !v.knownTag(name) {
				problems = append(problems, QueryProblem{Token: token, Name: name, Kind: ProblemTag})
				continue
			}
			f.Tags = append(f.Tags, name)
		}
	}

	f.Name = strings.Join(strings.Fields(string(rest)), " ")
	return f, problems
}

// Query writes a filter set back out as a line, which is what the box shows
// when a view is opened with filters already on it. The order is fixed —
// context, tags, fields, then the words — so the same filter set is always the
// same line, whatever order it was typed in.
//
// Sort order is not in it: it is not something the line can say, and a filter
// set carrying it would come back out of the box changed.
func (f Filters) Query() string {
	var parts []string
	for _, c := range f.Contexts {
		parts = append(parts, "@"+c)
	}
	for _, t := range f.Tags {
		parts = append(parts, "#"+t)
	}
	for _, d := range f.Durations {
		parts = append(parts, "#"+string(d))
	}
	if f.Focus == "only" {
		parts = append(parts, "#"+FocusTag)
	}
	if f.Due != "" {
		parts = append(parts, "due:"+f.Due)
	}
	if f.Completed != "" {
		parts = append(parts, "completed:"+f.Completed)
	}
	if f.Name != "" {
		parts = append(parts, f.Name)
	}
	return strings.Join(parts, " ")
}
