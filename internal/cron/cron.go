// Package cron implements the day-granular cron dialect from design.md:
// three fields (day of month, month, day of week) with standard syntax and
// standard semantics, including the rule that when both day-of-month and
// day-of-week are restricted, a day matching either one fires.
package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type field uint64 // bitmask of allowed values

type Expr struct {
	raw     string
	dom     field
	mon     field
	dow     field
	domStar bool // dom field was "*" (unrestricted)
	dowStar bool // dow field was "*" (unrestricted)
}

var monthNames = map[string]int{
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

var dayNames = map[string]int{
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

// Parse parses "<dom> <month> <dow>".
func Parse(s string) (*Expr, error) {
	parts := strings.Fields(s)
	if len(parts) != 3 {
		return nil, fmt.Errorf("want 3 fields (day-of-month month day-of-week), got %d", len(parts))
	}
	e := &Expr{raw: s}
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
	return e, nil
}

func parseField(s string, min, max int, names map[string]int) (field, bool, error) {
	var f field
	star := true
	for _, part := range strings.Split(s, ",") {
		step := 1
		if i := strings.IndexByte(part, '/'); i >= 0 {
			n, err := strconv.Atoi(part[i+1:])
			if err != nil || n < 1 {
				return 0, false, fmt.Errorf("bad step %q", part)
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
				return 0, false, err
			}
		}
		if lo < min || hi > max || lo > hi {
			return 0, false, fmt.Errorf("value out of range in %q", part)
		}
		for v := lo; v <= hi; v += step {
			f |= 1 << uint(v)
		}
	}
	return f, star, nil
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
// day-of-week are restricted, either matching is enough.
func (e *Expr) Matches(t time.Time) bool {
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

// NextAfter returns the first matching day strictly after the given day,
// or the zero time if none exists within ~8 years (an impossible rule,
// e.g. "31 2 *").
func (e *Expr) NextAfter(t time.Time) time.Time {
	d := t.AddDate(0, 0, 1)
	for i := 0; i < 366*8; i++ {
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
	monPhrase := ""
	if mon != "*" {
		names := make([]string, 0, 12)
		for m := 1; m <= 12; m++ {
			if e.mon&(1<<uint(m)) != 0 {
				names = append(names, time.Month(m).String())
			}
		}
		monPhrase = " in " + strings.Join(names, ", ")
	}
	switch {
	case dom == "*" && dow == "*":
		if mon == "*" {
			return "every day"
		}
		return "every day" + monPhrase
	case dom == "*":
		names := make([]string, 0, 7)
		for d := 0; d <= 6; d++ {
			if e.dow&(1<<uint(d)) != 0 {
				names = append(names, time.Weekday(d).String())
			}
		}
		return "every " + strings.Join(names, ", ") + monPhrase
	case dow == "*":
		days := make([]string, 0, 31)
		for d := 1; d <= 31; d++ {
			if e.dom&(1<<uint(d)) != 0 {
				days = append(days, ordinal(d))
			}
		}
		return "the " + strings.Join(days, ", ") + " of every month" + monPhrase
	default:
		return e.raw
	}
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
