package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// An action may wait on a sibling instead of on a date, and a project's action
// list is the one list that draws what that means: the waiting action sits
// under what it waits on (design.md, "Time fields"; implementation.md,
// "Writing a project").

func pictureProject(t *testing.T, a *app.App) (*app.Project, *app.Action) {
	t.Helper()
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Picture hung", DOD: "On the wall"},
		[]app.ActionFields{{Title: "Buy the picture"}})
	if err != nil {
		t.Fatal(err)
	}
	return p, p.Actions[0]
}

func TestProjectPageNestsAWaitingAction(t *testing.T) {
	s, a := newTestServer(t)
	p, buy := pictureProject(t, a)
	if _, err := a.CreateAction(p.ID, app.ActionFields{
		Title: "Hang it", SnoozeActionID: buy.ID,
	}); err != nil {
		t.Fatal(err)
	}

	body := getPage(t, s, "/project/1")
	if !strings.Contains(body, `data-depth="1"`) {
		t.Errorf("the waiting action is not drawn under what it waits on: %s", body)
	}
	// the badge names the action rather than a date, because that is what the
	// waiting is actually about
	if !strings.Contains(body, "zzz until Buy the picture") {
		t.Errorf("the row does not say what it is waiting on: %s", body)
	}
	// and it is not in Next actions, the one view a snoozed action is left out of
	if body := getPage(t, s, "/next"); strings.Contains(body, "Hang it") {
		t.Errorf("a waiting action is not an answer to what do I do next: %s", body)
	}
	// nothing offers to park it any more: the meta line is where the waiting
	// is written, and there is no state to toggle beside it
	if body := getPage(t, s, "/action/2"); strings.Contains(body, "data-key-label=\"parked\"") {
		t.Errorf("the action page still carries a Parked button: %s", body)
	}
}

// The meta line writes the waiting, in either spelling, and reads back as the
// title whichever way it was typed.
func TestMetaLineWritesTheWaiting(t *testing.T) {
	s, a := newTestServer(t)
	p, buy := pictureProject(t, a)
	hang, err := a.CreateAction(p.ID, app.ActionFields{Title: "Hang it"})
	if err != nil {
		t.Fatal(err)
	}

	for _, meta := range []string{"snooze:(buy the picture)", "snooze:#" + itoa(buy.ID)} {
		if err := a.SnoozeAction(hang.ID, ""); err != nil {
			t.Fatal(err)
		}
		rec := postForm(t, s, "/action/2", url.Values{
			"title": {"Hang it"}, "meta": {meta}, "back": {"/project/1"},
		})
		if rec.Code >= 400 {
			t.Fatalf("%s refused: %d %s", meta, rec.Code, rec.Body.String())
		}
		got, err := a.Action(hang.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.SnoozeActionID != buy.ID {
			t.Fatalf("%s did not set the waiting: %+v", meta, got)
		}
		if line := got.Meta(); line != "snooze:(Buy the picture)" {
			t.Fatalf("%s reads back as %q", meta, line)
		}
	}

	// a name that matches no sibling is refused by name rather than dropped —
	// swallowing it would leave the action looking workable just after you
	// said it was not
	r := httptest.NewRequest(http.MethodPost, "/action/2", strings.NewReader(url.Values{
		"title": {"Hang it"}, "meta": {"snooze:(paint the wall)"}, "back": {"/project/1"},
	}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code < 400 {
		t.Fatalf("a snooze naming nothing must be refused: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "matches no open action") {
		t.Fatalf("the refusal must say why: %s", rec.Body.String())
	}
}
