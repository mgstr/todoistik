package app

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func vocab(contexts, tags []string) *Vocabulary {
	v := &Vocabulary{Contexts: map[string]bool{}, Tags: map[string]bool{}}
	for _, c := range contexts {
		v.Contexts[c] = true
	}
	for _, t := range tags {
		v.Tags[t] = true
	}
	return v
}

func TestParseDescriptionFields(t *testing.T) {
	v := vocab([]string{"home", "person"}, []string{"car", "finance"})
	f, err := ParseDescription(
		"Ring the fitter first\n\n@home #short #focus #car @waitingFor(Marju) #today", v, true)
	if err != nil {
		t.Fatal(err)
	}
	if f.Prose != "Ring the fitter first" {
		t.Fatalf("prose: %q", f.Prose)
	}
	if f.Context != "home" || f.Duration != DurShort || !f.NeedsFocus || !f.Today {
		t.Fatalf("fields: %+v", f)
	}
	if f.AssignedTo != "Marju" {
		t.Fatalf("assigned: %q", f.AssignedTo)
	}
	if !reflect.DeepEqual(f.Tags, []string{"car"}) {
		t.Fatalf("tags: %v", f.Tags)
	}
}

// The rule that keeps prose prose: a token is metadata only if its name is
// already known. Everything else is left exactly where it was written.
func TestParseDescriptionLeavesUnknownTokensAlone(t *testing.T) {
	v := vocab([]string{"home"}, []string{"car"})
	in := "mail marju@gmail.com about invoice #12345, see @garage notes #hobby"
	f, err := ParseDescription(in, v, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.Prose != in {
		t.Fatalf("unknown names must stay prose:\n got %q\nwant %q", f.Prose, in)
	}
	if f.Context != "" || len(f.Tags) != 0 {
		t.Fatalf("nothing should have been taken: %+v", f)
	}
}

// An @ in the middle of a word is not a token even when the name is known.
func TestParseDescriptionNeedsAWordBoundary(t *testing.T) {
	v := vocab([]string{"home"}, nil)
	f, _ := ParseDescription("write to andres@home.example", v, false)
	if f.Context != "" {
		t.Fatalf("an address is not a context: %+v", f)
	}
}

func TestParseDescriptionRejectsContradictions(t *testing.T) {
	v := vocab([]string{"home", "online"}, nil)
	cases := []struct {
		name, text string
		inProject  bool
		wants      string
	}{
		{"two contexts", "@home @online", true, "at most one"},
		{"two sizes", "#short #long", true, "Pick one"},
		{"waiting with no name", "@waitingFor", true, "who or what"},
		{"parked outside a project", "#parked", false, "inside a project"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := ParseDescription(c.text, v, c.inProject); err == nil {
				t.Fatal("expected an error")
			} else if !strings.Contains(err.Error(), c.wants) {
				t.Fatalf("error should say %q: %v", c.wants, err)
			}
		})
	}
}

// Opening and saving an action twice must not shuffle or lose anything.
func TestDescribeRoundTrips(t *testing.T) {
	v := vocab([]string{"home"}, []string{"car"})
	act := &Action{
		ProjectID: 7, BecameNextAt: ptrNow(), // in a project and next, so not parked
		Description: "Ring the fitter first\nthen measure",
		Context:     "home", ContextParam: "garage", Duration: DurMedium,
		NeedsFocus: true, AssignedTo: "Marju", Tags: []string{"car", TodayTag},
		DueDate: "2026-09-20", SnoozeUntil: "2026-09-10",
	}
	text := Describe(act)
	f, err := ParseDescription(text, v, true)
	if err != nil {
		t.Fatal(err)
	}
	if f.Prose != act.Description {
		t.Fatalf("prose: got %q want %q", f.Prose, act.Description)
	}
	if f.Context != "home" || f.ContextParam != "garage" || f.Duration != DurMedium ||
		!f.NeedsFocus || f.AssignedTo != "Marju" || !f.Today || f.Parked ||
		f.DueDate != "2026-09-20" || f.SnoozeUntil != "2026-09-10" {
		t.Fatalf("fields did not survive: %+v", f)
	}
	if !reflect.DeepEqual(f.Tags, []string{"car"}) {
		t.Fatalf("tags: %v", f.Tags)
	}
	// and again, from the parsed form: the text must be identical the second
	// time, or every open-and-save would churn the field
	act2 := &Action{
		ProjectID: 7, BecameNextAt: ptrNow(),
		Description: f.Prose, Context: f.Context, ContextParam: f.ContextParam,
		Duration: f.Duration, NeedsFocus: f.NeedsFocus, AssignedTo: f.AssignedTo,
		DueDate: f.DueDate, SnoozeUntil: f.SnoozeUntil,
		Tags: append(f.Tags, TodayTag),
	}
	if again := Describe(act2); again != text {
		t.Fatalf("not stable:\n first %q\nsecond %q", text, again)
	}
}

// A parked action is one inside a project with no becameNextActionAt, and that
// is what the token has to mean in both directions.
func TestDescribeParked(t *testing.T) {
	parked := &Action{ProjectID: 3}
	if got := Describe(parked); got != "#parked" {
		t.Fatalf("parked: %q", got)
	}
	now := timeNow()
	next := &Action{ProjectID: 3, BecameNextAt: &now}
	if got := Describe(next); got != "" {
		t.Fatalf("a next action carries no token: %q", got)
	}
	standalone := &Action{BecameNextAt: &now}
	if got := Describe(standalone); got != "" {
		t.Fatalf("a standalone action carries no token: %q", got)
	}
}

func TestParseDescriptionDates(t *testing.T) {
	v := vocab(nil, nil)
	f, err := ParseDescription("chase it up due:2026-09-20 snooze:2026-09-10", v, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.DueDate != "2026-09-20" || f.SnoozeUntil != "2026-09-10" {
		t.Fatalf("dates: %+v", f)
	}
	if f.Prose != "chase it up" {
		t.Fatalf("prose: %q", f.Prose)
	}
	if _, err := ParseDescription("due:soon", v, false); err == nil {
		t.Fatal("a due date that is not a date must be refused, not dropped")
	}
	if _, err := ParseDescription("due:2026-09-20 due:2026-09-21", v, false); err == nil {
		t.Fatal("two due dates must be refused")
	}
	// a bare word with a colon is not a date token
	f, _ = ParseDescription("note: ring first", v, false)
	if f.Prose != "note: ring first" {
		t.Fatalf("ordinary prose with a colon: %q", f.Prose)
	}
}

func TestDescribeEmpty(t *testing.T) {
	if got := Describe(&Action{BecameNextAt: ptrNow()}); got != "" {
		t.Fatalf("nothing to say means an empty box, got %q", got)
	}
	if got := Describe(&Action{Description: "just prose", BecameNextAt: ptrNow()}); got != "just prose" {
		t.Fatalf("prose alone gets no trailing line, got %q", got)
	}
}

func timeNow() time.Time { return time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC) }
func ptrNow() *time.Time { t := timeNow(); return &t }
