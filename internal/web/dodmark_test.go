package web

import (
	"strings"
	"testing"

	"todoistik/internal/app"
)

// A project with no definition of done is in design.md's error state, and the
// box it is fixed in is marked (design.md, "Error state"). The mark is drawn
// from `data-dodcheck` by app.js, so what these pin is which screens carry the
// attribute: the project's own page, where an empty DOD is a project in error,
// and neither of the two screens that create one, where it is a required box
// nobody has typed in yet.

// dodBox is the Definition of done box as the page rendered it.
func dodBox(t *testing.T, body string) string {
	t.Helper()
	const open = `<textarea name="dod"`
	i := strings.Index(body, open)
	if i < 0 {
		t.Fatalf("the page has no Definition of done box")
	}
	rest := body[i:]
	return rest[:strings.Index(rest, ">")+1]
}

func TestAProjectWithNoDODMarksTheBox(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	// the state is reached by clearing the DOD of a project that had one, which
	// is the only way in: it cannot be created this way (design.md, "Error
	// state")
	if err := a.UpdateProject(1, app.ProjectFields{Title: "Winter tyres on"}); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/project/1")
	box := dodBox(t, body)
	if !strings.Contains(box, "data-dodcheck") {
		t.Errorf("the empty DOD box is not marked: %s", box)
	}
	if strings.Contains(box, "required") {
		t.Errorf("the DOD box refuses the save it must still allow: %s", box)
	}
	// the banner stays: it says what is wrong, the box says where
	if !strings.Contains(body, `class="error-banner">no definition of done`) {
		t.Errorf("the project in error carries no banner")
	}
}

// The mark is on the box wherever that box is an existing project's, empty or
// not — app.js reads it off what is typed, so a project that has a DOD still
// carries the attribute and simply wears no border.
func TestAProjectThatHasADODStillCarriesTheCheck(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	if box := dodBox(t, getPage(t, s, "/project/1")); !strings.Contains(box, "data-dodcheck") {
		t.Errorf("the project page's DOD box is not checked: %s", box)
	}
}

// Creating one is the other half: the DOD is `required` there, the create
// button is dead until it is filled in, and a red box on a form that has only
// just opened would be shouting about the state every new project starts in.
func TestCreatingAProjectDoesNotMarkTheEmptyDOD(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Winter-proof the car", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	act, err := a.CreateAction(0, app.ActionFields{Title: "Change the tyres"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/process?item=1&one=1&as=project",
		"/action/" + itoa(act.ID) + "/promote",
	} {
		box := dodBox(t, getPage(t, s, path))
		if strings.Contains(box, "data-dodcheck") {
			t.Errorf("GET %s marks a DOD nobody has typed in yet: %s", path, box)
		}
		if !strings.Contains(box, "required") {
			t.Errorf("GET %s does not require a DOD to create a project: %s", path, box)
		}
	}
}
