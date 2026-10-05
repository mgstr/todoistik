package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// The phrases a rule can be written in instead of fields (design.md,
// "Schedule"). They are a second way of writing the commonest rules, not a
// second kind of rule: each one fills in the same Expr the fields do, so
// matching, the search for the next occurrence and everything in internal/app
// are unaware that a phrase was ever involved.
//
// The one exception is the last day of the month, which three calendar fields
// cannot say — it is the 28th, 29th, 30th or 31st depending on the month — and
// which is therefore the one phrase that is not shorthand for anything.

var allDays = field(1<<7 - 1)     // Sunday..Saturday
var allMonths = field(1<<13 - 2)  // 1..12
var allDom = field(1<<32 - 2)     // 1..31
var workdays = field(1<<6 - 2)    // Monday..Friday
var weekends = field(1<<0 | 1<<6) // Sunday, Saturday
var monthTails = []string{"of every month", "of the month", "of month"}

// weekday reads a day of the week by its full name or the three letters the
// fields take.
func weekday(s string) (int, bool) {
	if v, ok := dayNames[s]; ok {
		return v, true
	}
	for d := time.Sunday; d <= time.Saturday; d++ {
		if s == strings.ToLower(d.String()) {
			return int(d), true
		}
	}
	return 0, false
}

// parsePhrase reads a rule written in words. It answers nil, nil for anything
// that is not a phrase at all, which is how a field expression gets past it;
// what is plainly a phrase and wrong gets an error of its own, because "month:
// bad value" is no answer to "every funday".
func parsePhrase(s string) (*Expr, error) {
	text := strings.Join(strings.Fields(strings.ToLower(s)), " ")
	e := &Expr{raw: s, mon: allMonths, dom: allDom, dow: allDays, domStar: true, dowStar: true, yearStar: true}
	switch text {
	case "every day":
		e.words = "every day"
		return e, nil
	case "workdays":
		e.dow, e.dowStar, e.words = workdays, false, "workdays"
		return e, nil
	case "weekends":
		e.dow, e.dowStar, e.words = weekends, false, "weekends"
		return e, nil
	}
	if rest, ok := strings.CutPrefix(text, "every "); ok {
		e.dow, e.dowStar = 0, false
		names := strings.FieldsFunc(strings.ReplaceAll(rest, " and ", ","), func(r rune) bool { return r == ',' || r == ' ' })
		for _, name := range names {
			d, ok := weekday(name)
			if !ok {
				return nil, fmt.Errorf("%q is not a day of the week", name)
			}
			e.dow |= 1 << uint(d)
		}
		if e.dow == 0 {
			return nil, fmt.Errorf("every what?")
		}
		e.words = "every " + spell(values(e.dow, 0, 6), func(d int) string { return time.Weekday(d).String() })
		return e, nil
	}
	for _, tail := range monthTails {
		head, ok := strings.CutSuffix(text, " day "+tail)
		if !ok {
			continue
		}
		head = strings.TrimPrefix(head, "the ")
		e.dom, e.domStar = 0, false
		switch head {
		case "last":
			e.domLast, e.words = true, "the last day of every month"
			return e, nil
		case "first":
			head = "1"
		}
		n, err := strconv.Atoi(strings.TrimRight(head, "stndrh"))
		if err != nil || n < 1 || n > 31 {
			return nil, fmt.Errorf("%q is not a day of the month", head)
		}
		e.dom, e.words = 1<<uint(n), "the "+ordinal(n)+" of every month"
		return e, nil
	}
	return nil, nil
}
