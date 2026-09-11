package app

import (
	"errors"
	"testing"
	"time"

	"todoistik/internal/request"
)

// A completed item is frozen (design.md, "Completion"). These tests pin the
// rule at the domain layer rather than at the screens, because that is where
// it is enforced: a second tab, a stale page, the API and the next way in all
// arrive here.

func completedAction(t *testing.T, a *App) *Action {
	t.Helper()
	act, err := a.CreateAction(0, ActionFields{Title: "Change the winter tyres"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	act, err = a.Action(act.ID)
	if err != nil {
		t.Fatal(err)
	}
	return act
}

func completedProject(t *testing.T, a *App) *Project {
	t.Helper()
	p, err := a.CreateProject(
		ProjectFields{Title: "Winter tyres on", DOD: "All four swapped and balanced"},
		[]ActionFields{{Title: "Book the garage"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	return p
}

// Every write an action's page can perform, refused once the action is
// completed. Named one by one rather than driven off a list, so that a new
// write added to the app is a new line here rather than a silent gap.
func TestACompletedActionRefusesEveryEdit(t *testing.T) {
	a, _ := newTestApp(t)
	act := completedAction(t, a)

	cases := map[string]error{
		"edit":     a.UpdateAction(act.ID, ActionFields{Title: "Change the summer tyres"}),
		"complete": a.CompleteAction(act.ID),
		"delete":   a.DeleteAction(act.ID),
		"park":     a.SetNext(act.ID, false),
		"next":     a.SetNext(act.ID, true),
		"snooze":   a.SnoozeAction(act.ID, "2026-10-01"),
		"tag":      a.ToggleTag("action", act.ID, "car"),
		"pick":     a.ToggleTag("action", act.ID, TodayTag),
	}
	for what, err := range cases {
		if !errors.Is(err, ErrCompleted) {
			t.Errorf("%s on a completed action: err=%v, want ErrCompleted", what, err)
		}
	}

	// and nothing of it moved
	again, err := a.Action(act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Title != "Change the winter tyres" || again.SnoozeUntil != "" || len(again.Tags) != 0 {
		t.Fatalf("the frozen action was changed anyway: %+v", again)
	}
	if again.CompletedAt == nil || !again.CompletedAt.Equal(*act.CompletedAt) {
		t.Fatal("completing an already completed action must not restamp the moment")
	}
}

// Detaching and promoting reshape an item, which is an edit with a bigger
// name (design.md, "Reshaping items").
func TestACompletedActionIsNotReshaped(t *testing.T) {
	a, _ := newTestApp(t)
	p, err := a.CreateProject(
		ProjectFields{Title: "Winter tyres on", DOD: "All four swapped"},
		[]ActionFields{{Title: "Book the garage"}})
	if err != nil {
		t.Fatal(err)
	}
	inProject := p.Actions[0].ID
	if err := a.CompleteAction(inProject); err != nil {
		t.Fatal(err)
	}
	if err := a.Detach(inProject); !errors.Is(err, ErrCompleted) {
		t.Errorf("detaching a completed action: err=%v, want ErrCompleted", err)
	}

	standalone := completedAction(t, a)
	_, err = a.Promote(standalone.ID,
		ProjectFields{Title: "Tyres sorted", DOD: "Done"},
		[]ActionFields{{Title: "Book the garage"}})
	if !errors.Is(err, ErrCompleted) {
		t.Errorf("promoting a completed action: err=%v, want ErrCompleted", err)
	}
	// promotion consumes the action it is given, so the refusal has to leave
	// it standing — half a promotion would lose the archived item outright
	if _, err := a.Action(standalone.ID); err != nil {
		t.Fatalf("the refused promotion took the action with it: %v", err)
	}
}

func TestACompletedProjectRefusesEveryEdit(t *testing.T) {
	a, _ := newTestApp(t)
	p := completedProject(t, a)

	cases := map[string]error{
		"edit":     a.UpdateProject(p.ID, ProjectFields{Title: "Summer tyres on", DOD: "x"}),
		"complete": a.CompleteProject(p.ID),
		"delete":   a.DeleteProject(p.ID),
		"tag":      a.ToggleTag("project", p.ID, "car"),
	}
	for what, err := range cases {
		if !errors.Is(err, ErrCompleted) {
			t.Errorf("%s on a completed project: err=%v, want ErrCompleted", what, err)
		}
	}

	again, err := a.Project(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.Title != "Winter tyres on" || len(again.Tags) != 0 {
		t.Fatalf("the frozen project was changed anyway: %+v", again)
	}
}

// A finished project takes no new work, whichever way an action arrives at it:
// the add-action screen, or the Action branch of Inbox Zero.
func TestACompletedProjectTakesNoNewAction(t *testing.T) {
	a, _ := newTestApp(t)
	p := completedProject(t, a)

	if _, err := a.CreateAction(p.ID, ActionFields{Title: "Balance them"}, false); !errors.Is(err, ErrCompleted) {
		t.Errorf("adding an action to a completed project: err=%v, want ErrCompleted", err)
	}
	it, _, err := a.Capture("Balance the wheels")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProcessAction(it.ID, ActionFields{Title: "Balance the wheels"}, p.ID, false); !errors.Is(err, ErrCompleted) {
		t.Errorf("filing an inbox item into a completed project: err=%v, want ErrCompleted", err)
	}
	// and the inbox item is still there to be filed somewhere that exists
	if _, err := a.InboxItem(it.ID); err != nil {
		t.Fatalf("the refused branch consumed the inbox item: %v", err)
	}

	again, err := a.Project(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Actions) != 1 {
		t.Fatalf("the completed project grew an action: %d", len(again.Actions))
	}
}

// The one write a completed item accepts, and the reason the rest can be
// refused flatly: bringing it back is how anything else is reached.
func TestBringingItBackUnfreezesIt(t *testing.T) {
	a, _ := newTestApp(t)
	act := completedAction(t, a)
	if err := a.UncompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateAction(act.ID, ActionFields{Title: "Change the summer tyres"}); err != nil {
		t.Fatalf("an action brought back is an ordinary action again: %v", err)
	}

	p := completedProject(t, a)
	if err := a.UncompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateProject(p.ID, ProjectFields{Title: "Summer tyres on", DOD: "All four swapped"}); err != nil {
		t.Fatalf("a project brought back is an ordinary project again: %v", err)
	}
}

// A completion request is answered by completing the action it names, and the
// freeze must not be what stops that: the item it names is open by definition
// — one that is not is already refused for saying so (see requests.go).
func TestTheFreezeDoesNotBlockACompletionRequest(t *testing.T) {
	a, now := newTestApp(t)
	act, err := a.CreateAction(0, ActionFields{Title: "Change the winter tyres"}, false)
	if err != nil {
		t.Fatal(err)
	}
	// the work was finished two days after the action was written and a day
	// before the request is answered — the ordinary shape of one (design.md,
	// "Completion requests")
	finished := now.Add(48 * time.Hour)
	*now = now.Add(72 * time.Hour)
	it, _, err := a.Capture(request.Write(request.Line{
		Source: "reminders", ItemID: act.ID, At: finished, Name: act.Title,
	}))
	if err != nil {
		t.Fatal(err)
	}
	done, err := a.ProcessCompletion(it.ID)
	if err != nil {
		t.Fatalf("answering a request about an open action: %v", err)
	}
	if done.CompletedAt == nil || !done.CompletedAt.Equal(finished.UTC()) {
		t.Fatalf("stamped %v, want the moment the work was finished (%v)", done.CompletedAt, finished)
	}
	// the request left the inbox with the answer, so there is no second one
	if _, err := a.InboxItem(it.ID); err == nil {
		t.Fatal("the answered request is still in the inbox")
	}
}
