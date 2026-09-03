package app

import (
	"path/filepath"
	"testing"
	"time"
)

// newTestApp returns an app with a controllable clock in UTC.
func newTestApp(t *testing.T) (*App, *time.Time) {
	t.Helper()
	a, err := Open(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC) // a Friday
	a.now = func() time.Time { return now }
	return a, &now
}

func TestCaptureDuplicates(t *testing.T) {
	a, _ := newTestApp(t)
	_, acc, err := a.Capture("Pay the rent")
	if err != nil || !acc {
		t.Fatalf("first capture: acc=%v err=%v", acc, err)
	}
	_, acc, err = a.Capture("Pay the rent")
	if err != nil || acc {
		t.Fatalf("duplicate should be dropped: acc=%v err=%v", acc, err)
	}
	items, _ := a.Inbox()
	if len(items) != 1 {
		t.Fatalf("inbox has %d items, want 1", len(items))
	}
	// processing the first makes the same text capturable again
	if err := a.ProcessTrash("inbox", items[0].ID); err != nil {
		t.Fatal(err)
	}
	_, acc, _ = a.Capture("Pay the rent")
	if !acc {
		t.Fatal("comparison must be against the open inbox only, not history")
	}
}

func TestScheduleFiringAndCollapse(t *testing.T) {
	a, now := newTestApp(t)
	if _, err := a.CreateSchedule("Water the plants", "* * *", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSchedule("Pay the rent", "1 * *", " YYYY-MM"); err != nil {
		t.Fatal(err)
	}
	// three weeks away
	*now = now.AddDate(0, 0, 21) // 2026-09-25
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	items, _ := a.Inbox()
	texts := map[string]bool{}
	for _, it := range items {
		texts[it.Text] = true
	}
	if !texts["Water the plants"] {
		t.Error("daily schedule did not fire")
	}
	if len(items) != 1 {
		t.Errorf("suffix-less daily occurrences must collapse to one item, got %d: %v", len(items), texts)
	}
	// crossing a month boundary makes the rent fire with its month suffix
	*now = time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC)
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	items, _ = a.Inbox()
	found := false
	for _, it := range items {
		if it.Text == "Pay the rent 2026-10" {
			found = true
		}
	}
	if !found {
		t.Errorf("rent with suffix not fired; inbox: %v", itemTexts(items))
	}
}

func itemTexts(items []*InboxItem) []string {
	var out []string
	for _, it := range items {
		out = append(out, it.Text)
	}
	return out
}

func TestScheduleNeverBackfires(t *testing.T) {
	a, now := newTestApp(t)
	// mark today as already opened, then create a monthly schedule mid-month
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSchedule("Monthly chore", "1 * *", ""); err != nil {
		t.Fatal(err)
	}
	*now = now.AddDate(0, 0, 1) // Sep 5: the Sep 1 occurrence predates the schedule
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	if items, _ := a.Inbox(); len(items) != 0 {
		t.Fatalf("schedule back-fired for an occurrence before it existed: %v", itemTexts(items))
	}
	*now = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	if items, _ := a.Inbox(); len(items) != 1 {
		t.Fatalf("first in-force occurrence should fire once, inbox: %v", itemTexts(items))
	}
}

func TestOneShotFiresOnceAndDeletesItself(t *testing.T) {
	a, now := newTestApp(t)
	if _, err := a.CreateSchedule("File the return", "2026-09-10", ""); err != nil {
		t.Fatal(err)
	}
	*now = time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	if items, _ := a.Inbox(); len(items) != 1 {
		t.Fatalf("one-shot did not fire: %v", itemTexts(items))
	}
	if ss, _ := a.Schedules(""); len(ss) != 0 {
		t.Fatal("one-shot must delete itself after firing")
	}
}

func TestTodayTagClears(t *testing.T) {
	a, now := newTestApp(t)
	if err := a.DayStart(); err != nil { // the day is opened before anything else happens
		t.Fatal(err)
	}
	act, err := a.CreateAction(0, ActionFields{Title: "Call the bank"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ToggleTag("action", act.ID, TodayTag); err != nil {
		t.Fatal(err)
	}
	tv, _ := a.TodayItems()
	if len(tv.Picked) != 1 {
		t.Fatalf("picked = %d, want 1", len(tv.Picked))
	}
	if err := a.DayStart(); err != nil { // same day: nothing clears
		t.Fatal(err)
	}
	tv, _ = a.TodayItems()
	if len(tv.Picked) != 1 {
		t.Fatal("same-day DayStart must not clear #today")
	}
	*now = now.AddDate(0, 0, 1)
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	tv, _ = a.TodayItems()
	if len(tv.Picked) != 0 {
		t.Fatal("#today must clear on the first use of a new day")
	}
	// and #today must not be on the editable tag list
	tags, _ := a.Tags()
	for _, tag := range tags {
		if tag == TodayTag {
			t.Fatal("#today must not appear on the editable tag list")
		}
	}
}

func TestStalledDerivation(t *testing.T) {
	a, _ := newTestApp(t)
	p, err := a.CreateProject(
		ProjectFields{Title: "Trip booked", DOD: "Flights and hotel confirmed"},
		[]ActionFields{{Title: "Book the flight"}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := a.Project(p.ID)
	if got.Stalled {
		t.Fatal("project with a next action must not be stalled")
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	got, _ = a.Project(p.ID)
	if !got.Stalled {
		t.Fatal("project with no open next action must be stalled")
	}
	// a waiting-for next action keeps it un-stalled
	act, _ := a.CreateAction(p.ID, ActionFields{Title: "Ask Sam to confirm dates", AssignedTo: "Sam"}, false)
	got, _ = a.Project(p.ID)
	if got.Stalled {
		t.Fatal("a project whose only next action is waiting-for is not stalled")
	}
	// a snoozed next action also keeps it un-stalled
	if err := a.UpdateAction(act.ID, ActionFields{Title: "Ask Sam to confirm dates"}); err != nil {
		t.Fatal(err)
	}
	if err := a.SnoozeAction(act.ID, "2027-01-01"); err != nil {
		t.Fatal(err)
	}
	got, _ = a.Project(p.ID)
	if got.Stalled {
		t.Fatal("a snoozed next action still counts for the stalled check")
	}
	// a parked-only project is stalled
	if err := a.SetNext(act.ID, false); err != nil {
		t.Fatal(err)
	}
	got, _ = a.Project(p.ID)
	if !got.Stalled {
		t.Fatal("parked-only project must be stalled")
	}
	// snoozing the project exempts it
	if err := a.UpdateProject(p.ID, ProjectFields{Title: got.Title, DOD: got.DOD, SnoozeUntil: "2027-01-01"}); err != nil {
		t.Fatal(err)
	}
	got, _ = a.Project(p.ID)
	if got.Stalled {
		t.Fatal("snoozed project is exempt from the stalled check")
	}
}

func TestCompletionRules(t *testing.T) {
	a, _ := newTestApp(t)
	p, err := a.CreateProject(
		ProjectFields{Title: "Garage cleared", DOD: "Car fits inside"},
		[]ActionFields{{Title: "Sort the shelves"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != ErrOpenActions {
		t.Fatalf("completing with open actions: err=%v, want ErrOpenActions", err)
	}
	if err := a.DeleteProject(p.ID); err != ErrOpenActions {
		t.Fatalf("deleting with open actions: err=%v, want ErrOpenActions", err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	// clearing the DOD is allowed (error state), but blocks completion
	if err := a.UpdateProject(p.ID, ProjectFields{Title: "Garage cleared", DOD: ""}); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != ErrNoDOD {
		t.Fatalf("completing without DOD: err=%v, want ErrNoDOD", err)
	}
	if err := a.UpdateProject(p.ID, ProjectFields{Title: "Garage cleared", DOD: "Car fits inside"}); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	entries, _ := a.Archive(Filters{})
	if len(entries) != 1 || entries[0].Project == nil {
		t.Fatalf("archive should hold the completed project, got %+v", entries)
	}
	// the completed step is found with its project, not on its own
	for _, e := range entries {
		if e.Action != nil {
			t.Fatal("a completed project action must not be listed on its own")
		}
	}
}

func TestDelegationRestampsClock(t *testing.T) {
	a, now := newTestApp(t)
	act, err := a.CreateAction(0, ActionFields{Title: "Draft the contract"}, false)
	if err != nil {
		t.Fatal(err)
	}
	created := *act.BecameNextAt
	*now = now.AddDate(0, 0, 21)
	if err := a.UpdateAction(act.ID, ActionFields{Title: "Draft the contract", AssignedTo: "Alex"}); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Action(act.ID)
	if !got.BecameNextAt.After(created) {
		t.Fatal("delegating must restamp becameNextActionAt")
	}
	if wf, _ := a.WaitingFor(Filters{}); len(wf) != 1 {
		t.Fatal("delegated action should be in Waiting for")
	}
	if next, _ := a.NextActions(Filters{}); len(next) != 0 {
		t.Fatal("delegated action must leave Next actions")
	}
}

func TestDetachStampsParkedAction(t *testing.T) {
	a, _ := newTestApp(t)
	p, _ := a.CreateProject(ProjectFields{Title: "Shed built", DOD: "Roof on"},
		[]ActionFields{{Title: "Pour the base"}})
	parked, err := a.CreateAction(p.ID, ActionFields{Title: "Paint the shed"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if parked.BecameNextAt != nil {
		t.Fatal("parked action must not be next")
	}
	if err := a.Detach(parked.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := a.Action(parked.ID)
	if got.ProjectID != 0 || got.BecameNextAt == nil {
		t.Fatalf("detached action must be standalone and next: %+v", got)
	}
	if tasks, _ := a.Tasks(Filters{}); len(tasks) != 1 {
		t.Fatal("detached action should appear in Tasks")
	}
}

func TestNextActionsFilters(t *testing.T) {
	a, _ := newTestApp(t)
	mk := func(f ActionFields) *Action {
		act, err := a.CreateAction(0, f, false)
		if err != nil {
			t.Fatal(err)
		}
		return act
	}
	mk(ActionFields{Title: "Buy milk", Context: "grocery"})
	mk(ActionFields{Title: "Buy the special cheese", Context: "grocery", ContextParam: "Selver"})
	mk(ActionFields{Title: "Fix the tap", Context: "home", NeedsFocus: true, Tags: []string{"house"}})
	mk(ActionFields{Title: "Think of a gift"}) // no context: always shown

	acts, _ := a.NextActions(Filters{Contexts: []string{"grocery(Selver)"}})
	if len(acts) != 3 { // milk (bare grocery), cheese (exact), gift (no context)
		t.Fatalf("Selver filter: got %d actions, want 3", len(acts))
	}
	acts, _ = a.NextActions(Filters{Contexts: []string{"grocery"}})
	if len(acts) != 2 { // milk + gift; the parameterised form is narrower
		t.Fatalf("bare grocery filter: got %d, want 2: %v", len(acts), titles(acts))
	}
	acts, _ = a.NextActions(Filters{Focus: "only"})
	if len(acts) != 1 || acts[0].Title != "Fix the tap" {
		t.Fatalf("focus only: %v", titles(acts))
	}
	acts, _ = a.NextActions(Filters{Tags: []string{"house"}})
	if len(acts) != 1 { // untagged items excluded once a tag is selected
		t.Fatalf("tag filter: %v", titles(acts))
	}
	acts, _ = a.NextActions(Filters{Name: "buy milk"})
	if len(acts) != 1 {
		t.Fatalf("name filter words AND together: %v", titles(acts))
	}
}

func titles(acts []*Action) []string {
	var out []string
	for _, a := range acts {
		out = append(out, a.Title)
	}
	return out
}

func TestErrorStates(t *testing.T) {
	a, _ := newTestApp(t)
	act, _ := a.CreateAction(0, ActionFields{Title: "File the return", DueDate: "2026-09-10", SnoozeUntil: "2026-09-20"}, false)
	got, _ := a.Action(act.ID)
	if len(got.Errors()) == 0 {
		t.Fatal("snooze past due date must be an error state")
	}
}

func TestTagListManagement(t *testing.T) {
	a, _ := newTestApp(t)
	act, _ := a.CreateAction(0, ActionFields{Title: "Change the tyres", Tags: []string{"car"}}, false)
	if err := a.RemoveTag("car"); err == nil {
		t.Fatal("removing a tag still in use must be refused")
	}
	if err := a.ToggleTag("action", act.ID, "car"); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveTag("car"); err != nil {
		t.Fatalf("removing an unused tag should work: %v", err)
	}
}
