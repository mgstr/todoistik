package web

import (
	"strings"
	"testing"

	"todoistik/internal/app"
)

// A stalled project says so once, on the heading of the list the next action
// is missing from (implementation.md, "Writing a project"). It used to say it
// twice — a banner across the top and, if you had arrived by completing the
// last action, a panel repeating it in a sentence — which is the thing these
// pin down as gone.

func stalledProject(t *testing.T, a *app.App, title string) *app.Project {
	t.Helper()
	p, err := a.CreateProject(
		app.ProjectFields{Title: title, DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}})
	if err != nil {
		t.Fatal(err)
	}
	// the one action gone is what leaves the project with no next action
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAStalledProjectMarksItsActionsHeading(t *testing.T) {
	s, a := newTestServer(t)
	stalledProject(t, a, "Winter tyres on")
	body := getPage(t, s, "/project/1")
	if !strings.Contains(body, `<h2 class="stalled">Actions</h2>`) {
		t.Errorf("the Actions heading is not marked on a stalled project")
	}
	if strings.Contains(body, "error-banner") {
		t.Errorf("a stalled project still carries a banner: %s", body)
	}
	if strings.Contains(body, "No next action left") {
		t.Errorf("the page still says there is no next action in prose")
	}
}

func TestAProjectWithANextActionDoesNotMarkItsHeading(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	if body := getPage(t, s, "/project/1"); !strings.Contains(body, "<h2>Actions</h2>") {
		t.Errorf("the Actions heading is marked on a project that is not stalled")
	}
}

// The keyboard hands the selected row from one page to the next, and only the
// list it came from may claim it. A view's name does not say which list that
// is — the project page opened from Next calls itself `next` too — so the pane
// says whether this is the view's own list or a screen inside it. Without it
// the cursor handed on by completing an action landed on the project's action
// list, on the action just completed, and took the screen's own Done off the
// key bar (app.js, viewKey).
func TestAScreenInsideAViewSaysSoOnItsPane(t *testing.T) {
	s, a := newTestServer(t)
	stalledProject(t, a, "Winter tyres on")
	if body := getPage(t, s, "/next"); strings.Contains(body, "data-step") {
		t.Errorf("the Next list calls itself a screen inside a view")
	}
	if body := getPage(t, s, "/project/1?from=%2Fnext"); !strings.Contains(body, "data-step") {
		t.Errorf("the project page does not say it is a screen inside Next")
	}
}

// The page is the same page however it was reached: completing the last
// action lands on it with nothing in the address saying so, and it still
// offers Done — the one Done, in the action bar, which `d` presses.
func TestTheStalledPageOffersDoneWhateverTheWayIn(t *testing.T) {
	s, a := newTestServer(t)
	stalledProject(t, a, "Winter tyres on")
	for _, path := range []string{"/project/1", "/project/1?from=%2Fnext"} {
		body := getPage(t, s, path)
		if n := strings.Count(body, `action="/project/1/complete"`); n != 1 {
			t.Errorf("%s offers %d ways to complete the project, want exactly one", path, n)
		}
		if !strings.Contains(body, `class="inline kb-complete"`) {
			t.Errorf("%s does not offer the keyboard's Done", path)
		}
	}
}
