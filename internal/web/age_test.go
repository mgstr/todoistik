package web

import (
	"testing"
	"time"
)

// The age labels are all boundaries, and every one of them was argued over
// once (see research/item-line-study.html). Two of those arguments were about
// gaps a reader would never notice until they hit one: an age of two days,
// and the days between "a month ago" and "2 months ago". Pin them all.
func TestHumanAge(t *testing.T) {
	cases := []struct {
		days int
		want string
	}{
		{0, "today"},
		{1, "yesterday"},
		{2, "2 days ago"}, // the range starts here: "yesterday" covers only day 1
		{6, "6 days ago"},
		{7, "a week ago"},
		{13, "a week ago"},
		{14, "2 weeks ago"},
		{20, "2 weeks ago"},
		{21, "3 weeks ago"},
		{27, "3 weeks ago"},
		{28, "a month ago"},
		{59, "a month ago"}, // runs to just under two months, closing the gap
		{60, "2 months ago"},
		{89, "2 months ago"},
		{90, "3 months ago"},
		{364, "11 months ago"}, // 12 thirty-day months, but not yet a year
		{365, "a year ago"},
		{729, "a year ago"},
		{730, "2 years ago"},
		{1095, "3 years ago"},
	}
	for _, c := range cases {
		got := humanAge(time.Now().Add(-time.Duration(c.days) * 24 * time.Hour))
		if got != c.want {
			t.Errorf("humanAge(%d days) = %q, want %q", c.days, got, c.want)
		}
	}
}

// A date that has not arrived yet is not an age. No caller should pass one,
// but a due date is a string away from a created date and this should not
// read as "in -3 days".
func TestHumanAgeFuture(t *testing.T) {
	if got := humanAge(time.Now().Add(72 * time.Hour)); got != "today" {
		t.Errorf("humanAge(3 days ahead) = %q, want %q", got, "today")
	}
}
