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

// A completed item opens read-only (design.md, "Completion"). The domain
// refuses the writes whatever arrives — that is pinned in internal/app — and
// this pins the other half: the screen does not offer them in the first
// place. A page that still drew the form would be a page that looks editable
// and answers every save with a banner.

func newTestServer(t *testing.T) (*Server, *app.App) {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	// no token: the handler is the mux either way, and a cookie per request
	// would be testing the login and not the page
	s, err := New(a, "", conf.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return s, a
}

func getPage(t *testing.T, s *Server, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func TestACompletedActionOpensWithNoFormOnIt(t *testing.T) {
	s, a := newTestServer(t)
	act, err := a.CreateAction(0, app.ActionFields{
		Title:       "Change the winter tyres",
		Description: "quote at https://example.com/tyres",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}

	body := getPage(t, s, "/action/1")

	// nothing to type in, and nothing that would write the action back. The
	// field names rather than "<textarea": the capture dialog rides on every
	// page and has one of its own
	for _, frag := range []string{`name="title"`, `name="meta"`, `name="description"`, `action="/action/1"`} {
		if strings.Contains(body, frag) {
			t.Errorf("a completed action's page still carries %q", frag)
		}
	}
	// and none of the state changes an open action offers
	for _, verb := range []string{"complete", "delete", "pick", "park", "next", "detach", "promote"} {
		if strings.Contains(body, "/action/1/"+verb) {
			t.Errorf("a completed action's page still offers %q", verb)
		}
	}
	// the one control that is offered, because it is what lifts the freeze
	if !strings.Contains(body, "/action/1/uncomplete") {
		t.Error("a completed action cannot be brought back from its own page")
	}
	// the fields are still all readable, and the link in the description is
	// still followable — a frozen item is read, and reading it is the point
	if !strings.Contains(body, "Change the winter tyres") {
		t.Error("the title is not on the page")
	}
	if !strings.Contains(body, `href="https://example.com/tyres"`) {
		t.Error("the description's link is not followable")
	}
}

func TestACompletedProjectOpensWithNoFormOnIt(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped and balanced"},
		[]app.ActionFields{{Title: "Book the garage"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}

	body := getPage(t, s, "/project/1")

	for _, frag := range []string{`name="title"`, `name="meta"`, `name="dod"`, `action="/project/1"`} {
		if strings.Contains(body, frag) {
			t.Errorf("a completed project's page still carries %q", frag)
		}
	}
	for _, path := range []string{"/project/1/complete", "/project/1/delete", "/project/1/addaction"} {
		if strings.Contains(body, path) {
			t.Errorf("a completed project's page still offers %q", path)
		}
	}
	if !strings.Contains(body, "/project/1/uncomplete") {
		t.Error("a completed project cannot be brought back from its own page")
	}
	// what is kept is the whole thing: the DOD that was met and the actions it
	// was completed with
	if !strings.Contains(body, "All four swapped and balanced") {
		t.Error("the DOD is not on the page")
	}
	if !strings.Contains(body, "Book the garage") {
		t.Error("the actions it was completed with are not on the page")
	}
}

// The rows of a finished project's action list are marks, not controls: a
// check that posted /complete again would restamp the moment the work was
// done, and `c` on a row submits exactly that form.
func TestACompletedActionRowHasNoCheckboxToPress(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}, {Title: "Balance them"}})
	if err != nil {
		t.Fatal(err)
	}
	// one completed, one still open — the project's own page shows both
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/project/1")

	if strings.Contains(body, `action="/action/1/complete"`) {
		t.Error("the completed action's row still posts /complete")
	}
	if strings.Contains(body, `action="/action/1/pick"`) {
		t.Error("the completed action's row can still be picked for today")
	}
	// and the open one beside it is untouched
	if !strings.Contains(body, `action="/action/2/complete"`) {
		t.Error("the open action's row lost its checkbox")
	}
}

// The screens behind the removed controls are turned away too, so a bookmark
// or a stale tab does not open a form the app would refuse to act on.
func TestTheScreensBehindTheRemovedControlsTurnAway(t *testing.T) {
	s, a := newTestServer(t)
	act, err := a.CreateAction(0, app.ActionFields{Title: "Change the winter tyres"}, false)
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]string{
		"/action/1/promote":    "/action/1",
		"/project/1/addaction": "/project/1",
		"/doing/1":             "/next",
	} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusSeeOther {
			t.Errorf("GET %s: %d, want a redirect away", path, rec.Code)
			continue
		}
		if got := rec.Header().Get("Location"); got != want {
			t.Errorf("GET %s redirected to %q, want %q", path, got, want)
		}
	}
}
