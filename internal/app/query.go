package app

import (
	"regexp"
	"strconv"
	"strings"
	"time"
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
	// the windows those filters have. The due windows are words rather than
	// dates because the question is "what is coming at me", and the answer
	// moves with the day (design.md, "Calendar"); a completed window is a word
	// or a date — see CompletedRange.
	ProblemWindow = "window"
)

// DueWindows is what the due filter accepts, in the order a person would say
// it.
var DueWindows = []string{"today", "tomorrow", "thisweek", "nextweek"}

// periodsRe is `2weeks`, `3months`, `1year`: the period you are in and the
// ones before it. The singular is accepted for the reason `1day` is on a meta
// line — refusing grammar the app understood would only be pedantry.
var periodsRe = regexp.MustCompile(`^(\d+)(week|month|year)s?$`)

// CompletedRange reads a `completed:` value into the days it covers, first and
// last inclusive, counted from today. Everything it takes is a calendar period
// and never a rolling window (design.md, "Archive"):
//
//   - 2026-09-13 — that day
//   - today, yesterday
//   - monday … sunday — the most recent one before today, never today itself,
//     which is `today`: the mirror of a due date's day name, which is never
//     today either
//   - week, month, year — the one you are in, weeks starting on Monday
//   - 2weeks, 3months, 2years — the one you are in and the ones before it, so
//     `1week` is `week` and `2weeks` on a Wednesday starts on last week's Monday
//
// Case does not matter; the caller writes the value back lowercased.
func CompletedRange(val, today string) (from, to string, ok bool) {
	val = strings.ToLower(val)
	if ValidDate(val) {
		return val, val, true
	}
	now, err := time.Parse(DateFormat, today)
	if err != nil {
		return "", "", false
	}
	day := func(t time.Time) string { return t.Format(DateFormat) }
	if wd, isDay := weekdays[val]; isDay {
		back := (int(now.Weekday()) - int(wd) + 7) % 7
		if back == 0 {
			back = 7
		}
		d := day(now.AddDate(0, 0, -back))
		return d, d, true
	}
	n, unit := 1, val
	switch val {
	case "today":
		return today, today, true
	case "yesterday":
		d := day(now.AddDate(0, 0, -1))
		return d, d, true
	case "week", "month", "year":
	default:
		m := periodsRe.FindStringSubmatch(val)
		if m == nil {
			return "", "", false
		}
		if n, err = strconv.Atoi(m[1]); err != nil || n < 1 {
			return "", "", false
		}
		unit = m[2]
	}
	switch unit {
	case "week":
		mon, sun := weekOf(now)
		start, _ := time.Parse(DateFormat, mon)
		return day(start.AddDate(0, 0, -7*(n-1))), sun, true
	case "month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return day(first.AddDate(0, -(n - 1), 0)), day(first.AddDate(0, 1, -1)), true
	default:
		first := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		return day(first.AddDate(-(n - 1), 0, 0)), day(first.AddDate(1, 0, -1)), true
	}
}

// ValidCompleted says whether a `completed:` value is one CompletedRange
// reads. Which values are readable does not depend on the day.
func ValidCompleted(val string) bool {
	_, _, ok := CompletedRange(val, "2000-01-01")
	return ok
}

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
		if key == "completed" {
			if !ValidCompleted(val) {
				problems = append(problems, QueryProblem{Token: token, Name: val, Kind: ProblemWindow})
				continue
			}
			f.Completed = strings.ToLower(val)
			continue
		}
		if !hasWord(DueWindows, val) {
			problems = append(problems, QueryProblem{Token: token, Name: val, Kind: ProblemWindow})
			continue
		}
		f.Due = val
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
