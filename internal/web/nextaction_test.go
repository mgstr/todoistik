package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"todoistik/internal/app"
)

// refuse posts a form that the handler must not take, and hands back the page
// it answered with. A refusal is a redraw and not an error page: what was
// typed is on it, and the reason with it. What is checked here is that nothing
// was written — the callers check that, and that the page says why.
func refuse(t *testing.T, s *Server, path string, form url.Values) string {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code >= 300 && rec.Code < 400 {
		t.Fatalf("POST %s was accepted: %d", path, rec.Code)
	}
	return rec.Body.String()
}

// A project's page is its fields and then its plan: every open action as a row,
// in the order it was put in, with the one that is next marked where it stands
// (design.md, "The order of a plan is yours"; implementation.md, "The
// project's page"). These pin what the page draws and that every press on the
// plan is written as it is made.

func twoActionProject(t *testing.T, a *app.App) *app.Project {
	t.Helper()
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Picture hung", DOD: "On the wall"},
		[]app.ActionFields{{Title: "Buy the picture"}, {Title: "Borrow a drill"}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestThePlanIsOneListWithTheNextOneMarked(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	body := getPage(t, s, "/project/1")

	if !strings.Contains(body, `<h2 class="planhead">Actions</h2>`) {
		t.Errorf("the plan has no Actions heading: %s", body)
	}
	// both actions are rows, the next one among them, in the plan's order
	one := strings.Index(body, `href="/action/1" class="title"`)
	two := strings.Index(body, `href="/action/2" class="title"`)
	if one < 0 || two < 0 || one > two {
		t.Fatalf("the plan's rows are missing or out of order: %d %d", one, two)
	}
	// the next one is marked, and only it
	if n := strings.Count(body, `class="tonext isnext"`); n != 1 {
		t.Errorf("%d rows are marked next, want one", n)
	}
	if strings.Index(body, `class="tonext isnext"`) > one {
		t.Error("the mark is not on the first row")
	}
	// no open boxes for it, and nothing about the plan in the project's form:
	// the plan is written as you go, not on Save
	for _, gone := range []string{`name="atitle"`, `name="nextid"`, `data-drafts`, "More actions"} {
		if strings.Contains(body, gone) {
			t.Errorf("the page still carries %s", gone)
		}
	}
	// rows of the plan can be dragged, and moved and deleted by key
	for _, want := range []string{
		`data-plan="1" draggable="true"`,
		`action="/action/1/up" class="kb-up"`,
		`action="/action/1/down" class="kb-down"`,
		`action="/action/1/delete" class="kb-delete"`,
		`data-plan-title="Buy the picture"`,
		`data-reorder="/project/1/reorder"`,
		`data-live="/project/1/addaction"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	// with nothing completed there is no record to show
	if strings.Contains(body, "Completed actions") {
		t.Error("an empty Completed actions list is drawn")
	}
}

// One Done on the screen, and what it finishes is whatever is in front of
// you. A project with open work cannot be completed — internal/app answers
// ErrOpenActions — so the project's own Done is drawn only once there is
// nothing open left, which is exactly when the app would accept it.
func TestTheProjectsDoneIsOfferedOnlyWithNothingOpen(t *testing.T) {
	s, a := newTestServer(t)
	p := twoActionProject(t, a)

	body := getPage(t, s, "/project/1")
	if strings.Contains(body, `action="/project/1/complete"`) {
		t.Errorf("a project with open actions offers a Done the app refuses: %s", body)
	}
	if n := strings.Count(body, "kb-complete"); n != 2 {
		t.Errorf("%d completes on the page, want the two rows'", n)
	}
	// and no row is the screen's subject: with nothing selected, `d` has
	// nothing to press until the project can be finished
	if strings.Contains(body, "data-kb-subject") {
		t.Error("a row of the plan answers for the screen")
	}

	for _, act := range p.Actions {
		if err := a.CompleteAction(act.ID); err != nil {
			t.Fatal(err)
		}
	}
	body = getPage(t, s, "/project/1")
	if !strings.Contains(body, `action="/project/1/complete"`) {
		t.Errorf("a project with nothing left open does not offer Done: %s", body)
	}
	// and it is the only one: a completed row carries marks, not forms
	if n := strings.Count(body, "kb-complete"); n != 1 {
		t.Errorf("%d completes on the page, want only the project's", n)
	}
}

// The completed actions are their own list under the plan, most recently
// completed first, and none of them carries the next mark or the plan's keys.
func TestCompletedActionsAreTheirOwnListNewestFirst(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(app.ProjectFields{Title: "Picture hung", DOD: "On the wall"},
		[]app.ActionFields{{Title: "First done"}, {Title: "Second done"}, {Title: "Still open"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond) // stamps are to the second
	if err := a.CompleteAction(p.Actions[1].ID); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/project/1")
	i := strings.Index(body, "<h2>Completed actions</h2>")
	if i < 0 {
		t.Fatalf("no Completed actions list: %s", body)
	}
	if open := strings.Index(body, "Still open"); open > i {
		t.Error("an open action is under the completed ones")
	}
	rest := body[i:]
	first, second := strings.Index(rest, "First done"), strings.Index(rest, "Second done")
	if first < 0 || second < 0 || second > first {
		t.Errorf("the completed list is not newest first: %d %d", first, second)
	}
	for _, gone := range []string{"kb-next", "data-plan=", "kb-up", "isnext"} {
		if strings.Contains(rest[:strings.Index(rest, "</ul>")], gone) {
			t.Errorf("a completed row carries %s", gone)
		}
	}
}

// A refused Delete is the project's page again with the reason on it, not a
// bare line of text: the delete key submits natively, so a plain 400 would be
// the whole screen (implementation.md, "A refused post is never silent").
func TestARefusedProjectDeleteComesBackAsThePage(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)

	body := refuse(t, s, "/project/1/delete", url.Values{"back": {"/today"}})
	if !strings.Contains(body, `<p class="ask">`+app.ErrOpenActions.Error()+`</p>`) {
		t.Fatalf("the page does not say why the delete was refused: %s", body)
	}
	if !strings.Contains(body, `name="back" value="/today"`) {
		t.Error("the page forgot the view it was opened from")
	}
	if _, err := a.Project(1); err != nil {
		t.Errorf("the refused delete deleted the project: %v", err)
	}
}

// `n` on a row of the plan makes that action the project's next one, and the
// mark moves to it — one ▶, so the old one loses it (design.md, "Project"). It
// is offered on the rows that could take it and nowhere else.
func TestTheNextMarkMovesAndIsOnThePlansRowsOnly(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)

	body := getPage(t, s, "/project/1")
	if !strings.Contains(body, `action="/action/2/next" class="kb-next inline"`) {
		t.Errorf("a row of the plan carries no mark for making it next: %s", body)
	}
	// the action already next carries none: there is nothing for a press to do
	if strings.Contains(body, `action="/action/1/next"`) {
		t.Error("the next action is offered a key that would do nothing")
	}

	postForm(t, s, "/action/2/next", url.Values{})
	p, err := a.Project(1)
	if err != nil {
		t.Fatal(err)
	}
	if next := p.NextAction(a.Today()); next == nil || next.ID != 2 {
		t.Fatalf("the next action is %+v after the mark was pressed", next)
	}
	body = getPage(t, s, "/project/1")
	if n := strings.Count(body, `class="tonext isnext"`); n != 1 {
		t.Errorf("%d rows are marked next after the move, want one", n)
	}
	if !strings.Contains(body, `action="/action/1/next"`) {
		t.Error("the action that was next is not offered the mark again")
	}
	if strings.Contains(body, `action="/action/2/next"`) {
		t.Error("the new next action is still offered the mark")
	}

	// the views that are not plans carry no mark at all
	if body := getPage(t, s, "/next"); strings.Contains(body, "kb-next") {
		t.Error("the Next view offers the mark")
	}
	if body := getPage(t, s, "/tasks"); strings.Contains(body, "kb-next") {
		t.Error("Tasks offers the mark")
	}

	// a snoozed row holds the mark's place open and offers nothing
	sleep := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if err := a.SnoozeAction(1, sleep); err != nil {
		t.Fatal(err)
	}
	body = getPage(t, s, "/project/1")
	if strings.Contains(body, `action="/action/1/next"`) {
		t.Errorf("a snoozed row offers the mark: %s", body)
	}
	if !strings.Contains(body, `<span class="tonext gap">`) {
		t.Error("a row that cannot be made next does not keep the mark's place")
	}
}

// A project holding nothing but snoozed actions has no next action and cannot
// be completed either, so its page carries no Done and no mark — and no
// stalled ring, because it is waiting rather than stuck (design.md, "Stalled
// projects"). A project with nothing open at all is ringed, on the heading of
// the list the missing action would be added to.
func TestTheStalledRingIsOnTheActionsHeading(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(app.ProjectFields{Title: "Tyres changed", DOD: "On the car"},
		[]app.ActionFields{{Title: "Ring the fitter"}})
	if err != nil {
		t.Fatal(err)
	}
	sleep := time.Now().AddDate(0, 0, 7).Format("2006-01-02")
	if err := a.SnoozeAction(p.Actions[0].ID, sleep); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/project/1")
	if !strings.Contains(body, `<h2 class="planhead">Actions</h2>`) {
		t.Errorf("a project waiting on a snooze is ringed: %s", body)
	}
	if strings.Contains(body, "isnext") {
		t.Error("a snoozed action is marked next")
	}
	if strings.Contains(body, `action="/project/1/complete"`) {
		t.Error("a project with an open action offers a Done the app refuses")
	}

	stalledProject(t, a, "Winter tyres on")
	body = getPage(t, s, "/project/2")
	if !strings.Contains(body, `<h2 class="planhead stalled">Actions</h2>`) {
		t.Errorf("a stalled project is not ringed on its Actions heading: %s", body)
	}
}

// Save writes the project's own three fields, and nothing of the plan.
func TestSavingTheProjectPageWritesTheProject(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	if err := a.AddTag("home"); err != nil {
		t.Fatal(err)
	}
	postForm(t, s, "/project/1", url.Values{
		"title": {"Picture hung"}, "dod": {"On the wall, straight"}, "meta": {"#home"},
		"back": {"/projects"},
	})
	p, err := a.Project(1)
	if err != nil {
		t.Fatal(err)
	}
	if p.DOD != "On the wall, straight" || len(p.Tags) != 1 {
		t.Errorf("the project was not saved: %+v", p)
	}
	if len(p.Actions) != 2 || p.Actions[0].Title != "Buy the picture" {
		t.Errorf("saving the project touched its plan: %+v", p.Actions)
	}
	// a meta line the app cannot read refuses the save and says why
	body := refuse(t, s, "/project/1", url.Values{
		"title": {"Picture hung"}, "dod": {"changed"}, "meta": {"@nowhere"},
	})
	if !strings.Contains(body, `<p class="ask">`) {
		t.Errorf("the refusal does not say why: %s", body)
	}
	if p, _ = a.Project(1); p.DOD != "On the wall, straight" {
		t.Errorf("a refused save was written: %q", p.DOD)
	}
}

// The dialog writes as soon as it is confirmed: adding puts the action at the
// end of the plan, and editing saves over the row, each one an undo step of
// its own.
// The dialog is written for one project, so its line can take that project's
// tags: the mark reads the project's own box and writes the dialog's
// (implementation.md, "The project's page").
func TestTheDialogCopiesTheProjectsTags(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	body := getPage(t, s, "/project/1")
	dlg := body[strings.Index(body, `id="draft-dialog"`):]
	dlg = dlg[:strings.Index(dlg, `</dialog>`)]
	if !strings.Contains(dlg, `data-copy-meta="itemform" data-copy-into="meta"`) {
		t.Errorf("the add-action dialog has no project-tags mark: %s", dlg)
	}
}

func TestTheDialogAddsAndEditsAtOnce(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	if err := a.AddContext("errand", ""); err != nil {
		t.Fatal(err)
	}

	rec := postForm(t, s, "/project/1/addaction", url.Values{
		"title": {"Hang it"}, "meta": {"#short"}, "description": {""}, "back": {"/project/1"},
	})
	if got := rec.Header().Get("Location"); got != "/project/1" {
		t.Errorf("adding went to %q, want back to the project", got)
	}
	p, _ := a.Project(1)
	if plan := p.Plan(); len(plan) != 3 || plan[2].Title != "Hang it" || plan[2].Duration != "short" {
		t.Fatalf("the added action is not at the end of the plan: %+v", plan)
	}
	if step, err := a.LastUndo(); err != nil || step.Entries[0].Event != app.EvCreated {
		t.Errorf("adding was not an undo step: %+v %v", step, err)
	}

	postForm(t, s, "/action/1", url.Values{
		"title": {"Buy the picture at Selver"}, "meta": {"@errand"}, "description": {"40x50"},
		"back": {"/project/1"},
	})
	p, _ = a.Project(1)
	if got := p.Plan()[0]; got.Title != "Buy the picture at Selver" || got.Context != "errand" {
		t.Errorf("the edit was not saved over the row: %+v", got)
	}

	// a line the app cannot read comes back as the page, with nothing added
	body := refuse(t, s, "/project/1/addaction", url.Values{
		"title": {"Find the hammer"}, "meta": {"@nowhere"},
	})
	if !strings.Contains(body, `<p class="ask">`) {
		t.Errorf("a refused add does not say why: %s", body)
	}
	if p, _ = a.Project(1); len(p.Plan()) != 3 {
		t.Errorf("a refused add was written: %d rows", len(p.Plan()))
	}
}

// K and J move a row by one; a drag hands back the whole order, and a drag
// drawn on a page older than the plan is refused rather than half applied.
func TestThePlanIsReorderedFromThePage(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(app.ProjectFields{Title: "Picture hung", DOD: "On the wall"},
		[]app.ActionFields{{Title: "A"}, {Title: "B"}, {Title: "C"}})
	if err != nil {
		t.Fatal(err)
	}
	plan := func() string {
		got, _ := a.Project(p.ID)
		var out []string
		for _, o := range got.Plan() {
			out = append(out, o.Title)
		}
		return strings.Join(out, "")
	}
	postForm(t, s, "/action/3/up", url.Values{})
	if plan() != "ACB" {
		t.Fatalf("up gave %s", plan())
	}
	postForm(t, s, "/action/1/down", url.Values{})
	if plan() != "CAB" {
		t.Fatalf("down gave %s", plan())
	}
	postForm(t, s, "/project/1/reorder", url.Values{"order": {"2,3,1"}})
	if plan() != "BCA" {
		t.Fatalf("the drag gave %s", plan())
	}
	body := refuse(t, s, "/project/1/reorder", url.Values{"order": {"2,3"}})
	if !strings.Contains(body, "the plan has changed") {
		t.Errorf("a stale drag does not say why: %s", body)
	}
	if plan() != "BCA" {
		t.Errorf("a stale drag was written: %s", plan())
	}
}

// The Project branch of processing writes the first action in open boxes, and
// creates the project and that action in the one submit — a project not yet
// made has nowhere to write a row as you go.
func TestTheProjectBranchWritesItsFirstActionOpen(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Winter tyres on\nthe garage is on Pärnu mnt", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/process?item=1&as=project")
	if !strings.Contains(body, `<h2 class="nexthead">`) {
		t.Errorf("the project branch does not open its first action: %s", body)
	}
	// the capture's body goes to that action's description, where material a
	// project needs belongs — and it is in the box, not behind a dialog
	if !strings.Contains(body, ">the garage is on Pärnu mnt</textarea>") {
		t.Error("the capture's body is not in the first action's description box")
	}

	rec := postForm(t, s, "/process/1/project", url.Values{
		"title": {"Winter tyres on"}, "dod": {"All four swapped"}, "meta": {""},
		"atitle": {"Book the garage"}, "ameta": {"#today"},
		"adescription": {"the garage is on Pärnu mnt"},
	})
	if rec.Code >= 400 {
		t.Fatalf("the project was refused: %d %s", rec.Code, rec.Body.String())
	}
	p, err := a.Project(1)
	if err != nil {
		t.Fatal(err)
	}
	next := p.NextAction(a.Today())
	if next == nil || next.Title != "Book the garage" {
		t.Fatalf("the project has no first action: %+v", p)
	}
	// #today is the app's to manage and is applied once there is an action to
	// apply it to, which the open boxes do not change
	picked := false
	for _, tag := range next.Tags {
		if tag == app.TodayTag {
			picked = true
		}
	}
	if !picked {
		t.Errorf("#today on the first action was not applied: %+v", next)
	}
}
