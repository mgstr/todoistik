package request

import (
	"testing"
	"time"
)

// The line is written by one program and read by another, so the grammar is
// the contract between them: a round trip that loses a field is a completion
// nobody can confirm.
func TestRoundTrip(t *testing.T) {
	in := Line{
		Source: "reminders",
		ItemID: 15,
		At:     time.Date(2026, 9, 8, 14, 30, 0, 0, time.UTC),
		Name:   "залогировать кеш 8 сентября",
	}
	text := Write(in)
	want := "Completion request from reminders ::15 ::2026-09-08T14:30 залогировать кеш 8 сентября"
	if text != want {
		t.Fatalf("Write() = %q, want %q", text, want)
	}
	out, ok := Parse(text)
	if !ok {
		t.Fatal("the line it just wrote did not parse")
	}
	if out.Source != in.Source || out.ItemID != in.ItemID || out.Name != in.Name {
		t.Errorf("Parse() = %+v, want %+v", out, in)
	}
	if !out.At.Equal(in.At) {
		t.Errorf("At = %v, want %v", out.At, in.At)
	}
}

// A line that only looks like a request is an ordinary capture. Half-reading
// one would put a screen up offering to complete something it cannot name.
func TestParseRejects(t *testing.T) {
	cases := []struct {
		name, in string
	}{
		{"an ordinary thought", "Buy new winter tyres"},
		{"the words alone", "Completion request from reminders"},
		{"no item", "Completion request from reminders ::2026-09-08T14:30 a name"},
		{"an unreadable id", "Completion request from reminders ::abc ::2026-09-08T14:30 a name"},
		{"an unreadable time", "Completion request from reminders ::15 ::yesterday a name"},
		{"a day with no time", "Completion request from reminders ::15 ::2026-09-08 a name"},
		{"no marks at all", "Completion request from reminders 15 2026-09-08T14:30 a name"},
		{"item zero, which is no item", "Completion request from reminders ::0 ::2026-09-08T14:30 a name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, ok := Parse(c.in); ok {
				t.Errorf("Parse(%q) read it as a request", c.in)
			}
		})
	}
}

// A request with no name still parses: the item may be gone by the time the
// line is read, but the id is what the answer acts on.
func TestParseNamelessStillParses(t *testing.T) {
	l, ok := Parse("Completion request from reminders ::15 ::2026-09-08T14:30")
	if !ok || l.ItemID != 15 || l.Name != "" {
		t.Fatalf("Parse() = %+v, ok=%v", l, ok)
	}
}

// In places the wall clock the source was looking at into the reader's
// timezone. The line carries no zone because both sides are one person's
// machines.
func TestIn(t *testing.T) {
	l, _ := Parse("Completion request from reminders ::15 ::2026-09-08T14:30 a name")
	tallinn, err := time.LoadLocation("Europe/Tallinn")
	if err != nil {
		t.Skip("no tzdata here")
	}
	got := l.In(tallinn)
	want := time.Date(2026, 9, 8, 14, 30, 0, 0, tallinn)
	if !got.Equal(want) {
		t.Errorf("In() = %v, want %v", got, want)
	}
}

// The marker is the identity a mirrored item carries where the other program
// keeps it, and reading it back is what survives a rename.
func TestMarker(t *testing.T) {
	if got := Marker(15); got != "(::15)" {
		t.Fatalf("Marker(15) = %q", got)
	}
	cases := []struct {
		in     string
		id     int64
		marked bool
		bare   string
	}{
		{"залогировать кеш (::15)", 15, true, "залогировать кеш"},
		{"Find GC7XYZ (::402)", 402, true, "Find GC7XYZ"},
		{"typed straight into Reminders", 0, false, "typed straight into Reminders"},
		{"a name with (brackets) in it", 0, false, "a name with (brackets) in it"},
		{"a name that ends in (::abc)", 0, false, "a name that ends in (::abc)"},
		{"only the marker (::7)", 7, true, "only the marker"},
		{"(::7)", 7, true, ""},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			id, marked := ItemFromMarker(c.in)
			if marked != c.marked || id != c.id {
				t.Errorf("ItemFromMarker(%q) = %d, %v; want %d, %v", c.in, id, marked, c.id, c.marked)
			}
			if bare := WithoutMarker(c.in); bare != c.bare {
				t.Errorf("WithoutMarker(%q) = %q, want %q", c.in, bare, c.bare)
			}
		})
	}
}
