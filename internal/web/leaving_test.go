package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// Resolving an item leaves its page (design.md, "Editing items"). Completing
// used to post nothing but the verb, so back() fell to the Referer — the page
// the press was on — and a completed item came back as its own frozen record
// with the press looking like it had failed. These pin where each resolution
// lands instead.

func postForm(t *testing.T, s *Server, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST %s: %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec
}

// formOn is the one form on a page that posts to the given path, so a test
// about what a button carries is not also a test about how the page is
// indented.
func formOn(t *testing.T, body, action string) string {
	t.Helper()
	i := strings.Index(body, `action="`+action+`"`)
	if i < 0 {
		t.Fatalf("no form posting to %s", action)
	}
	j := strings.Index(body[i:], "</form>")
	if j < 0 {
		t.Fatalf("the form posting to %s is never closed", action)
	}
	return body[i : i+j]
}

func TestCompletingAnActionFromItsPageGoesWhereThePageWasOpenedFrom(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Change the winter tyres"}, false); err != nil {
		t.Fatal(err)
	}
	// the page carries where it was opened from, the way Save and Delete do
	if form := formOn(t, getPage(t, s, "/action/1?from=%2Ftasks"), "/action/1/complete"); !strings.Contains(
		form, `name="back" value="/tasks"`) {
		t.Errorf("the Complete form does not say where to go: %s", form)
	}
	rec := postForm(t, s, "/action/1/complete", url.Values{"back": {"/tasks"}})
	if got := rec.Header().Get("Location"); got != "/tasks" {
		t.Errorf("completing landed on %q, want the view it was opened from", got)
	}
}

// A standalone action has nothing to ask about, but one inside a project can
// leave it with no next action — and then the ask is the way out, carrying the
// destination on so the screen after it still knows where you came from.
func TestTheProjectAskCarriesTheWayOutOn(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	rec := postForm(t, s, "/action/1/complete", url.Values{"back": {"/next"}})
	if got, want := rec.Header().Get("Location"), "/project/1?ask=1&from=%2Fnext"; got != want {
		t.Errorf("the ask opened at %q, want %q", got, want)
	}
	// and the screen it opens posts that destination from its own button
	if form := formOn(t, getPage(t, s, "/project/1?ask=1&from=%2Fnext"), "/project/1/complete"); !strings.Contains(
		form, `name="back" value="/next"`) {
		t.Errorf("the ask's Complete-the-project form does not say where to go: %s", form)
	}
}

// An action opened from its own project comes back to it. Handing that on as
// the ask's way out would make the screen its own exit.
func TestTheAskIsNeverItsOwnWayOut(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	rec := postForm(t, s, "/action/1/complete", url.Values{"back": {"/project/1"}})
	if got, want := rec.Header().Get("Location"), "/project/1?ask=1"; got != want {
		t.Errorf("the ask opened at %q, want %q", got, want)
	}
}

func TestResolvingAProjectLeavesItsPage(t *testing.T) {
	s, a := newTestServer(t)
	for _, title := range []string{"Winter tyres on", "Repaint the hallway"} {
		p, err := a.CreateProject(
			app.ProjectFields{Title: title, DOD: "done is done"},
			[]app.ActionFields{{Title: "first step"}})
		if err != nil {
			t.Fatal(err)
		}
		// a project with an open action can be neither completed nor deleted
		if err := a.CompleteAction(p.Actions[0].ID); err != nil {
			t.Fatal(err)
		}
	}

	rec := postForm(t, s, "/project/1/complete", url.Values{"back": {"/calendar"}})
	if got := rec.Header().Get("Location"); got != "/calendar" {
		t.Errorf("completing a project landed on %q, want the view it was opened from", got)
	}
	rec = postForm(t, s, "/project/2/delete", url.Values{"back": {"/calendar"}})
	if got := rec.Header().Get("Location"); got != "/calendar" {
		t.Errorf("deleting a project landed on %q, want the view it was opened from", got)
	}
}

// With nothing posted it is the pile the project was in, and not the Referer:
// back() would pick the page of the project that has just stopped being one.
func TestAResolvedProjectFallsBackToTheProjects(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	rec := postForm(t, s, "/project/1/complete", nil)
	if got := rec.Header().Get("Location"); got != "/projects" {
		t.Errorf("completing a project with nowhere to go landed on %q, want /projects", got)
	}
}
