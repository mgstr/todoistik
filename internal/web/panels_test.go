package web

import (
	"strings"
	"testing"

	"todoistik/internal/conf"
)

// The panel state is small, and every rule in it is one a person notices the
// moment it is wrong: a zen that does not give the panels back, a screen that
// undoes the answer you just gave it, a key that means two things. These are
// those rules and not the encoding around them.

func TestDefaultIsEverythingOn(t *testing.T) {
	p := defaultPanels()
	if s := p.shown(); !s.Title || !s.Nav || !s.Keybar || s.Zen {
		t.Errorf("the app opens as %+v, want all three panels on and zen off", s)
	}
}

func TestZenGivesTheSameScreenBack(t *testing.T) {
	p := defaultPanels().toggle("nav") // the rail is off, by hand
	p = p.toggle("zen")
	if s := p.shown(); s.Title || s.Nav || s.Keybar || !s.Zen {
		t.Errorf("zen shows %+v, want nothing", s)
	}
	p = p.toggle("zen")
	s := p.shown()
	if !s.Title || !s.Keybar {
		t.Errorf("leaving zen gave back %+v, want the two that were on", s)
	}
	if s.Nav {
		t.Error("leaving zen turned the rail back on, and it was off before")
	}
}

// Asking for one panel while zen is on is a newer answer than "none of them",
// so it ends zen rather than being remembered for later.
func TestAPanelKeyEndsZen(t *testing.T) {
	p := defaultPanels().toggle("zen").toggle("title")
	s := p.shown()
	if s.Zen {
		t.Error("asking for the title bar left zen on")
	}
	if s.Title {
		t.Error("title was on before zen, so asking for it again turns it off")
	}
	if !s.Nav || !s.Keybar {
		t.Errorf("the other two came back as %+v, want both on", s)
	}
}

func TestZenViewsTurnItOnAndOffAgain(t *testing.T) {
	p := defaultPanels()

	p, changed := p.forScreen("doing")
	if !changed || !p.Zen || !p.Auto {
		t.Fatalf("arriving at a zen screen gave %+v, want zen on and marked as the app's idea", p)
	}
	// still there: the screen has had its say, and does not get it again
	if _, changed := p.forScreen("doing"); changed {
		t.Error("the same screen rendered twice changed the panels the second time")
	}
	p, changed = p.forScreen("")
	if !changed || p.Zen || p.Auto {
		t.Errorf("leaving gave %+v, want the panels back", p)
	}
}

// Zen that was asked for by hand is not the screen's to undo, on the way in or
// on the way out.
func TestZenByHandSurvivesAZenScreen(t *testing.T) {
	p := defaultPanels().toggle("zen")
	p, _ = p.forScreen("doing")
	if !p.Zen || p.Auto {
		t.Fatalf("arriving with zen already on gave %+v, want it left alone", p)
	}
	p, _ = p.forScreen("next")
	if !p.Zen {
		t.Error("leaving turned off a zen nobody asked the app for")
	}
}

// "Zen can be toggled manually afterwards" has to hold on the very screens the
// setting names, which means the screen's answer is given on arrival only.
func TestZenTurnedOffOnAZenScreenStaysOff(t *testing.T) {
	p, _ := defaultPanels().forScreen("processing")
	p = p.toggle("zen") // by hand, on the screen that opened in zen
	if p.Zen {
		t.Fatal("the key did not turn zen off")
	}
	if p, _ = p.forScreen("processing"); p.Zen {
		t.Error("the next render of the same screen turned zen back on")
	}
	// but coming back to it later is a fresh arrival
	p, _ = p.forScreen("")
	if p, _ = p.forScreen("processing"); !p.Zen {
		t.Error("arriving again did not open in zen")
	}
}

func TestTrailInheritsZenFromAnyStep(t *testing.T) {
	c := conf.Config{ZenViews: []string{"processing"}}
	trail := []crumb{{Slug: "inbox", Name: "Inbox"}, {Slug: "processing", Name: "Processing"}, {Name: "Action"}}
	if got := zenScreen(trail, c); got != "processing" {
		t.Errorf("stage two matched %q, want the run it is part of", got)
	}
	if got := zenScreen([]crumb{{Slug: "inbox", Name: "Inbox"}}, c); got != "" {
		t.Errorf("the inbox matched %q, want no match", got)
	}
}

func TestStateSurvivesTheRoundTrip(t *testing.T) {
	p := panelState{Title: true, Nav: false, Keybar: true, Zen: true, Auto: true, At: "doing"}
	if got := decodePanels(p.encode()); got != p {
		t.Errorf("stored %+v, read back %+v", p, got)
	}
	if got := decodePanels("not a=query&%%%"); got != defaultPanels() {
		t.Errorf("unreadable state gave %+v, want the defaults", got)
	}
}

// A settings file that names a screen the app does not have has to stop
// startup, like every other thing wrong with that file.
func TestZenViewsAreCheckedAgainstTheScreens(t *testing.T) {
	if err := checkZenViews(conf.Defaults()); err != nil {
		t.Errorf("the defaults were refused: %v", err)
	}
	err := checkZenViews(conf.Config{ZenViews: []string{"doing", "dooing"}})
	if err == nil {
		t.Fatal("a screen that does not exist was accepted")
	}
	if !strings.Contains(err.Error(), "dooing") {
		t.Errorf("the error %q does not name the wrong one", err)
	}
}
