package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todoistik/internal/conf"
)

// Which palette the screen is painted in is the one display answer that has
// two sources: the settings file says what the app opens with, and the
// Settings screen remembers what it was told since (implementation.md,
// "Theme"). These pin the handover — that the file is obeyed until something
// is chosen, that a choice outlives the press, and that the word reaches both
// the places the page wears it, since a page that renders perfectly with the
// attribute missing is exactly how this would break silently.

func pressTheme(t *testing.T, s *Server, name string) int {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/theme/"+name, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec.Code
}

func TestTheFileSaysWhatTheAppOpensWith(t *testing.T) {
	c := conf.Defaults()
	c.Theme = conf.ThemeDark
	s, _ := serverWith(t, c)
	body := getPage(t, s, "/next")
	// both places: the document wears it so the first paint is right, and the
	// pane carries it so a boosted swap can put it back up there
	for _, want := range []string{`<html lang="en" data-theme="dark">`, `data-theme="dark">`} {
		if !strings.Contains(body, want) {
			t.Errorf("the page does not say %s", want)
		}
	}
}

func TestAChosenThemeOutlivesThePress(t *testing.T) {
	s, _ := serverWith(t, conf.Defaults()) // auto
	if code := pressTheme(t, s, conf.ThemeLight); code != http.StatusSeeOther {
		t.Fatalf("POST /theme/light: %d, want a redirect back to the page it was pressed on", code)
	}
	if got := s.theme(); got != conf.ThemeLight {
		t.Errorf("theme is %q after being told light, want %q", got, conf.ThemeLight)
	}
	if body := getPage(t, s, "/settings"); !strings.Contains(body, `data-theme="light"`) {
		t.Error("the page was not painted in the theme that was just chosen")
	}
}

// The file is the default and not the last word, which is the same arrangement
// zen.views and the panel state have: a file cannot be rewritten by a keypress.
func TestAChosenThemeBeatsTheFile(t *testing.T) {
	c := conf.Defaults()
	c.Theme = conf.ThemeDark
	s, _ := serverWith(t, c)
	pressTheme(t, s, conf.ThemeLight)
	if got := s.theme(); got != conf.ThemeLight {
		t.Errorf("theme is %q, want %q — the screen said light after the file said dark", got, conf.ThemeLight)
	}
	// and the way back to the file's answer is to say so
	pressTheme(t, s, conf.ThemeAuto)
	if got := s.theme(); got != conf.ThemeAuto {
		t.Errorf("theme is %q, want %q", got, conf.ThemeAuto)
	}
}

// A stored word outlives the code that wrote it, so one the app no longer
// knows falls back to the file rather than painting nothing.
func TestAThemeTheAppDoesNotKnowIsNotWorn(t *testing.T) {
	c := conf.Defaults()
	c.Theme = conf.ThemeDark
	s, a := serverWith(t, c)
	if err := a.SetState(themeState, "solarized"); err != nil {
		t.Fatal(err)
	}
	if got := s.theme(); got != conf.ThemeDark {
		t.Errorf("theme is %q, want the file's %q", got, conf.ThemeDark)
	}
	// and nothing can write one through the door
	if code := pressTheme(t, s, "solarized"); code != http.StatusNotFound {
		t.Errorf("POST /theme/solarized: %d, want 404", code)
	}
}

// The row offers all three, with the one in force lit and the letter on the
// next one round — so the key bar names the answer the press lands on rather
// than the name of the row (design.md, "Theme").
func TestTheRowOffersAllThreeAndTheKeyIsOnTheNext(t *testing.T) {
	for _, cur := range conf.Themes {
		got := themeChoices(cur)
		if len(got) != len(conf.Themes) {
			t.Fatalf("%q: the row holds %d answers, want %d", cur, len(got), len(conf.Themes))
		}
		on, key := "", ""
		for _, c := range got {
			if c.On {
				on = c.Name
			}
			if c.Key {
				key = c.Name
			}
		}
		if on != cur {
			t.Errorf("%q: the row lights %q", cur, on)
		}
		if want := conf.NextTheme(cur); key != want {
			t.Errorf("%q: the letter is on %q, want %q", cur, key, want)
		}
		if key == on {
			t.Errorf("%q: the letter is on the answer already in force, so the press does nothing", cur)
		}
	}
}

func TestTheSettingsScreenDrawsTheRow(t *testing.T) {
	s, _ := serverWith(t, conf.Defaults())
	body := getPage(t, s, "/settings")
	for _, want := range []string{
		`action="/theme/auto"`, `action="/theme/light"`, `action="/theme/dark"`,
		`data-key="h"`, `data-key-label="theme light"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the Settings screen does not say %s", want)
		}
	}
}
