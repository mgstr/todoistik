package app

import "testing"

// The verb list is the half of the verb check that the server owns: what is on
// it, what comes off it, and how the Settings screen counts it. The rule that
// reads a Russian infinitive lives in app.js beside the mark it draws, and is
// not tested here because there is nothing of it here.

func TestVerbsSeedOnce(t *testing.T) {
	a, _ := newTestApp(t)
	vs, err := a.Verbs()
	if err != nil {
		t.Fatal(err)
	}
	if !hasWord(vs, "call") {
		t.Fatalf("a fresh database has no seeded verbs: %v", vs)
	}
	// A word taken off has to stay off: a seed that is re-applied is a set of
	// defaults, and a list you cannot take a word off is not one you keep.
	if err := a.RemoveVerb("call"); err != nil {
		t.Fatal(err)
	}
	if err := a.seedVerbs(); err != nil {
		t.Fatal(err)
	}
	if vs, _ = a.Verbs(); hasWord(vs, "call") {
		t.Error("the seed ran a second time and put a removed verb back")
	}
}

func TestVerbsAddAndRemove(t *testing.T) {
	a, _ := newTestApp(t)
	// Cyrillic goes on the list the same way Latin does: the imperative
	// "Позвони" is not an infinitive, so the shape rule never accepts it and
	// the list is the only way the app learns it.
	for _, w := range []string{"Позвони", "PING"} {
		if err := a.AddVerb(w); err != nil {
			t.Fatalf("%s: %v", w, err)
		}
	}
	vs, _ := a.Verbs()
	// case is not a difference: a title opens with a capital and the list
	// holds the word
	for _, w := range []string{"позвони", "ping"} {
		if !hasWord(vs, w) {
			t.Errorf("%s was not learned: %v", w, vs)
		}
	}
	for _, bad := range []string{"", "  ", "call mum", "2fa", "@call"} {
		if err := a.AddVerb(bad); err == nil {
			t.Errorf("%q was accepted: a verb is one word of letters", bad)
		}
	}
	// Unlike a tag, a verb comes off while it is in use — removing it edits no
	// item, it only means the box goes yellow the next time one of the titles
	// that opens with it is opened.
	if _, err := a.CreateAction(0, ActionFields{Title: "Ping the router"}, false); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveVerb("Ping"); err != nil {
		t.Fatalf("a verb in use was refused: %v", err)
	}
	if vs, _ = a.Verbs(); hasWord(vs, "ping") {
		t.Error("ping is still on the list")
	}
}

func TestFirstWord(t *testing.T) {
	for in, want := range map[string]string{
		"Call the bank":     "call",
		"  Buy milk  ":      "buy",
		"\"Read\" the note": "read",
		"Позвонить маме":    "позвонить",
		"back-up the disk":  "back-up",
		"":                  "",
		"   ":               "",
		"2 letters":         "",
	} {
		if got := FirstWord(in); got != want {
			t.Errorf("FirstWord(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVerbListCounts(t *testing.T) {
	a, _ := newTestApp(t)
	for _, title := range []string{"Call the bank", "call Marju", "Buy milk"} {
		if _, err := a.CreateAction(0, ActionFields{Title: title}, false); err != nil {
			t.Fatal(err)
		}
	}
	list, err := a.VerbList()
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, v := range list {
		counts[v.Name] = v.Count
	}
	// the count is what makes the seeded hundred prunable: which of them are
	// yours, and which have never opened an action
	if counts["call"] != 2 {
		t.Errorf("call counted %d actions, want 2 — case is not a difference", counts["call"])
	}
	if counts["buy"] != 1 {
		t.Errorf("buy counted %d actions, want 1", counts["buy"])
	}
	if counts["write"] != 0 {
		t.Errorf("write counted %d actions, want 0", counts["write"])
	}
}
