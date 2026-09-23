package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todoistik/internal/app"
	"todoistik/internal/conf"
)

// What each of `d`, `⌫` and `b` looks like on the way out is decided by the
// settings file and worn by the keyboard layer, which reads it off the pane
// (implementation.md, "A moment that shows itself"). The layer is JavaScript
// and is not tested here; the handover between the two is, because it is the
// half that can silently stop being true — a renamed field or a dropped
// attribute leaves the page rendering perfectly and every effect gone.

func serverWith(t *testing.T, c conf.Config) (*Server, *app.App) {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	s, err := New(a, "", c)
	if err != nil {
		t.Fatal(err)
	}
	return s, a
}

func TestThePaneCarriesWhatTheSettingsFileSaidAboutMotion(t *testing.T) {
	s, _ := serverWith(t, conf.Defaults())
	body := getPage(t, s, "/next")
	for _, want := range []string{
		`data-anim-done="strike"`,
		`data-anim-delete="collapse"`,
		`data-anim-back="sweep"`,
		`data-anim-ms="160"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the pane does not say %s, so the keyboard layer has nothing to read", want)
		}
	}
}

// `anim.ms = 0` is how the file turns the whole of it off in one line, and the
// number has to reach the page for that to be true — the three words are still
// there and still say an effect each, and the zero is what vetoes them.
func TestNoMotionAtAllStillReachesThePage(t *testing.T) {
	c := conf.Defaults()
	c.AnimMS = 0
	s, _ := serverWith(t, c)
	if body := getPage(t, s, "/next"); !strings.Contains(body, `data-anim-ms="0"`) {
		t.Error("the pane does not say the duration is zero")
	}
}

// The doing screen is one line of text and nothing else, so it is the line
// `anim.done = strike` draws its rule through. Nowhere else on that screen
// could the rule honestly go, and without the mark the effect falls back to a
// plain fade — which is a silent downgrade rather than an error.
func TestTheDoingScreenOffersALineToStrikeThrough(t *testing.T) {
	s, a := serverWith(t, conf.Defaults())
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Change the winter tyres"}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/doing/1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /doing/1: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "data-anim-line") {
		t.Error("the doing screen's title does not say it is the line to strike through")
	}
}
