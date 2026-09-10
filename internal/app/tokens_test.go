package app

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// The day the tests are standing on: a Monday, so that "the next monday" has
// a wrong answer to give (today) as well as the right one (a week out).
const testToday = "2026-09-14"

func vocab(contexts, tags []string) *Vocabulary {
	v := &Vocabulary{Contexts: map[string]bool{}, Tags: map[string]bool{}, Today: testToday}
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

// A project's line is the same notation narrowed to what a project has. What
// it does not have is refused by name: a size written on a project is a
// mistake about where the thing belongs, not a field to drop quietly.
func TestParseProjectMeta(t *testing.T) {
	v := vocab([]string{"home"}, []string{"car", "house"})
	f, err := ParseProjectMeta("#house #car snooze:2026-10-01", v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.Tags, []string{"car", "house"}) {
		t.Fatalf("tags: %v", f.Tags)
	}
	if f.SnoozeUntil != "2026-10-01" {
		t.Fatalf("snooze: %q", f.SnoozeUntil)
	}
	for _, c := range []struct{ text, wants string }{
		{"@home", "no context"},
		{"@waitingFor(Marju)", "no @waitingFor"},
		{"#short", "no size"},
		{"#focus", "no #focus"},
		{"#today", "no #today"},
		{"#parked", "no #parked"},
		{"due:2026-10-01", "no due date"},
		{"#nosuchtag", "is not notation"},
		{"a house in the country", "is not notation"},
	} {
		_, err := ParseProjectMeta(c.text, v)
		if err == nil {
			t.Fatalf("%q should have been refused", c.text)
		}
		if !strings.Contains(err.Error(), c.wants) {
			t.Fatalf("%q: error should say %q: %v", c.text, c.wants, err)
		}
	}
}

func TestWriteProjectMetaRoundTrips(t *testing.T) {
	v := vocab(nil, []string{"car", "house"})
	p := &Project{Tags: []string{"house", "car"}, SnoozeUntil: "2026-10-01"}
	line := WriteProjectMeta(p)
	f, err := ParseProjectMeta(line, v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.Tags, []string{"car", "house"}) || f.SnoozeUntil != "2026-10-01" {
		t.Fatalf("did not survive: %+v", f)
	}
	if again := WriteProjectMeta(&Project{Tags: f.Tags, SnoozeUntil: f.SnoozeUntil}); again != line {
		t.Fatalf("not stable:\n first %q\nsecond %q", line, again)
	}
	if got := WriteProjectMeta(&Project{}); got != "" {
		t.Fatalf("nothing to say means an empty box, got %q", got)
	}
}

// A date may be written as a word, and the word is resolved on the way in, so
// that what is stored is always the date it landed on.
func TestParseMetaRelativeDates(t *testing.T) {
	v := vocab(nil, nil)
	cases := []struct{ in, due string }{
		{"due:today", "2026-09-14"},
		{"due:tomorrow", "2026-09-15"},
		{"due:2026-09-20", "2026-09-20"},
		// the next one of that name, and never the one being stood on
		{"due:monday", "2026-09-21"},
		{"due:tuesday", "2026-09-15"},
		{"due:friday", "2026-09-18"},
		{"due:sunday", "2026-09-20"},
		{"due:3days", "2026-09-17"},
		{"due:3d", "2026-09-17"},
		{"due:1day", "2026-09-15"},
		{"due:0days", "2026-09-14"},
		{"due:30days", "2026-10-14"},
	}
	for _, c := range cases {
		f, err := ParseMeta(c.in, v, false)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if f.DueDate != c.due {
			t.Fatalf("%s: due %s, want %s", c.in, f.DueDate, c.due)
		}
	}
	// snooze reads the same words, and is written back out as the date
	f, err := ParseMeta("snooze:friday", v, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.SnoozeUntil != "2026-09-18" {
		t.Fatalf("snooze: %s", f.SnoozeUntil)
	}
	if got := (&Action{SnoozeUntil: f.SnoozeUntil}).Meta(); got != "snooze:2026-09-18" {
		t.Fatalf("a word must not survive the round trip: %q", got)
	}
}

// A snooze names a day that is still ahead, so the words that mean today are
// refused on it — and only on it, since they are a real answer to a deadline.
func TestParseMetaSnoozeRefusesToday(t *testing.T) {
	v := vocab(nil, nil)
	for _, in := range []string{"snooze:today", "snooze:0days", "snooze:0d"} {
		if _, err := ParseMeta(in, v, false); err == nil {
			t.Fatalf("%s was accepted", in)
		}
	}
	// an explicit date in the past is still allowed: that is a claim that went
	// stale, and the weekly review is what catches it
	if _, err := ParseMeta("snooze:2020-01-01", v, false); err != nil {
		t.Fatalf("a past date is not an error: %v", err)
	}
}

// A word that is not in the vocabulary is refused like any other, and the
// refusal says what the line does take.
func TestParseMetaRefusesUnknownDateWords(t *testing.T) {
	v := vocab(nil, nil)
	for _, in := range []string{"due:someday", "due:next-friday", "due:3weeks", "due:tomorow", "snooze:d3"} {
		_, err := ParseMeta(in, v, false)
		if err == nil {
			t.Fatalf("%s was accepted", in)
		}
		if !strings.Contains(err.Error(), "tomorrow") {
			t.Fatalf("%s: the refusal does not say what is taken: %v", in, err)
		}
	}
}

// A word is resolved against the app's clock and not the browser's, which is
// what the vocabulary carrying the day is for.
func TestVocabularyCarriesToday(t *testing.T) {
	a, _ := newTestApp(t) // a Friday, 2026-09-04
	v, err := a.Vocabulary()
	if err != nil {
		t.Fatal(err)
	}
	if v.Today != "2026-09-04" {
		t.Fatalf("today: %q", v.Today)
	}
	f, err := ParseMeta("due:monday snooze:tomorrow", v, false)
	if err != nil {
		t.Fatal(err)
	}
	if f.DueDate != "2026-09-07" || f.SnoozeUntil != "2026-09-05" {
		t.Fatalf("due %s, snooze %s", f.DueDate, f.SnoozeUntil)
	}
}

func TestParseSomedayMeta(t *testing.T) {
	v := vocab([]string{"home"}, []string{"car", "house"})
	tags, err := ParseSomedayMeta("#house #car", v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tags, []string{"car", "house"}) {
		t.Fatalf("tags: %v", tags)
	}
	// an idea carries its area of responsibility and nothing else: not a field
	// an action has, and not a date, since an idea nobody is committed to has
	// nothing to be shown later than
	for _, c := range []struct{ text, wants string }{
		{"@home", "no context"},
		{"@waitingFor(Marju)", "no @waitingFor"},
		{"#short", "no size"},
		{"#focus", "no #focus"},
		{"#today", "no #today"},
		{"#parked", "no #parked"},
		{"due:2026-10-01", "no due date"},
		{"snooze:2026-10-01", "no snooze"},
		{"#nosuchtag", "is not notation"},
		{"a house in the country", "is not notation"},
	} {
		if _, err := ParseSomedayMeta(c.text, v); err == nil {
			t.Fatalf("%q should have been refused", c.text)
		} else if !strings.Contains(err.Error(), c.wants) {
			t.Fatalf("%q: error should say %q: %v", c.text, c.wants, err)
		}
	}
}

func TestWriteSomedayMetaRoundTrips(t *testing.T) {
	v := vocab(nil, []string{"car", "house"})
	it := &SomedayItem{Tags: []string{"house", "car"}}
	line := WriteSomedayMeta(it)
	if line != "#car #house" {
		t.Fatalf("line: %q", line)
	}
	tags, err := ParseSomedayMeta(line, v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tags, []string{"car", "house"}) {
		t.Fatalf("did not survive: %v", tags)
	}
	if got := WriteSomedayMeta(&SomedayItem{}); got != "" {
		t.Fatalf("nothing to say means an empty box, got %q", got)
	}
}

// A capture may be typed with notation in it, and Inbox Zero is where it comes
// to mean something: the branch's meta line takes what it can hold and the
// words that are left are the title.
func TestReadingACapturedLine(t *testing.T) {
	v := vocab([]string{"garage"}, []string{"car", "house"})
	for _, c := range []struct{ line, meta, rest string }{
		{"Book the tyre change @garage #car #short",
			"@garage #short #car", "Book the tyre change"},
		{"#car Book the tyre change", "#car", "Book the tyre change"},
		{"Call Marju @waitingFor(Marju) due:2026-10-01 #focus",
			"@waitingFor(Marju) #focus due:2026-10-01", "Call Marju"},
		// a name the app does not know is prose, which is what keeps
		// marju@gmail.com out of the context box
		{"Mail marju@gmail.com about the #kitchen", "", "Mail marju@gmail.com about the #kitchen"},
		{"Nothing to see here", "", "Nothing to see here"},
	} {
		meta, rest := MetaFromText(c.line, v)
		if meta != c.meta || rest != c.rest {
			t.Errorf("%q → meta %q rest %q, want meta %q rest %q", c.line, meta, rest, c.meta, c.rest)
		}
	}

	// a line the notation cannot account for is left alone entirely, rather
	// than half moved and half left where a dropped token would be invisible
	meta, rest := MetaFromText("Buy paint @garage @nosuch #short #long", v)
	if meta != "" || rest != "Buy paint @garage @nosuch #short #long" {
		t.Errorf("a refused line should not be split: meta %q rest %q", meta, rest)
	}

	// the two narrow lines take the areas of responsibility and leave the rest
	// of the notation in the text, since neither is something you do
	for _, c := range []struct{ line, meta, rest string }{
		{"Rebuild the shed @garage #house #short", "#house", "Rebuild the shed @garage #short"},
		{"Learn to sail #car #house", "#car #house", "Learn to sail"},
		{"Something #nosuch", "", "Something #nosuch"},
	} {
		meta, rest := TagsFromText(c.line, v)
		if meta != c.meta || rest != c.rest {
			t.Errorf("%q → meta %q rest %q, want meta %q rest %q", c.line, meta, rest, c.meta, c.rest)
		}
	}
}

// A capture may run to more than one line, and only the first is the item
// (design.md, "Inbox item"). These pin the two halves of that: which line the
// notation is read out of, and where the body is allowed to land.

func TestSplitCaptureKeepsTheBodyOffTheLine(t *testing.T) {
	line, body := SplitCapture("Book the tyre change\nhttps://mail.example/#search/x\nquoted 240 eur")
	if line != "Book the tyre change" {
		t.Fatalf("line: %q", line)
	}
	if body != "https://mail.example/#search/x\nquoted 240 eur" {
		t.Fatalf("body: %q", body)
	}
	if l, b := SplitCapture("Buy milk"); l != "Buy milk" || b != "" {
		t.Fatalf("a one-line capture has no body: %q / %q", l, b)
	}
}

// The body arrives from wherever the capture came from and was never written
// to be read: a link to a mail is full of `#`. It must not be able to reach a
// meta line even when it does say a name the app knows.
func TestSeedReadsNotationFromTheFirstLineOnly(t *testing.T) {
	v := vocab([]string{"garage"}, []string{"car"})
	s := SeedCapture("action", "Book the tyre change @garage\nthe #car thread from Marju", v)
	if s.Meta != "@garage" {
		t.Fatalf("meta: %q — the body's #car is not notation", s.Meta)
	}
	if s.Title != "Book the tyre change" {
		t.Fatalf("title: %q", s.Title)
	}
	if s.Description != "the #car thread from Marju" {
		t.Fatalf("description: %q", s.Description)
	}
}

// A project has no description of its own, so its body goes to the first
// action — and never to the DOD, which is the one sentence the form exists to
// force out of you (design.md, "Inbox Zero").
func TestSeedGivesAProjectNoDOD(t *testing.T) {
	v := vocab(nil, []string{"car"})
	s := SeedCapture("project", "Winter tyres sorted #car\nquotes are in the mail", v)
	if s.Meta != "#car" || s.Title != "Winter tyres sorted" {
		t.Fatalf("seed: %+v", s)
	}
	if s.Description != "quotes are in the mail" {
		t.Fatalf("description: %q — it is the first action's", s.Description)
	}
}

// An unclarified idea is one free-form field and its tags, so there is nothing
// to split the capture into: all of it goes in the one box.
func TestSeedGivesASomedayItemTheWholeCapture(t *testing.T) {
	v := vocab(nil, []string{"hobby"})
	s := SeedCapture("someday", "Restore the bicycle #hobby\nthe frame is in the shed", v)
	if s.Meta != "#hobby" {
		t.Fatalf("meta: %q", s.Meta)
	}
	if s.Title != "Restore the bicycle\nthe frame is in the shed" {
		t.Fatalf("text: %q", s.Title)
	}
	if s.Description != "" {
		t.Fatalf("description: %q — the someday form has no second box", s.Description)
	}
}
