package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// The title bar is a path from the view to where you are, and the Back button
// walks it one step at a time (design.md, "Panels"). Both are built from the
// same address, so these pin the two together: a trail that names a screen the
// way out does not go to is a trail that is describing a different app.

// trail is the title bar's steps, in order, as the page rendered them.
func trail(t *testing.T, body string) []string {
	t.Helper()
	const open, shut = `<span class="crumb">`, `</span>`
	var out []string
	rest := body
	for {
		i := strings.Index(rest, open)
		if i < 0 {
			return out
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, shut)
		if j < 0 {
			return out
		}
		// a crumb may carry a count badge; the name is what comes before it
		name := rest[:j]
		if k := strings.Index(name, "<span"); k >= 0 {
			name = name[:k]
		}
		out = append(out, strings.TrimSpace(name))
		rest = rest[j:]
	}
}

// pageFrom is a page opened from somewhere, which is how every detail screen
// is actually reached: the Referer is what says which screen you pressed a row
// on (see parentView).
func pageFrom(t *testing.T, s *Server, path, from string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("Referer", "http://localhost"+from)
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s from %s: %d", path, from, rec.Code)
	}
	return rec.Body.String()
}

// want compares the bar against the steps a case names.
func want(t *testing.T, got []string, steps ...string) {
	t.Helper()
	if strings.Join(got, " / ") != strings.Join(steps, " / ") {
		t.Errorf("the title bar reads %q, want %q",
			strings.Join(got, " / "), strings.Join(steps, " / "))
	}
}

// The window and the bar say the same path, and only the window starts it at
// the app: the name is what tells one window from another, and inside the app
// it told no screen from any other (design.md, "Panels"). They are drawn from
// one trail, and this is what says the window's is the bar's with the app in
// front and nothing else different.
func TestTheWindowStartsAtTheAppAndTheBarDoesNot(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ path, want string }{
		{"/someday", "Someday/Maybe"},
		{"/process?item=1&one=1", "Inbox / Processing"},
	} {
		body := getPage(t, s, c.path)
		if got := docTitle(t, body); got != "todoistik / "+c.want {
			t.Errorf("GET %s: the window says %q, want %q", c.path, got, "todoistik / "+c.want)
		}
		if got := strings.Join(trail(t, body), " / "); got != c.want {
			t.Errorf("GET %s: the title bar says %q, want %q", c.path, got, c.want)
		}
	}
}

// The view's count is one number until a filter hides part of the view, and
// then it is how many are on the screen of how many the view holds — a pair
// that includes "0 / 2", which is the answer a filter matching nothing gives
// and the one a count that is simply not written would have swallowed
// (design.md, "Panels").
func TestTheBarCountsAgainstTheFilter(t *testing.T) {
	s, a := newTestServer(t)
	for _, title := range []string{"Pay the rent", "Call the bank"} {
		if _, err := a.CreateAction(0, app.ActionFields{Title: title}); err != nil {
			t.Fatal(err)
		}
	}
	const badge = `Tasks<span class="badge navcount`
	for _, c := range []struct{ path, want string }{
		{"/tasks?f=1&q=", ` ">2</span>`},
		{"/tasks?f=1&q=rent", `">1 / 2</span>`},
		{"/tasks?f=1&q=nothing-is-called-this", `">0 / 2</span>`},
	} {
		body := getPage(t, s, c.path)
		if !strings.Contains(body, badge+c.want) {
			i := strings.Index(body, `<header id="titlebar">`)
			j := strings.Index(body, `</header>`)
			t.Errorf("GET %s: the title bar is %q, want the count %q", c.path, body[i:j], c.want)
		}
	}
}

// ...and an item opened from the filtered list goes on saying the pair: the
// filter is still on the view, and it is what Back returns to. A bar that
// dropped to the one number a step in said the filter had been cleared.
func TestTheBarKeepsThePairOnAnItemOfAFilteredView(t *testing.T) {
	s, a := newTestServer(t)
	var rent int64
	for _, title := range []string{"Pay the rent", "Call the bank"} {
		act, err := a.CreateAction(0, app.ActionFields{Title: title})
		if err != nil {
			t.Fatal(err)
		}
		if rent == 0 {
			rent = act.ID
		}
	}
	const pair = `Tasks<span class="badge navcount">1 / 2</span>`
	getPage(t, s, "/tasks?f=1&q=rent")
	if body := pageFrom(t, s, "/action/"+itoa(rent), "/tasks"); !strings.Contains(body, pair) {
		t.Error("an item opened from a filtered view does not say how much of the view is showing")
	}
	getPage(t, s, "/tasks?f=1&q=")
	if body := pageFrom(t, s, "/action/"+itoa(rent), "/tasks"); strings.Contains(body, pair) {
		t.Error("the pair outlived the filter it was counting")
	}
}

// docTitle is what the window shows, as the page rendered it.
func docTitle(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, "<title>")
	j := strings.Index(body, "</title>")
	if i < 0 || j < i {
		t.Fatal("the page has no title")
	}
	return body[i+len("<title>") : j]
}

// An action opened from its project is two screens deep, and the trail says
// both: the project's page is where Back goes, so it is a step and not a hop
// that happened invisibly.
func TestAnActionUnderAProjectNamesTheProject(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	body := pageFrom(t, s, "/action/"+itoa(p.Actions[0].ID), "/project/"+itoa(p.ID))
	want(t, trail(t, body), "Projects", "Edit project", "Edit action")
	if !strings.Contains(body, `href="/project/`+itoa(p.ID)+`" class="button"`) &&
		!strings.Contains(body, `<a class="button" href="/project/`+itoa(p.ID)+`">Back</a>`) {
		t.Error("Back does not go to the project the trail names")
	}
}

// A standalone action is a task, and every screen about one says so: the noun
// is the whole of what distinguishes it from an action (design.md, "Tasks").
func TestAStandaloneActionIsATaskInTheTrail(t *testing.T) {
	s, a := newTestServer(t)
	act, err := a.CreateAction(0, app.ActionFields{Title: "Pay the rent"})
	if err != nil {
		t.Fatal(err)
	}
	want(t, trail(t, pageFrom(t, s, "/action/"+itoa(act.ID), "/tasks")), "Tasks", "Edit task")

	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	want(t, trail(t, pageFrom(t, s, "/action/"+itoa(act.ID), "/archive")), "Archive", "Completed task")
}

// The processing branches, and the path each of them opens. Action is the one
// with a screen between the question and the form, because it is the one with
// a second question to ask.
func TestTheProcessingBranchesEachNameTheirPath(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	item := "/process?item=1&one=1"

	want(t, trail(t, getPage(t, s, item)), "Inbox", "Processing")
	want(t, trail(t, getPage(t, s, item+"&as=task")), "Inbox", "Processing", "Create task")
	want(t, trail(t, getPage(t, s, item+"&as=action")), "Inbox", "Processing", "Pick project")
	want(t, trail(t, getPage(t, s, item+"&as=action&project="+itoa(p.ID))),
		"Inbox", "Processing", "Pick project", "Create action")
	want(t, trail(t, getPage(t, s, item+"&as=project")), "Inbox", "Processing", "Create project")
	want(t, trail(t, getPage(t, s, item+"&as=someday")), "Inbox", "Processing", "Create someday")
}

// Back is one step of that path and not a jump to the beginning: from the
// action form it is the picker, and from the picker it is the question.
func TestBackFromTheActionFormIsThePicker(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	form := getPage(t, s, "/process?item=1&one=1&as=action&project="+itoa(p.ID))
	if !strings.Contains(form, `data-cancel="/process?item=1&amp;as=action&amp;one=1"`) {
		t.Error("the action form does not go back to the picker it was chosen on")
	}
	pick := getPage(t, s, "/process?item=1&one=1&as=action")
	if !strings.Contains(pick, `data-cancel="/process?item=1&amp;one=1"`) {
		t.Error("the picker does not go back to the question")
	}
}

// Task and Action are two answers, and the difference between them is settled
// before either form is drawn: the Action form's Project box shows the answer
// and is not a control, and the Task form has no such box, because a task has
// no project to name (design.md, "Inbox Zero").
func TestTaskAndActionSettleTheProjectBeforeTheForm(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}

	stage1 := getPage(t, s, "/process?item=1&one=1")
	for _, key := range []string{`data-key="t" data-key-label="task"`, `data-key="a" data-key-label="action"`} {
		if !strings.Contains(stage1, key) {
			t.Errorf("the question does not offer %s", key)
		}
	}

	task := getPage(t, s, "/process?item=1&one=1&as=task")
	if strings.Contains(task, "pickerbox") {
		t.Error("the Task form still has a Project row, which a task has nothing to put in")
	}
	if strings.Contains(task, "pickerlist") {
		t.Error("the Task form still asks which project, after the answer was given")
	}

	pick := getPage(t, s, "/process?item=1&one=1&as=action")
	if !strings.Contains(pick, "Winter-proof the car") {
		t.Error("the picker does not list the active project")
	}
	if !strings.Contains(pick, "data-newproject") {
		t.Error("the picker has no way to name a project that does not exist yet")
	}

	form := getPage(t, s, "/process?item=1&one=1&as=action&project="+itoa(p.ID))
	if !strings.Contains(form, `value="Winter-proof the car"`) {
		t.Error("the action form does not show the project that was picked")
	}
	if !strings.Contains(form, `name="projectid" value="`+itoa(p.ID)+`"`) {
		t.Error("the action form does not carry the picked project to the Create")
	}
	if !strings.Contains(form, `name="as" value="action"`) {
		t.Error("the action form does not say which branch it is, so a refusal cannot come back")
	}
}

// A project that is finished is not a target, and neither is one that does not
// exist. Both leave the question unanswered, so the picker is drawn rather
// than a form filed into nothing.
func TestAnImpossibleProjectFallsBackToThePicker(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	for _, pid := range []string{itoa(p.ID), "999"} {
		body := getPage(t, s, "/process?item=1&one=1&as=action&project="+pid)
		want(t, trail(t, body), "Inbox", "Processing", "Pick project")
	}
}

// A project that does not exist yet is answered on the picker and carried to
// the form as two fields, because until the Create there is nothing to make it
// out of (design.md, "Inbox Zero").
func TestAPendingProjectRidesToTheForm(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s,
		"/process?item=1&one=1&as=action&newproject=Winter-proof+the+car&newdod=it+starts")
	want(t, trail(t, body), "Inbox", "Processing", "Pick project", "Create action")
	// the leading + says it is not a project yet; html/template writes it as
	// an entity in an attribute, which is what the page actually carries
	if !strings.Contains(body, `value="&#43; Winter-proof the car"`) {
		t.Error("the form does not say which project is about to be made")
	}
	if !strings.Contains(body, `name="newproject" value="Winter-proof the car"`) ||
		!strings.Contains(body, `name="newdod" value="it starts"`) {
		t.Error("the pending project is not carried to the Create that writes it")
	}
}

// A refused form comes back as the form it was, project and all: the answer
// given two screens ago is not thrown away with the words that were typed.
func TestARefusedActionComesBackWithItsProject(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	form := strings.NewReader(
		"as=action&projectid=" + itoa(p.ID) + "&title=Book+it&meta=%40nowhere&description=")
	req := httptest.NewRequest(http.MethodPost, "/process/1/action?one=1", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	want(t, trail(t, body), "Inbox", "Processing", "Pick project", "Create action")
	if !strings.Contains(body, `value="Winter-proof the car"`) {
		t.Error("the bounced form forgot which project the action was being filed into")
	}
}

// The rest of the map, one line each: every screen that sits under a view says
// which screen it is, and the two that used to put an item's own text in the
// crumb now say what they are like everything else.
func TestEveryDetailScreenNamesItself(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	task, err := a.CreateAction(0, app.ActionFields{Title: "Pay the rent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSchedule("Water the plants", "* * 1", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Learn to sail", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProcessSomeday(1, app.SomedayFields{Text: "Learn to sail"}); err != nil {
		t.Fatal(err)
	}

	want(t, trail(t, getPage(t, s, "/project/"+itoa(p.ID)+"/addaction")),
		"Projects", "Edit project", "Create action")
	want(t, trail(t, pageFrom(t, s, "/action/"+itoa(task.ID)+"/promote", "/tasks")),
		"Tasks", "Edit task", "Promote task to project")
	want(t, trail(t, getPage(t, s, "/schedule/new")), "Scheduler", "Create scheduler")
	want(t, trail(t, getPage(t, s, "/schedule/1")), "Scheduler", "Edit scheduler")
	want(t, trail(t, getPage(t, s, "/somedayitem/1")), "Someday/Maybe", "Edit someday")
	want(t, trail(t, getPage(t, s, "/doing/"+itoa(task.ID)+"?from=/next")),
		"Next actions", "Doing")
}
