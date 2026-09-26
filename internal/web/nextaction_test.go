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

// refuse posts a form that is expected to be turned away, which postForm
// cannot: it fails the test on anything but a redirect.
// refuse posts a form that the handler must not take, and hands back the page
// it answered with. A refusal is a redraw and not an error page: the actions
// written into a project's list live in the form until the press is taken, so
// throwing the page away would throw away work that was nowhere else. What is
// checked here is that nothing was written — the callers check that, and that
// the page says why.
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

// A project is read and written through the one action at the head of its
// plan, and that action is open on the screen rather than a row to be pressed
// (design.md, "Projects"; implementation.md, "Writing a project"). These pin
// the four halves of that: which action it is, that the page's one Save writes
// the project and the action together, that an empty box makes the action when
// something is typed in it and does not when nothing is, and that the id the
// form carries is checked rather than trusted.

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

func TestTheNextActionIsOpenAndTheRestAreRows(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	body := getPage(t, s, "/project/1")

	// the first of the plan is in the boxes, named as the project form's own
	// action so that one Save can carry both
	if !strings.Contains(body, `name="atitle" value="Buy the picture"`) {
		t.Errorf("the next action is not open in the project's boxes: %s", body)
	}
	if !strings.Contains(body, `name="nextid" value="1"`) {
		t.Error("the form does not say which action the boxes are about")
	}
	// and it is not a row as well: one action, one place on the screen
	if strings.Contains(body, `href="/action/1" class="title"`) {
		t.Error("the next action is both open and listed")
	}
	// the one after it still is a row, under its own heading
	if !strings.Contains(body, `href="/action/2" class="title"`) {
		t.Error("the actions after the next one are not listed")
	}
	if !strings.Contains(body, "<h2>More actions</h2>") {
		t.Error("the rest of the plan has no heading of its own")
	}
	// the marks the row carried are on the heading, so completing the next
	// action is still one press from the project's page
	if !strings.Contains(body, `action="/action/1/complete"`) {
		t.Error("the next action cannot be completed from the project's page")
	}
	if !strings.Contains(body, `action="/action/1/pick"`) {
		t.Error("the next action cannot be picked for today from the project's page")
	}
	// it is that action's row for the keyboard — j/k reach it, `enter` opens
	// its page — and it says it is the screen's subject as well, so `d` and
	// `t` answer for it with nothing selected (keys.md, "What is built")
	if !strings.Contains(body, `data-kb-row data-kb-subject`) {
		t.Error("the next action heading is not the row the keyboard walks")
	}
	if !strings.Contains(body, `data-href="/action/1"`) {
		t.Error("the heading does not open the action's own page")
	}
}

// One Done on the screen, and what it finishes is whatever is in front of
// you. A project with open work cannot be completed — internal/app answers
// ErrOpenActions — so the project's own Done is drawn only once there is no
// next action left, which is exactly when the app would accept it.
func TestTheProjectsDoneIsOfferedOnlyWithNoNextAction(t *testing.T) {
	s, a := newTestServer(t)
	p := twoActionProject(t, a)

	body := getPage(t, s, "/project/1")
	if strings.Contains(body, `action="/project/1/complete"`) {
		t.Errorf("a project with a next action offers a Done the app refuses: %s", body)
	}
	// the one Done on the screen is the next action's, and it is the screen's
	// own because the heading says it is the subject
	if n := strings.Count(body, "kb-complete"); n != 2 {
		t.Errorf("%d completes on the page, want the next action's and the one open row's", n)
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

// The mark that moves the crown: `n` on a row of the plan makes that action the
// project's next one, and the page comes back with it in the boxes at the top
// (design.md, "Project"). It is offered on the rows that could take it and
// nowhere else — a snoozed action is not workable, and a list outside a plan has
// no project to be next of.
func TestTheNextMarkIsOnThePlansRowsOnly(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)

	body := getPage(t, s, "/project/1")
	if !strings.Contains(body, `action="/action/2/next" class="kb-next inline"`) {
		t.Errorf("a row of the plan carries no mark for making it next: %s", body)
	}
	// and the action already in the boxes carries none: it is next, so there is
	// nothing for a press to do and nothing is offered
	if strings.Contains(body, `action="/action/1/next"`) {
		t.Error("the next action is offered a key that would do nothing")
	}

	rec := postForm(t, s, "/action/2/next", url.Values{})
	if rec.Code >= 400 {
		t.Fatalf("the press was refused: %d %s", rec.Code, rec.Body.String())
	}
	p, err := a.Project(1)
	if err != nil {
		t.Fatal(err)
	}
	if next := p.NextAction(a.Today()); next == nil || next.ID != 2 {
		t.Fatalf("the next action is %+v after the mark was pressed", next)
	}
	body = getPage(t, s, "/project/1")
	if !strings.Contains(body, `name="atitle" value="Borrow a drill"`) {
		t.Errorf("the pointed-at action is not in the project's boxes: %s", body)
	}

	// the views that are not plans carry no mark at all: making an action next
	// is a claim about the order of one project's work, and the answer is
	// invisible from anywhere else (keys.md, `n`)
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
// be completed either, so its page carries no Done at all — and no stalled
// ring, because it is waiting rather than stuck (design.md, "Stalled projects").
func TestAProjectWaitingOnASnoozeHasEmptyBoxesAndNoRing(t *testing.T) {
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
	if !strings.Contains(body, `<h2 class="nexthead">`) {
		t.Errorf("the boxes are not empty: %s", body)
	}
	if strings.Contains(body, "nexthead stalled") {
		t.Error("a project waiting on a snooze is marked stalled")
	}
	if strings.Contains(body, `action="/project/1/complete"`) {
		t.Error("a project with an open action offers a Done the app refuses")
	}
}

// The third mark on the heading: the project's tags, brought down onto the
// action's line. It writes a box and posts nothing, so all the server decides
// is whether it can be pressed at all — which is whether the project has tags
// (implementation.md, "Writing a project").
func TestTheProjectTagsMarkIsOfferedOnlyWhenThereAreTags(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)

	body := getPage(t, s, "/project/1")
	if !strings.Contains(body, `data-copy-meta="itemform"`) {
		t.Errorf("the heading carries no mark for the project's tags: %s", body)
	}
	// nothing to bring down yet, so the mark is there and dead — the key bar
	// reads `disabled` and drops the key, which is the standing rule
	if !strings.Contains(body, `data-key="#" data-key-label="project tags"`) {
		t.Error("the mark does not declare its key")
	}
	i := strings.Index(body, "data-copy-meta")
	if j := strings.Index(body[i:], ">"); !strings.Contains(body[i:i+j], "disabled") {
		t.Errorf("a project with no tags offers a live mark: %s", body[i:i+j])
	}

	// give the project a tag and it comes alive
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if err := a.ToggleTag("project", 1, "car"); err != nil {
		t.Fatal(err)
	}
	body = getPage(t, s, "/project/1")
	i = strings.Index(body, "data-copy-meta")
	if j := strings.Index(body[i:], ">"); strings.Contains(body[i:i+j], "disabled") {
		t.Errorf("a project with a tag offers a dead mark: %s", body[i:i+j])
	}
	// it is a button and not a form: nothing is posted, the page's one Save
	// commits what it writes
	if strings.Contains(body, `action="/action/1/meta"`) {
		t.Error("the mark posts something")
	}
}

// The boxes belong to the project's form without sitting inside it: the two
// marks above them are forms of their own, and a form cannot be nested in a
// form.
func TestTheNextActionBoxesBelongToTheProjectForm(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	body := getPage(t, s, "/project/1")
	for _, want := range []string{
		`name="atitle" value="Buy the picture" required data-verbcheck form="itemform"`,
		`name="ameta"`,
		`name="adescription"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s: %s", want, body)
		}
	}
	// three boxes, the Save button — which has always been outside the form and
	// named it the same way — and the three hidden fields a row in the plan's
	// list carries. The <template> the key layer clones holds three more of
	// those, and they are not on the page until a row is made from it.
	live := body
	if i := strings.Index(live, "<template"); i >= 0 {
		live = live[:i]
	}
	if n := strings.Count(live, `form="itemform"`); n != 4 {
		t.Errorf("%d things are joined to the project form, want the three boxes and Save", n)
	}
}

func TestSavingTheProjectPageWritesBothHalves(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	// a name is notation only once the app knows it (design.md, "Writing an
	// action"), and both lines here are written in names
	if err := a.AddTag("home"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddContext("errand", ""); err != nil {
		t.Fatal(err)
	}
	rec := postForm(t, s, "/project/1", url.Values{
		"title": {"Picture hung"}, "dod": {"On the wall, straight"}, "meta": {"#home"},
		"nextid": {"1"}, "atitle": {"Buy the picture at Selver"},
		"ameta": {"@errand #short"}, "adescription": {"the 40x50 one"},
		"back": {"/projects"},
	})
	if rec.Code >= 400 {
		t.Fatalf("the save was refused: %d %s", rec.Code, rec.Body.String())
	}
	p, err := a.Project(1)
	if err != nil {
		t.Fatal(err)
	}
	if p.DOD != "On the wall, straight" {
		t.Errorf("the project's DOD was not saved: %q", p.DOD)
	}
	next := p.NextAction(a.Today())
	if next.Title != "Buy the picture at Selver" {
		t.Errorf("the next action's title was not saved: %q", next.Title)
	}
	if next.Context != "errand" || next.Duration != "short" {
		t.Errorf("the next action's meta line was not read: %+v", next)
	}
	if next.Description != "the 40x50 one" {
		t.Errorf("the next action's description was not saved: %q", next.Description)
	}
}

// A meta line the app cannot read refuses the whole press. Saving the project
// and dropping the action would leave half the screen written and say nothing
// about it (design.md, "Writing an action").
func TestARefusedMetaLineSavesNeitherHalf(t *testing.T) {
	s, a := newTestServer(t)
	twoActionProject(t, a)
	refuse(t, s, "/project/1", url.Values{
		"title": {"Picture hung"}, "dod": {"changed"}, "meta": {""},
		"nextid": {"1"}, "atitle": {"Buy the picture"}, "ameta": {"@nowhere"},
		"back": {"/projects"},
	})
	p, err := a.Project(1)
	if err != nil {
		t.Fatal(err)
	}
	if p.DOD != "On the wall" {
		t.Errorf("the project was written although the action was refused: %q", p.DOD)
	}
}

// A stalled project's boxes are empty, and saving with them empty is a save:
// the app never prevents a project from being stalled (design.md, "Stalled
// projects"). Typed in, the same boxes make the project's next action.
func TestTheEmptyBoxesMakeTheNextActionOrLeaveItAlone(t *testing.T) {
	s, a := newTestServer(t)
	stalledProject(t, a, "Winter tyres on")
	if err := a.AddContext("phone", ""); err != nil {
		t.Fatal(err)
	}

	rec := postForm(t, s, "/project/1", url.Values{
		"title": {"Winter tyres on"}, "dod": {"All four swapped, in the boot"},
		"meta": {""}, "nextid": {""}, "atitle": {""}, "ameta": {""}, "adescription": {""},
		"back": {"/projects"},
	})
	if rec.Code >= 400 {
		t.Fatalf("saving a stalled project was refused: %d %s", rec.Code, rec.Body.String())
	}
	p, err := a.Project(1)
	if err != nil {
		t.Fatal(err)
	}
	if p.DOD != "All four swapped, in the boot" {
		t.Errorf("the project was not saved: %q", p.DOD)
	}
	if n := len(p.OpenActions()); n != 0 {
		t.Errorf("an empty box invented %d actions", n)
	}
	if !p.ComputeStalled(a.Today()) {
		t.Error("the project stopped being stalled without an action being written")
	}

	rec = postForm(t, s, "/project/1", url.Values{
		"title": {"Winter tyres on"}, "dod": {"All four swapped, in the boot"},
		"meta": {""}, "nextid": {""}, "atitle": {"Book the garage"},
		"ameta": {"@phone"}, "adescription": {""}, "back": {"/projects"},
	})
	if rec.Code >= 400 {
		t.Fatalf("writing the next action was refused: %d %s", rec.Code, rec.Body.String())
	}
	if p, err = a.Project(1); err != nil {
		t.Fatal(err)
	}
	next := p.NextAction(a.Today())
	if next == nil || next.Title != "Book the garage" || next.Context != "phone" {
		t.Errorf("the box did not make the project's next action: %+v", next)
	}
}

// The id the form carries is checked against the project. A page drawn before
// the action was completed elsewhere must not write to it, and an id from
// somewhere else must not reach an action this project does not own.
func TestAStaleNextActionIsRefused(t *testing.T) {
	s, a := newTestServer(t)
	p := twoActionProject(t, a)
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	body := refuse(t, s, "/project/1", url.Values{
		"title": {"Picture hung"}, "dod": {"On the wall"}, "meta": {""},
		"nextid": {"1"}, "atitle": {"Buy the picture"}, "ameta": {""},
		"adescription": {""}, "back": {"/projects"},
	})
	// the page says so, in the words the handler used — HTML-escaped on the
	// way in, which is why the apostrophe is not looked for here
	if !strings.Contains(body, "no longer this project") {
		t.Errorf("the refusal must say why: %s", body)
	}
}

// The Project branch of processing writes the first action in the same open
// boxes, and creates the project and that action in the one submit.
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
