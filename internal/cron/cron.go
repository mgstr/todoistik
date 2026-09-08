// Package cron implements the day-granular cron dialect from design.md:
// day of month, month, day of week, and an optional fourth field, the year.
// The first three are standard cron's calendar fields, with standard syntax
// and standard semantics, including the rule that when both day-of-month and
// day-of-week are restricted, a day matching either one fires. The year is
// not standard cron; design.md, "Schedule" says why it is here anyway.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type field uint64 // bitmask of allowed values

type Expr struct {
	raw      string
	dom      field
	mon      field
	dow      field
	years    []int // in order, empty when unrestricted
	domStar  bool  // dom field was "*" (unrestricted)
	dowStar  bool  // dow field was "*" (unrestricted)
	yearStar bool  // the year field was absent or "*"
}

// The years a rule may name. A schedule is a thing you will actually be
// reminded of, so the range is the century this app is used in: outside it a
// four-digit number is a typo, and being told so is worth more than being
// able to schedule 2317.
const (
	yearMin = 2000
	yearMax = 2099
)

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var dayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// Parse parses "<dom> <month> <dow> [year]". The year is optional, and left
// off means every year — so every rule written before the field existed goes
// on meaning exactly what it meant.
func Parse(s string) (*Expr, error) {
	parts := strings.Fields(s)
	if len(parts) != 3 && len(parts) != 4 {
		return nil, fmt.Errorf("want 3 or 4 fields (day-of-month month day-of-week [year]), got %d", len(parts))
	}
	e := &Expr{raw: s, yearStar: true}
	var err error
	e.dom, e.domStar, err = parseField(parts[0], 1, 31, nil)
	if err != nil {
		return nil, fmt.Errorf("day of month: %w", err)
	}
	e.mon, _, err = parseField(parts[1], 1, 12, monthNames)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	e.dow, e.dowStar, err = parseField(parts[2], 0, 7, dayNames)
	if err != nil {
		return nil, fmt.Errorf("day of week: %w", err)
	}
	if e.dow&(1<<7) != 0 { // 7 is Sunday, same as 0
		e.dow |= 1
		e.dow &^= 1 << 7
	}
	if len(parts) == 4 {
		e.years, e.yearStar, err = parseYears(parts[3])
		if err != nil {
			return nil, fmt.Errorf("year: %w", err)
		}
	}
	return e, nil
}

// parseYears reads the fourth field. The years are kept as a list rather than
// a bitmask because a century does not fit in one, and nothing here needs the
// speed: a rule is matched once a day.
func parseYears(s string) ([]int, bool, error) {
	var out []int
	star, err := eachValue(s, yearMin, yearMax, nil, func(v int) { out = append(out, v) })
	if err != nil {
		return nil, false, err
	}
	if star {
		return nil, true, nil
	}
	return out, false, nil
}

func parseField(s string, min, max int, names map[string]int) (field, bool, error) {
	var f field
	star, err := eachValue(s, min, max, names, func(v int) { f |= 1 << uint(v) })
	return f, star, err
}

// eachValue walks the values a field expression allows, in order, calling set
// for each one. It is the whole of the syntax — `*`, lists, ranges, steps —
// in one place, so that the year field takes exactly what the other three
// take without the rules being written twice.
func eachValue(s string, min, max int, names map[string]int, set func(int)) (bool, error) {
	star := true
	for _, part := range strings.Split(s, ",") {
		step := 1
		if i := strings.IndexByte(part, '/'); i >= 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n < 1 {
				return false, fmt.Errorf("bad step %q", part)
			}
			step, part = n, part[:i]
			star = false
		} else if part != "*" {
			star = false
		}
		lo, hi := min, max
		if part != "*" {
			var err error
			if i := strings.IndexByte(part, '-'); i >= 0 {
				lo, err = parseValue(part[:i], names)
				if err == nil {
					hi, err = parseValue(part[i+1:], names)
				}
			} else {
				lo, err = parseValue(part, names)
				hi = lo
				if step > 1 {
					hi = max
				}
			}
			if err != nil {
				return false, err
			}
		}
		if lo < min || hi > max || lo > hi {
			return false, fmt.Errorf("value out of range in %q", part)
		}
		for v := lo; v <= hi; v += step {
			set(v)
		}
	}
	return star, nil
}

func parseValue(s string, names map[string]int) (int, error) {
	if names != nil {
		if v, ok := names[strings.ToLower(s)]; ok {
			return v, nil
		}
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("bad value %q", s)
	}
	return v, nil
}

// Matches reports whether the expression fires on the given day.
// Standard cron semantics: month must match; if both day-of-month and
// day-of-week are restricted, either matching is enough. The year, where one
// is given, narrows like the month does — it says which years the rest of the
// rule applies in, and never widens it.
func (e *Expr) Matches(t time.Time) bool {
	if !e.inYears(t.Year()) {
		return false
	}
	if e.mon&(1<<uint(t.Month())) == 0 {
		return false
	}
	domHit := e.dom&(1<<uint(t.Day())) != 0
	dowHit := e.dow&(1<<uint(t.Weekday())) != 0
	switch {
	case e.domStar && e.dowStar:
		return true
	case e.domStar:
		return dowHit
	case e.dowStar:
		return domHit
	default:
		return domHit || dowHit
	}
}

func (e *Expr) inYears(y int) bool {
	if e.yearStar {
		return true
	}
	for _, v := range e.years {
		if v == y {
			return true
		}
	}
	return false
}

// lastYear is the last year this rule can fire in, and whether it has one.
func (e *Expr) lastYear() (int, bool) {
	if e.yearStar || len(e.years) == 0 {
		return 0, false
	}
	return e.years[len(e.years)-1], true
}

// NextAfter returns the first matching day strictly after the given day, or
// the zero time if there is none. A rule that names its years is searched to
// the end of the last one, so "1 1 * 2035" finds its day however far off it
// is; one that does not is given ~8 years, after which it is an impossible
// rule (e.g. "31 2 *") rather than a distant one.
func (e *Expr) NextAfter(t time.Time) time.Time {
	d := t.AddDate(0, 0, 1)
	limit := d.AddDate(8, 0, 0)
	if last, ok := e.lastYear(); ok {
		limit = time.Date(last+1, 1, 1, 0, 0, 0, 0, d.Location())
	}
	for d.Before(limit) {
		if e.Matches(d) {
			return d
		}
		d = d.AddDate(0, 0, 1)
	}
	return time.Time{}
}

func (e *Expr) String() string { return e.raw }

// Readable renders the rule in plain words, best effort; anything it does
// not have a nice phrase for falls back to the raw expression.
func (e *Expr) Readable() string {
	parts := strings.Fields(e.raw)
	dom, mon, dow := parts[0], parts[1], parts[2]
	// The years go on the end, where a date says them: "of September 2026"
	// reads as a date does, and "in 2026" is what is left to say when there is
	// no month to hang them off.
	years := ""
	if !e.yearStar {
		years = spell(e.years, func(y int) string { return strconv.Itoa(y) })
	}
	// The months are where the days are hung: with none named the days belong
	// to every month, and with some named they belong to those. "of every
	// month in September" was what saying both produced, and it means nothing.
	months := spell(values(e.mon, 1, 12), func(m int) string { return time.Month(m).String() })
	inMonths := "of every month"
	if mon != "*" {
		inMonths = "of " + months
	}
	// where the years hang: straight onto the months if any are named, and
	// after an "in" of their own otherwise
	tail := ""
	if years != "" {
		if mon == "*" {
			tail = " in " + years
		} else {
			tail = " " + years
		}
	}
	switch {
	case dom == "*" && dow == "*":
		if mon == "*" {
			if years == "" {
				return "every day"
			}
			return "every day in " + years
		}
		return "every day in " + months + tail
	case dom == "*":
		days := spell(values(e.dow, 0, 6), func(d int) string { return time.Weekday(d).String() })
		if mon == "*" {
			if years == "" {
				return "every " + days
			}
			return "every " + days + " in " + years
		}
		return "every " + days + " in " + months + tail
	case dow == "*":
		return "the " + spell(values(e.dom, 1, 31), ordinal) + " " + inMonths + tail
	default:
		return e.raw
	}
}

// values lists what a bitmask field allows, in order.
func values(f field, lo, hi int) []int {
	var out []int
	for v := lo; v <= hi; v++ {
		if f&(1<<uint(v)) != 0 {
			out = append(out, v)
		}
	}
	return out
}

// spell writes out the values a field allows, the way the expression that set
// them was written: a run reads as a run. "the 9th, 10th, 11th, 12th, 13th,
// 14th, 15th, 16th, 17th, 18th, 19th, 20th, 21st, 22nd, 23rd" is the same
// fortnight as "the 9th-23rd" and is unreadable, which is the whole job of
// this function. Two in a row stay a list — "the 9th-10th" is longer to read
// than "the 9th, 10th" and no clearer.
func spell(vs []int, name func(int) string) string {
	var out []string
	for i := 0; i < len(vs); i++ {
		j := i
		for j+1 < len(vs) && vs[j+1] == vs[j]+1 {
			j++
		}
		if j-i >= 2 {
			out = append(out, name(vs[i])+"-"+name(vs[j]))
			i = j
			continue
		}
		out = append(out, name(vs[i]))
	}
	return strings.Join(out, ", ")
}

func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%10 == 1 && n%100 != 11:
		suffix = "st"
	case n%10 == 2 && n%100 != 12:
		suffix = "nd"
	case n%10 == 3 && n%100 != 13:
		suffix = "rd"
	}
	return strconv.Itoa(n) + suffix
}
