package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// The verb check is drawn in the browser (see app.js, "The verb a title opens
// with"), so what the server owes it is two things and these are both: the
// list has to reach every page that holds a title box, and the box has to be
// marked as the one that gets checked. Either missing is a check that simply
// never fires, which is the kind of broken that looks like working.

func TestTheVerbListReachesTheTitleBox(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/project/1/addaction")
	if !strings.Contains(body, `data-verbs="`) {
		t.Error("the page carries no verb list, so nothing can be checked against it")
	}
	if !strings.Contains(body, "call ") {
		t.Error("the seeded verbs are not on the page")
	}
	if !strings.Contains(body, `name="title" value="" required data-verbcheck`) {
		t.Error("the title box is not marked as the one the verb check reads")
	}
}

// A project's title is a reference to an outcome and deliberately not a thing
// to do (design.md, "Inbox Zero", the Project branch). Marking it would be the
// app asking for the one title it does not want.
func TestAProjectTitleIsNotVerbChecked(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/project/1")
	if strings.Contains(body, `name="title" value="Winter tyres on" required data-verbcheck`) {
		t.Error("a project's title is verb-checked, and a project title is an outcome")
	}
}

// The review is the one list that says anything about a title (design.md,
// "Verbs"): the walk is where the titles are read rather than acted on. Every
// other list stays quiet, including the one the same actions appear on.
func TestOnlyTheReviewMarksARow(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateAction(0, app.ActionFields{Title: "milk"}, false); err != nil {
		t.Fatal(err)
	}
	if body := getPage(t, s, "/review/next"); !strings.Contains(body, `class="badge noverb" hidden`) {
		t.Error("a review row carries no verb badge, so the walk can say nothing about the title")
	}
	if body := getPage(t, s, "/next"); strings.Contains(body, "noverb") {
		t.Error("Next actions marks titles, and working from a list is not where that belongs")
	}
}

// A project has no verb to miss, and neither has a someday item or a schedule
// — so the badge is not merely hidden on those rows, it is absent.
func TestTheReviewMarksActionsOnly(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]app.ActionFields{{Title: "Book the garage"}}); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/review/projects")
	if strings.Contains(body, "noverb") || strings.Contains(body, "data-verbcheck") {
		t.Error("a project row is verb-checked in the review, and a project title is an outcome")
	}
}

func TestSettingsKeepsTheVerbList(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddVerb("позвонить"); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/settings")
	if !strings.Contains(body, "позвонить") {
		t.Error("the verb list is not on the Settings screen, which is where it is kept")
	}
	if !strings.Contains(body, `action="/settings/verbs/remove"`) {
		t.Error("a verb has no way off the list")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/settings/verbs/remove",
		strings.NewReader(url.Values{"word": {"позвонить"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("removing a verb answered %d", rec.Code)
	}
	if vs, _ := a.Verbs(); hasWord(vs, "позвонить") {
		t.Error("the verb is still on the list after being removed")
	}
}

func hasWord(list []string, w string) bool {
	for _, s := range list {
		if s == w {
			return true
		}
	}
	return false
}
