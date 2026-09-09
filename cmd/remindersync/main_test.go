package main

import "testing"

// captureText is where a reminder stops being a reminder: everything it holds
// has to survive into one line, because the reminder is deleted right after.
func TestCaptureText(t *testing.T) {
	cases := []struct {
		name string
		in   reminder
		want string
	}{
		{
			"title alone",
			reminder{Name: "Book winter tyres"},
			"Book winter tyres",
		},
		{
			"a day without a time keeps no time",
			reminder{Name: "Book winter tyres", AllDay: "2026-09-15"},
			"Book winter tyres (due 2026-09-15)",
		},
		{
			"a time is kept when the reminder carried one",
			reminder{Name: "Call the garage", Due: "2026-09-15 14:30"},
			"Call the garage (due 2026-09-15 14:30)",
		},
		{
			"the all-day date wins, so a bare day never gains 00:00",
			reminder{Name: "Call the garage", Due: "2026-09-15 00:00", AllDay: "2026-09-15"},
			"Call the garage (due 2026-09-15)",
		},
		{
			"the note follows the date",
			reminder{Name: "Book winter tyres", AllDay: "2026-09-15", Body: "Pärnu mnt is cheapest"},
			"Book winter tyres (due 2026-09-15) — Pärnu mnt is cheapest",
		},
		{
			"a note's own line breaks do not break the line",
			reminder{Name: "Book winter tyres", Body: "ask Marju\nabout the rims\n"},
			"Book winter tyres — ask Marju about the rims",
		},
		{
			"a reminder with nothing in it makes no line at all",
			reminder{Name: "   ", Body: "  "},
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := captureText(c.in); got != c.want {
				t.Errorf("captureText() = %q, want %q", got, c.want)
			}
		})
	}
}

// The list name and the reminder id are spliced into a script, so the escaping
// is load-bearing: a name with a quote in it must stay a name.
func TestJSString(t *testing.T) {
	cases := map[string]string{
		`Inbox`:            `"Inbox"`,
		`Marju's "list"`:   `"Marju's \"list\""`,
		"line\nbreak":      `"line\nbreak"`,
		`'); delete(); //`: `"'); delete(); //"`,
	}
	for in, want := range cases {
		if got := jsString(in); got != want {
			t.Errorf("jsString(%q) = %s, want %s", in, got, want)
		}
	}
}
