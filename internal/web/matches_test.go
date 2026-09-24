package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todoistik/internal/app"
	"todoistik/internal/conf"
)

// What a capture looks like is shown beside it while it is being processed,
// and a finished match can be copied into the branch's own form (design.md,
// "Matches while processing"). These pin the screen: that the list is stage
// one's alone, that only the finished half is pressable, and that a copy fills
// the form from the item rather than from the capture.

func matchServer(t *testing.T) (*Server, *app.App) {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	c := conf.Defaults()
	s, err := New(a, "", c)
	if err != nil {
		t.Fatal(err)
	}
	return s, a
}

// finished writes an action and completes it, which is the only shape a
// copyable match comes in.
func finished(t *testing.T, a *app.App, f app.ActionFields) *app.Action {
	t.Helper()
	act, err := a.CreateAction(0, f)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	return act
}

func TestTheMatchListIsStageOnesAlone(t *testing.T) {
	s, a := matchServer(t)
	finished(t, a, app.ActionFields{Title: "Pay the rent"})
	if _, _, err := a.Capture("Pay the rent", app.SourceApp); err != nil {
		t.Fatal(err)
	}

	stageOne := getPage(t, s, "/process?item=1&one=1")
	if !strings.Contains(stageOne, `class="matches"`) {
		t.Fatal("stage one drew no match list")
	}
	if !strings.Contains(stageOne, `data-key="1"`) {
		t.Error("the finished match carries no digit")
	}
	stageTwo := getPage(t, s, "/process?item=1&one=1&as=action")
	if strings.Contains(stageTwo, `class="matches"`) {
		t.Error("stage two asks a question that has been answered")
	}
}

// An open match is the warning and the finished one is the offer: only the
// second is pressable, because copying something already open would write the
// duplicate the list exists to point out.
func TestOnlyTheFinishedHalfCanBeCopied(t *testing.T) {
	s, a := matchServer(t)
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Pay the rent"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Pay the rent", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/process?item=1&one=1")
	if !strings.Contains(body, "already open") {
		t.Fatal("the open match was not shown")
	}
	if strings.Contains(body, "matchcopy") || strings.Contains(body, `data-key="1"`) {
		t.Error("an open match was offered as something to copy")
	}
}

// The digits keep the bar quiet: the rows are numbered on the screen, so the
// bar draws the range instead of nine entries saying "copy" (keys.md).
func TestAMatchRowKeepsItsKeyOutOfTheBar(t *testing.T) {
	s, a := matchServer(t)
	finished(t, a, app.ActionFields{Title: "Pay the rent"})
	if _, _, err := a.Capture("Pay the rent", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/process?item=1&one=1")
	if !strings.Contains(body, "data-key-quiet") {
		t.Error("the digit would be listed in the bar nine times over")
	}
}

// The two-minute rule is Done, and the digits are the match list's (keys.md,
// "The map").
func TestTheTwoMinuteRuleIsDone(t *testing.T) {
	s, a := matchServer(t)
	if _, _, err := a.Capture("Pay the rent", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	body := getPage(t, s, "/process?item=1&one=1")
	if !strings.Contains(body, `data-key="d" data-key-label="two-minute rule"`) {
		t.Error("the two-minute branch is not on d")
	}
	if strings.Contains(body, `data-key="2"`) {
		t.Error("2 is still a branch, and the match list needs the digits")
	}
}

// A copy starts the form from the finished item: its wording, what describes
// the work, and the description it was done from — with the capture's own body
// under it rather than instead of it (design.md, "Copying a finished one").
func TestCopyingAFinishedActionSeedsTheForm(t *testing.T) {
	s, a := matchServer(t)
	if err := a.AddContext("home", ""); err != nil {
		t.Fatal(err)
	}
	act := finished(t, a, app.ActionFields{
		Title: "Pay the rent for August", Context: "home", Duration: app.DurShort,
		Description: "Reference RENT-2026", DueDate: "2026-08-01",
	})
	if _, _, err := a.Capture("Pay the rent for October\nthe new amount is 640", app.SourceApp); err != nil {
		t.Fatal(err)
	}

	body := getPage(t, s, "/process?item=1&one=1&as=action&from="+itoa(act.ID))
	if !strings.Contains(body, `name="title" value="Pay the rent for August"`) {
		t.Error("the title did not come from the finished action")
	}
	if !strings.Contains(body, `name="meta" value="@home #short"`) {
		t.Error("the meta line did not come from the finished action")
	}
	if strings.Contains(body, "due:2026-08-01") {
		t.Error("a deadline from a month that has passed was copied")
	}
	if !strings.Contains(body, "Reference RENT-2026") || !strings.Contains(body, "the new amount is 640") {
		t.Error("the description should hold both what was copied and what was captured")
	}
	if !strings.Contains(body, "copied from a finished action") {
		t.Error("the screen did not say the boxes are not the capture's words")
	}
}

// A copied project brings the plan: its definition of done and every action it
// was completed with — the first in the open next-action boxes, the rest held
// as rows, which is the two places a project form keeps its actions.
func TestCopyingAFinishedProjectBringsThePlan(t *testing.T) {
	s, a := matchServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Book the tyre change"}, {Title: "Top up the screenwash"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, act := range p.Actions {
		if err := a.CompleteAction(act.ID); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.CompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Winter-proof the car", app.SourceApp); err != nil {
		t.Fatal(err)
	}

	body := getPage(t, s, "/process?item=1&one=1&as=project&from="+itoa(p.ID))
	if !strings.Contains(body, ">it starts in January</textarea>") {
		t.Error("the definition of done was not copied")
	}
	for _, want := range []string{`name="atitle" value="Book the tyre change"`, `name="atitle" value="Top up the screenwash"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing draft: %s", want)
		}
	}
	// the first lands in the open boxes, which is also what gates the create
	// button now: a required title box, not a hidden field standing in for one
	if !strings.Contains(body, `name="atitle" value="Book the tyre change" required data-verbcheck`) {
		t.Error("the first copied action is not in the open next-action boxes")
	}
	if !strings.Contains(body, `<input type="hidden" name="atitle" value="Top up the screenwash">`) {
		t.Error("the copied actions after the first are not rows")
	}
	if strings.Contains(body, "hasaction") {
		t.Error("the create gate still stands on a hidden field of its own")
	}
}

// A `from` that names nothing finished is a stale link, and the capture is
// what the screen is about: the form seeds from it rather than erroring.
func TestAStaleCopyFallsBackToTheCapture(t *testing.T) {
	s, a := matchServer(t)
	open, err := a.CreateAction(0, app.ActionFields{Title: "Pay the rent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Book the tyre change", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	for _, from := range []string{itoa(open.ID), "999"} {
		body := getPage(t, s, "/process?item=1&one=1&as=action&from="+from)
		if !strings.Contains(body, `name="title" value="Book the tyre change"`) {
			t.Errorf("from=%s: the form did not fall back to the capture", from)
		}
		if strings.Contains(body, "copied from") {
			t.Errorf("from=%s: the screen claimed a copy it did not make", from)
		}
	}
}

// Turned off is off: nothing is compared and the screen is what it was.
func TestMatchingCanBeTurnedOff(t *testing.T) {
	a, err := app.Open(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	c := conf.Defaults()
	c.DupMatch = conf.MatchNone
	s, err := New(a, "", c)
	if err != nil {
		t.Fatal(err)
	}
	finished(t, a, app.ActionFields{Title: "Pay the rent"})
	if _, _, err := a.Capture("Pay the rent", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/process?item=1&one=1", nil))
	if strings.Contains(rec.Body.String(), `class="matches"`) {
		t.Error("none should draw nothing")
	}
}

// A row is badged with the word the thing would be found under: a standalone
// action is a task, one inside a project is an action, and the copy link still
// names the branch that creates it (design.md, "Matches while processing").
func TestAMatchIsBadgedWithTheWordItIsFoundUnder(t *testing.T) {
	s, a := matchServer(t)
	finished(t, a, app.ActionFields{Title: "Change the tyres"})
	if _, err := a.CreateProject(
		app.ProjectFields{Title: "Winter-proof the car", DOD: "it starts in January"},
		[]app.ActionFields{{Title: "Change the tyres over"}},
	); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Change the tyres", app.SourceApp); err != nil {
		t.Fatal(err)
	}

	body := getPage(t, s, "/process?item=1&one=1")
	if !strings.Contains(body, `<span class="badge">task</span>`) {
		t.Error("the standalone action is not badged a task")
	}
	if !strings.Contains(body, `<span class="badge">action</span>`) {
		t.Error("the action inside a project is not badged an action")
	}
	// the badge changed and the branch did not: a copy still opens the form
	// that creates an action
	if !strings.Contains(body, "&amp;as=action&amp;from=") {
		t.Error("the copy link no longer names the branch it opens")
	}
}
