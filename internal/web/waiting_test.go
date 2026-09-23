package web

import (
	"html"
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

// The meta box completes a `snooze:` from the project's other open actions, so
// the list has to reach the box. It hangs on the box rather than on the pane
// because it is the one completion list that differs per screen
// (implementation.md, "Token boxes").
func TestTheMetaBoxCarriesTheActionsASnoozeMayName(t *testing.T) {
	s, a := newTestServer(t)
	p, buy := pictureProject(t, a)
	hang, err := a.CreateAction(p.ID, app.ActionFields{Title: "Hang it"})
	if err != nil {
		t.Fatal(err)
	}

	body := getPage(t, s, "/action/"+itoa(hang.ID))
	if !strings.Contains(body, "data-siblings=") {
		t.Fatalf("the box carries no completion list: %s", body)
	}
	if !strings.Contains(body, "Buy the picture") {
		t.Errorf("the sibling is not offered: %s", body)
	}
	// itself is never on the list: an action cannot wait on itself, and a
	// completion that wrote a refusal would be worse than none
	sibs := attrValue(t, body, "data-siblings")
	if strings.Contains(sibs, "Hang it") {
		t.Errorf("the action being edited is on its own completion list: %s", sibs)
	}
	if !strings.Contains(sibs, itoa(buy.ID)) {
		t.Errorf("the list carries no ids, so an ambiguous title has no way out: %s", sibs)
	}

	// a standalone action has no plan to be ordered inside, so no list at all
	loose, _ := a.CreateAction(0, app.ActionFields{Title: "Pay the rent"})
	if body := getPage(t, s, "/action/"+itoa(loose.ID)); strings.Contains(body, "data-siblings=") {
		t.Errorf("a standalone action offers siblings it cannot have: %s", body)
	}

	// and the add-action screen offers every open action, there being no
	// action yet for one of them to be
	body = getPage(t, s, "/project/"+itoa(p.ID)+"/addaction")
	sibs = attrValue(t, body, "data-siblings")
	if !strings.Contains(sibs, "Buy the picture") || !strings.Contains(sibs, "Hang it") {
		t.Errorf("the add-action box should offer both open actions: %s", sibs)
	}
}

// attrValue pulls one attribute's value out of rendered HTML, with the entity
// escaping undone — the list is JSON, so it arrives full of &#34;.
func attrValue(t *testing.T, body, attr string) string {
	t.Helper()
	i := strings.Index(body, attr+`="`)
	if i < 0 {
		t.Fatalf("no %s in the page", attr)
	}
	rest := body[i+len(attr)+2:]
	j := strings.Index(rest, `"`)
	if j < 0 {
		t.Fatalf("unterminated %s", attr)
	}
	return html.UnescapeString(rest[:j])
}
