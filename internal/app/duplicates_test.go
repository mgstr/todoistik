package app

import "testing"

func overlap(pct int) DupRule { return DupRule{Method: MatchOverlap, Overlap: pct} }
func similar(pct int) DupRule { return DupRule{Method: MatchSimilar, Similar: pct} }
func defaultRule() DupRule    { return DupRule{Method: MatchOverlap, Overlap: 70, Similar: 75} }
func titlesOf(ms []*Match) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Title())
	}
	return out
}

// The notation is taken out before anything is compared, and without asking a
// vocabulary: a `#` word is notation whether or not the app has heard of the
// name, and leaving the unknown ones in would make a capture match less the
// more notation was written on it (design.md, "Matches while processing").
func TestStripNotationTakesEveryTokenOut(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Book the tyre change @garage #car #short", "Book the tyre change"},
		{"Pay the rent due:2026-10-01 snooze:2026-09-28", "Pay the rent"},
		{"Call @person(Marju) about the roof", "Call about the roof"},
		{"Ask #hoem about it", "Ask about it"}, // a name on no list is notation too
		{"#car #short", ""},
	} {
		if got := StripNotation(c.in); got != c.want {
			t.Errorf("StripNotation(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The overlap rule measures against the shorter title, so a short capture
// finds the longer wording it was written as before. That is the commonest
// real repeat there is, and the other direction would score it away.
func TestOverlapMeasuresTheShorterTitle(t *testing.T) {
	short, long := wordSet("Call dentist"), wordSet("Call the dentist about the crown")
	if got := shared(short, long); got != 100 {
		t.Errorf("every word of the short title is in the long one: got %d", got)
	}
	// one differing article in a four-word title still matches at the default
	a, b := wordSet("Book a tyre change"), wordSet("Book the tyre change")
	if got := shared(a, b); got != 75 {
		t.Errorf("three of four words shared: got %d", got)
	}
	// and two titles that share only the verb do not
	if got := shared(wordSet("Fix car"), wordSet("Fix bike")); got != 50 {
		t.Errorf("one of two words shared: got %d", got)
	}
}

// Punctuation and case are not part of what a word is, and a word said twice
// is not two words.
func TestOverlapFoldsCaseAndPunctuation(t *testing.T) {
	if got := shared(wordSet("Pay the RENT!"), wordSet("pay the rent")); got != 100 {
		t.Errorf("same three words: got %d", got)
	}
	if n := len(wordSet("rent the rent")); n != 2 {
		t.Errorf("a set of words, not a list: got %d", n)
	}
}

// The similarity rule sees a typo, which is the whole reason it is offered: as
// words, "passpport" and "passport" are simply two different words.
func TestSimilarSeesATypo(t *testing.T) {
	if got := dice(bigrams("Renew passpport"), bigrams("Renew passport")); got < 85 {
		t.Errorf("a one-letter typo should score high: got %d", got)
	}
	if got := shared(wordSet("Renew passpport"), wordSet("Renew passport")); got != 50 {
		t.Errorf("as words it is half: got %d", got)
	}
	// and it is symmetric: neither side is the one being searched
	if dice(bigrams("a b c"), bigrams("c b a")) != dice(bigrams("c b a"), bigrams("a b c")) {
		t.Error("dice is not symmetric")
	}
}

// The two halves are what the screen draws: what is still open is the warning,
// what is finished is what can be copied.
func TestMatchesSplitsOpenFromFinished(t *testing.T) {
	a, _ := newTestApp(t)
	open, err := a.CreateAction(0, ActionFields{Title: "Pay the rent"}, false)
	if err != nil {
		t.Fatal(err)
	}
	done, err := a.CreateAction(0, ActionFields{Title: "Pay the rent for September"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(done.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAction(0, ActionFields{Title: "Buy milk"}, false); err != nil {
		t.Fatal(err)
	}
	o, d, ot, dt, err := a.Matches("Pay the rent @home #short", defaultRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(o) != 1 || o[0].Action.ID != open.ID || ot != 1 {
		t.Errorf("open: %v (%d)", titlesOf(o), ot)
	}
	if len(d) != 1 || d[0].Action.ID != done.ID || dt != 1 {
		t.Errorf("finished: %v (%d)", titlesOf(d), dt)
	}
	if !d[0].Done() || o[0].Done() {
		t.Error("the two halves disagree with the items in them")
	}
}

// A project matches by its own title, and the row says which kind it is
// because the answer decides which form a copy opens.
func TestMatchesFindsProjects(t *testing.T) {
	a, _ := newTestApp(t)
	p, err := a.CreateProject(ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]ActionFields{{Title: "Book the tyre change"}})
	if err != nil {
		t.Fatal(err)
	}
	o, _, _, _, err := a.Matches("Winter-proof the car", defaultRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(o) != 1 || o[0].Kind() != "project" || o[0].ID() != p.ID {
		t.Fatalf("wanted the project: %v", titlesOf(o))
	}
	// an action under it is matched on its own, and says whose step it was
	o, _, _, _, err = a.Matches("Book the tyre change", defaultRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(o) != 1 || o[0].Kind() != "action" || o[0].Home() != "Winter-proof the car" {
		t.Fatalf("wanted the action with its home: %v", o)
	}
}

// Nine, because the digits that press them are nine — and the count before
// capping is returned so the list can say what it is not showing (design.md,
// "Views").
func TestMatchesCapsAtNineAndSaysSo(t *testing.T) {
	a, _ := newTestApp(t)
	for i := 0; i < 12; i++ {
		act, err := a.CreateAction(0, ActionFields{Title: "Pay the rent"}, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := a.CompleteAction(act.ID); err != nil {
			t.Fatal(err)
		}
	}
	_, d, _, total, err := a.Matches("Pay the rent", defaultRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != MatchLimit || total != 12 {
		t.Errorf("showed %d of %d, want %d of 12", len(d), total, MatchLimit)
	}
}

// Off is off: the question is not asked and nothing is compared.
func TestMatchesAsksNothingWhenTurnedOff(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.CreateAction(0, ActionFields{Title: "Pay the rent"}, false); err != nil {
		t.Fatal(err)
	}
	o, d, _, _, err := a.Matches("Pay the rent", DupRule{Method: MatchNone})
	if err != nil {
		t.Fatal(err)
	}
	if len(o)+len(d) != 0 {
		t.Error("none should compare nothing")
	}
}

// A capture that is nothing but notation has no title to compare, and the
// empty string would otherwise match everything in the app.
func TestMatchesIgnoresACaptureThatIsAllNotation(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.CreateAction(0, ActionFields{Title: "Pay the rent"}, false); err != nil {
		t.Fatal(err)
	}
	o, d, _, _, err := a.Matches("@home #short", defaultRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(o)+len(d) != 0 {
		t.Errorf("matched on nothing: %v %v", titlesOf(o), titlesOf(d))
	}
}

// Only the first line is compared. A body arrives from wherever the capture
// came from and was not written to identify anything (design.md, "Inbox item").
func TestMatchesReadsTheFirstLineOnly(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.CreateAction(0, ActionFields{Title: "Send the quote to Marju"}, false); err != nil {
		t.Fatal(err)
	}
	o, _, _, _, err := a.Matches("Ping\nSend the quote to Marju", defaultRule())
	if err != nil {
		t.Fatal(err)
	}
	if len(o) != 0 {
		t.Errorf("the body was read: %v", titlesOf(o))
	}
}

// The threshold is the whole of what the rule is tuned by, and both ends of it
// mean something: 100 is "every word of the shorter title".
func TestOverlapThresholdDecides(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.CreateAction(0, ActionFields{Title: "Book the tyre change"}, false); err != nil {
		t.Fatal(err)
	}
	if o, _, _, _, _ := a.Matches("Book a tyre change", overlap(60)); len(o) != 1 {
		t.Error("three words of four should match at 60")
	}
	if o, _, _, _, _ := a.Matches("Book a tyre change", overlap(100)); len(o) != 0 {
		t.Error("at 100 every word has to be there")
	}
	if o, _, _, _, _ := a.Matches("Book the tyre change", overlap(100)); len(o) != 1 {
		t.Error("the same words are the same words")
	}
}

// The similarity rule is the other answer to the same question, and it is the
// one that finds a rewording the words miss.
func TestSimilarRuleFindsWhatWordsMiss(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.CreateAction(0, ActionFields{Title: "Renew passport"}, false); err != nil {
		t.Fatal(err)
	}
	if o, _, _, _, _ := a.Matches("Renew passpport", similar(75)); len(o) != 1 {
		t.Error("a typo should be found by characters")
	}
	if o, _, _, _, _ := a.Matches("Renew passpport", overlap(60)); len(o) != 0 {
		t.Error("and missed by words, which is why both are offered")
	}
}

// A copy carries what describes the work and not the occasion: the dates, the
// morning's pick and the delegation are all about a moment that has passed
// (design.md, "Copying a finished one").
func TestCopyMetaDropsTheOccasion(t *testing.T) {
	act := &Action{
		Title: "Pay the rent", Context: "home", Duration: DurShort, NeedsFocus: true,
		AssignedTo: "Marju", DueDate: "2026-09-01", SnoozeUntil: "2026-08-28",
		Tags: []string{"flat", TodayTag},
	}
	got := act.CopyMeta()
	want := "@home #short #focus #flat"
	if got != want {
		t.Errorf("CopyMeta = %q, want %q", got, want)
	}
	p := &Project{Title: "Move house", Tags: []string{"flat"}, SnoozeUntil: "2026-08-28"}
	if got := p.CopyMeta(); got != "#flat" {
		t.Errorf("project CopyMeta = %q, want %q", got, "#flat")
	}
}
