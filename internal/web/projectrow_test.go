package web

import (
	"strings"
	"testing"

	"todoistik/internal/app"
)

// The Project row is drawn when it names a project, and left off when it would
// only repeat the screen it is on (design.md, "Editing items"). It is the one
// field on the action form that is read rather than written, so a row with
// nothing to read is a row that costs a line and answers nothing — and the two
// cases it used to fill with a stand-in are exactly the two the screen already
// says out loud: a task, whose noun means standalone, and an action being added
// to the project whose form is open behind the dialog.

// projectRow is what the Project row reads on a page, or "" when the page has
// no such row. Both halves of the form are looked for — the box an open item is
// written in and the text a finished one reads as.
func projectRow(t *testing.T, body string) string {
	t.Helper()
	const lb = `<span class="lb">Project</span> `
	i := strings.Index(body, lb)
	if i < 0 {
		return ""
	}
	rest := body[i+len(lb):]
	if strings.HasPrefix(rest, `<input class="pickerbox"`) {
		const v = `value="`
		j := strings.Index(rest, v)
		k := strings.Index(rest[j+len(v):], `"`)
		return rest[j+len(v) : j+len(v)+k]
	}
	const open, shut = `<span class="val muted">`, `</span>`
	if j := strings.Index(rest, open); j == 0 {
		return rest[len(open) : len(open)+strings.Index(rest[len(open):], shut)]
	}
	t.Fatalf("the Project label is on the page but nothing readable follows it: %.80s", rest)
	return ""
}

// An action inside a project names it, open and finished alike: that is the one
// thing the row is for, and the page it is on says nothing else about where the
// action lives.
func TestAnActionsPageNamesItsProject(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	id := p.Actions[0].ID
	if got := projectRow(t, pageFrom(t, s, "/action/"+itoa(id), "/project/"+itoa(p.ID))); got != "Winter-proof the car" {
		t.Errorf("the action's page says its project is %q, want the project's title", got)
	}
	if err := a.CompleteAction(id); err != nil {
		t.Fatal(err)
	}
	if got := projectRow(t, pageFrom(t, s, "/action/"+itoa(id), "/archive")); got != "Winter-proof the car" {
		t.Errorf("the finished action reads its project as %q, want the project's title", got)
	}
}

// A task has no project, and the row is gone rather than filled in with a word
// standing in for one. The trail says "task" over both halves of the screen,
// which is where that fact belongs (design.md, "Tasks").
func TestATasksPageHasNoProjectRow(t *testing.T) {
	s, a := newTestServer(t)
	act, err := a.CreateAction(0, app.ActionFields{Title: "Pay the rent"})
	if err != nil {
		t.Fatal(err)
	}
	if got := projectRow(t, pageFrom(t, s, "/action/"+itoa(act.ID), "/tasks")); got != "" {
		t.Errorf("a task's page carries a Project row reading %q", got)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	if got := projectRow(t, pageFrom(t, s, "/action/"+itoa(act.ID), "/archive")); got != "" {
		t.Errorf("a finished task reads a Project row saying %q", got)
	}
}

// The add-action dialog has none either, and neither does the next action open
// on the same form: one project is in play on that screen, and the form around
// the dialog is the whole of the answer.
func TestWritingAProjectAsksForNoProjectRow(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/project/"+itoa(p.ID))
	if !strings.Contains(body, `id="draft-dialog"`) {
		t.Fatal("the project page has no add-action dialog to check")
	}
	if got := projectRow(t, body); got != "" {
		t.Errorf("writing a project asks which project, reading %q", got)
	}
}
