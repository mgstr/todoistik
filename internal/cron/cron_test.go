package cron

import (
	"testing"
	"time"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestMatches(t *testing.T) {
	cases := []struct {
		expr string
		date string
		want bool
	}{
		{"* * *", "2026-09-04", true},
		{"* * 1", "2026-09-07", true},  // a Monday
		{"* * 1", "2026-09-08", false}, // a Tuesday
		{"* * mon", "2026-09-07", true},
		{"* * 7", "2026-09-06", true}, // 7 = Sunday
		{"* * 0", "2026-09-06", true},
		{"1 * *", "2026-09-01", true},
		{"1 * *", "2026-09-02", false},
		{"1 3 *", "2026-03-01", true},
		{"1 3 *", "2026-04-01", false},
		{"1 mar *", "2026-03-01", true},
		{"1-5 * *", "2026-09-03", true},
		{"*/10 * *", "2026-09-11", true},
		{"*/10 * *", "2026-09-12", false},
		{"1,15 * *", "2026-09-15", true},
		// the standard either-matches rule: 13th OR Friday
		{"13 * 5", "2026-09-13", true}, // a Sunday, but the 13th
		{"13 * 5", "2026-09-11", true}, // a Friday, not the 13th
		{"13 * 5", "2026-09-12", false},
	}
	for _, c := range cases {
		e, err := Parse(c.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.expr, err)
		}
		if got := e.Matches(day(c.date)); got != c.want {
			t.Errorf("%q on %s = %v, want %v", c.expr, c.date, got, c.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{"", "* *", "* * * *", "32 * *", "* 13 *", "* * 8", "a * *", "1-0 * *"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", s)
		}
	}
}

func TestNextAfter(t *testing.T) {
	e, _ := Parse("1 * *")
	if got := e.NextAfter(day("2026-09-04")); !got.Equal(day("2026-10-01")) {
		t.Errorf("NextAfter = %v, want 2026-10-01", got)
	}
	impossible, _ := Parse("31 2 *")
	if got := impossible.NextAfter(day("2026-01-01")); !got.IsZero() {
		t.Errorf("impossible rule returned %v, want zero", got)
	}
}

func TestReadable(t *testing.T) {
	cases := map[string]string{
		"* * *":   "every day",
		"* * 1":   "every Monday",
		"1 * *":   "the 1st of every month",
		"* * 1,5": "every Monday, Friday",
	}
	for expr, want := range cases {
		e, _ := Parse(expr)
		if got := e.Readable(); got != want {
			t.Errorf("Readable(%q) = %q, want %q", expr, got, want)
		}
	}
}
