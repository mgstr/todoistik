package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todoistik/internal/app"
)

func getDashboard(t *testing.T, s *Server, form string) string {
	t.Helper()
	path := "/dashboard"
	if form != "" {
		path += "?form=" + form
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func dashMain(body string) string {
	return body[strings.Index(body, "<main>"):strings.Index(body, "</main>")]
}

// The bar's length is the one thing on this screen written as a style, and
// html/template sanitises every one of them — a value it will not vouch for is
// replaced with ZgotmplZ and the bar is drawn at nothing. That failure is
// silent and looks exactly like "there is no data", which is the worst thing a
// dashboard can look like by accident, so it is pinned here.
func TestTheBarsAreActuallyDrawn(t *testing.T) {
	s, a := newTestServer(t)
	for _, text := range []string{"Buy tyres", "Ask about the roof", "A newsletter"} {
		if _, _, err := a.Capture(text, app.SourceApp); err != nil {
			t.Fatal(err)
		}
	}
	inbox, _ := a.Inbox()
	if _, err := a.ProcessAction(inbox[0].ID, app.ActionFields{Title: "Book the tyre change"}, 0); err != nil {
		t.Fatal(err)
	}
	if err := a.ProcessTrash(inbox[1].ID); err != nil {
		t.Fatal(err)
	}

	for _, form := range []string{"traffic", "queue", "zero"} {
		body := getDashboard(t, s, form)
		if strings.Contains(body, "ZgotmplZ") {
			t.Fatalf("%s: a style was refused by the template's CSS filter and drawn as nothing", form)
		}
	}
	body := dashMain(getDashboard(t, s, "traffic"))
	// the longest bar on the form fills its track
	if !strings.Contains(body, `style="flex-grow:1000"`) {
		t.Error("the form's longest bar is not drawn full length")
	}
	// every segment says what it is and how much, so the screen reads with the
	// colours ignored entirely (design.md, "Dashboard")
	if !strings.Contains(body, `title="`+app.SourceApp+` 3"`) {
		t.Errorf("the segment for %q does not carry its own figure", app.SourceApp)
	}
	for _, want := range []string{"Incoming", "Outgoing", "trashed"} {
		if !strings.Contains(body, want) {
			t.Errorf("Traffic does not say %q", want)
		}
	}
	if body := dashMain(getDashboard(t, s, "queue")); !strings.Contains(body, "next actions") {
		t.Error("Queue does not name its series")
	}
}

// An empty database is the first thing this screen is ever opened against, and
// a form with nothing to count must still be a form.
func TestTheDashboardOpensOnAnEmptyDatabase(t *testing.T) {
	s, _ := newTestServer(t)
	for _, form := range []string{"", "traffic", "queue", "zero"} {
		body := getDashboard(t, s, form)
		if strings.Contains(body, "ZgotmplZ") || strings.Contains(body, "NaN") {
			t.Errorf("%q: an empty dashboard rendered a broken number", form)
		}
	}
}

// The rail's order is a decision (implementation.md, "Navigation") and the
// jump letter is derived from the link's own title, so the two cannot drift —
// but only if the title is there and says the right letter.
func TestTheDashboardSitsBetweenAuditAndSettings(t *testing.T) {
	s, _ := newTestServer(t)
	body := getDashboard(t, s, "")
	audit := strings.Index(body, `href="/audit"`)
	dash := strings.Index(body, `href="/dashboard"`)
	settings := strings.Index(body, `href="/settings"`)
	if audit < 0 || dash < 0 || settings < 0 {
		t.Fatalf("the rail is missing a row: audit=%d dashboard=%d settings=%d", audit, dash, settings)
	}
	if !(audit < dash && dash < settings) {
		t.Error("the Dashboard is not between Audit and Settings")
	}
	if !strings.Contains(body, `href="/dashboard" class="on" title="g d"`) {
		t.Error("the Dashboard's row does not carry its own jump letter while standing on it")
	}
}

// The forms are changed from the keyboard, and the key layer knows nothing
// about forms: a key exists because a link on the page declares it (keys.md,
// "The map"). So the links are the whole of the contract — three digits, and
// `j` and `k` going to the neighbours, wrapping at both ends.
func TestTheFormsAreLinksThatDeclareTheirKeys(t *testing.T) {
	s, _ := newTestServer(t)
	for _, c := range []struct{ form, on, next, prev string }{
		{"", "traffic", "queue", "zero"},
		{"queue", "queue", "zero", "traffic"},
		{"zero", "zero", "traffic", "queue"},
		{"nonsense", "traffic", "queue", "zero"},
	} {
		main := dashMain(getDashboard(t, s, c.form))
		for i, slug := range []string{"traffic", "queue", "zero"} {
			class := ""
			if slug == c.on {
				class = "on"
			}
			want := `<a href="/dashboard?form=` + slug + `" class="` + class + `" data-key="` + string(rune('1'+i)) + `"`
			if !strings.Contains(main, want) {
				t.Errorf("form=%q: missing %s", c.form, want)
			}
		}
		if !strings.Contains(main, `<a href="/dashboard?form=`+c.next+`" data-key="j"`) {
			t.Errorf("form=%q: j does not go to %s", c.form, c.next)
		}
		if !strings.Contains(main, `<a href="/dashboard?form=`+c.prev+`" data-key="k"`) {
			t.Errorf("form=%q: k does not go to %s", c.form, c.prev)
		}
		// nothing here is a section any more: the screen fits the window, and
		// a mark would hand `j` back to the scrolling it no longer does
		if strings.Contains(main, "data-kb-section") {
			t.Errorf("form=%q: the Dashboard still marks a section", c.form)
		}
	}
}

// The year is a grid of 31 columns whatever the month, so a day is found by
// its column: a month that is short leaves holes and never shifts.
func TestInboxZeroIsAFullGrid(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Buy tyres", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	main := dashMain(getDashboard(t, s, "zero"))
	grid := main[strings.Index(main, `<div class="zgrid">`):]
	// a header row and twelve months, each a label and 31 cells
	if n := strings.Count(grid, "<span"); n != 13*32 {
		t.Errorf("the grid holds %d cells, want %d", n, 13*32)
	}
	if n := strings.Count(grid, ` today"`); n != 1 {
		t.Errorf("%d cells are marked as today, want 1", n)
	}
	// the inbox was empty when today began, and that is a day it was empty on
	if !strings.Contains(grid, `<use href="#z-hit"/>`) {
		t.Error("today is not drawn with its mark")
	}
}
