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

func TestParseMetaFields(t *testing.T) {
	v := vocab([]string{"home", "person"}, []string{"car", "finance"})
	f, err := ParseMeta("@home #short #focus #car @waitingFor(Marju) #today", v, true)
	if err != nil {
		t.Fatal(err)
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

// The rule that decides what is metadata is unchanged — a token counts only if
// its name is already known — but the answer for everything else has moved.
// There is no prose on this line for an unknown name to stay as, so it is
// reported instead of being swallowed.
func TestParseMetaRefusesWhatIsNotNotation(t *testing.T) {
	v := vocab([]string{"home"}, []string{"car"})
	for _, in := range []string{
		"@garage",                     // a context that was never added
		"#hobby",                      // a tag that was never added
		"@home ring the fitter first", // prose, which belongs in the description
		"mail marju@gmail.com",        // not a token at all
	} {
		if _, err := ParseMeta(in, v, false); err == nil {
			t.Fatalf("%q should have been refused", in)
		}
	}
	if _, err := ParseMeta("@home #car due:2026-09-20", v, false); err != nil {
		t.Fatalf("a line of nothing but notation must pass: %v", err)
	}
	if _, err := ParseMeta("   ", v, false); err != nil {
		t.Fatalf("an empty line is not an error: %v", err)
	}
}

// An @ in the middle of a word is not a token even when the name is known —
// and what is left over is then refused rather than taken for a context.
func TestParseTokensNeedsAWordBoundary(t *testing.T) {
	v := vocab([]string{"home"}, nil)
	f, left, _ := parseTokens("write to andres@home.example", v, false)
	if f.Context != "" {
		t.Fatalf("an address is not a context: %+v", f)
	}
	if left != "write to andres@home.example" {
		t.Fatalf("the whole thing is leftover: %q", left)
	}
}

func TestParseMetaRejectsContradictions(t *testing.T) {
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
			if _, err := ParseMeta(c.text, v, c.inProject); err == nil {
				t.Fatal("expected an error")
			} else if !strings.Contains(err.Error(), c.wants) {
				t.Fatalf("error should say %q: %v", c.wants, err)
			}
		})
	}
}

// Opening and saving an action twice must not shuffle or lose anything.
func TestWriteMetaRoundTrips(t *testing.T) {
	v := vocab([]string{"home"}, []string{"car"})
	act := &Action{
		ProjectID: 7, BecameNextAt: ptrNow(), // in a project and next, so not parked
		Description: "Ring the fitter first\nthen measure",
		Context:     "home", ContextParam: "garage", Duration: DurMedium,
		NeedsFocus: true, AssignedTo: "Marju", Tags: []string{"car", TodayTag},
		DueDate: "2026-09-20", SnoozeUntil: "2026-09-10",
	}
	text := WriteMeta(act)
	if strings.Contains(text, "Ring the fitter") {
		t.Fatalf("the description has no business on the meta line: %q", text)
	}
	f, err := ParseMeta(text, v, true)
	if err != nil {
		t.Fatal(err)
	}
	if f.Context != "home" || f.ContextParam != "garage" || f.Duration != DurMedium ||
		!f.NeedsFocus || f.AssignedTo != "Marju" || !f.Today || f.Parked ||
		f.DueDate != "2026-09-20" || f.SnoozeUntil != "2026-09-10" {
		t.Fatalf("fields did not survive: %+v", f)
	}
	if !reflect.DeepEqual(f.Tags, []string{"car"}) {
		t.Fatalf("tags: %v", f.Tags)
	}
	// and again, from the parsed form: the line must be identical the second
	// time, or every open-and-save would churn the field
	act2 := &Action{
		ProjectID: 7, BecameNextAt: ptrNow(),
		Description: act.Description, Context: f.Context, ContextParam: f.ContextParam,
		Duration: f.Duration, NeedsFocus: f.NeedsFocus, AssignedTo: f.AssignedTo,
		DueDate: f.DueDate, SnoozeUntil: f.SnoozeUntil,
		Tags: append(f.Tags, TodayTag),
	}
	if again := WriteMeta(act2); again != text {
		t.Fatalf("not stable:\n first %q\nsecond %q", text, again)
	}
}

// A parked action is one inside a project with no becameNextActionAt, and that
// is what the token has to mean in both directions.
func TestWriteMetaParked(t *testing.T) {
	parked := &Action{ProjectID: 3}
	if got := WriteMeta(parked); got != "#parked" {
		t.Fatalf("parked: %q", got)
	}
	now := timeNow()
	next := &Action{ProjectID: 3, BecameNextAt: &now}
	if got := WriteMeta(next); got != "" {
		t.Fatalf("a next action carries no token: %q", got)
	}
	standalone := &Action{BecameNextAt: &now}
	if got := WriteMeta(standalone); got != "" {
		t.Fatalf("a standalone action carries no token: %q", got)
	}
}

func TestParseMetaDates(t *testing.T) {
	v := vocab(nil, nil)
	f, err := ParseMeta("due:2026-09-20 snooze:2026-09-10", v, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.DueDate != "2026-09-20" || f.SnoozeUntil != "2026-09-10" {
		t.Fatalf("dates: %+v", f)
	}
	if _, err := ParseMeta("due:soon", v, false); err == nil {
		t.Fatal("a due date that is not a date must be refused, not dropped")
	}
	if _, err := ParseMeta("due:2026-09-20 due:2026-09-21", v, false); err == nil {
		t.Fatal("two due dates must be refused")
	}
	// a bare word with a colon is not a date token, so it is leftover prose
	if _, left, _ := parseTokens("note: ring first", v, false); left != "note: ring first" {
		t.Fatalf("ordinary prose with a colon: %q", left)
	}
}

// An action carrying nothing gets an empty line, and its description never
// reaches this field at all.
func TestWriteMetaEmpty(t *testing.T) {
	if got := WriteMeta(&Action{BecameNextAt: ptrNow()}); got != "" {
		t.Fatalf("nothing to say means an empty box, got %q", got)
	}
	if got := WriteMeta(&Action{Description: "just prose", BecameNextAt: ptrNow()}); got != "" {
		t.Fatalf("the description is not metadata, got %q", got)
	}
}

func timeNow() time.Time { return time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC) }
func ptrNow() *time.Time { t := timeNow(); return &t }
