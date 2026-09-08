package app

import (
	"path/filepath"
	"reflect"
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

// A rule that names its years fires inside them and then has nothing left to
// do, which is the same end a one-shot has always come to (design.md,
// "Schedule").
func TestScheduleWithYearRunsOutAndDeletesItself(t *testing.T) {
	a, now := newTestApp(t)
	if _, err := a.CreateSchedule("Chase the invoice", "9-23 9 * 2026", " DD"); err != nil {
		t.Fatal(err)
	}
	*now = time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	items, _ := a.Inbox()
	if len(items) != 2 { // the 9th and the 10th, each distinct by its suffix
		t.Fatalf("want the 9th and 10th fired, got: %v", itemTexts(items))
	}
	if ss, _ := a.Schedules(""); len(ss) != 1 {
		t.Fatalf("schedule left mid-run, want 1 got %d", len(ss))
	}
	// past the last occurrence the rule can never fire again, so it goes
	*now = time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	ss, err := a.Schedules("")
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 0 {
		t.Fatalf("spent schedule not deleted: %v", ss[0].Rule)
	}
	// and a rule with no year still stays, having always something ahead
	if _, err := a.CreateSchedule("Pay the rent", "1 * *", ""); err != nil {
		t.Fatal(err)
	}
	*now = time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	if ss, _ := a.Schedules(""); len(ss) != 1 {
		t.Fatalf("open-ended schedule deleted itself, got %d", len(ss))
	}
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

// The review periods: a week for everything, the someday cadence for
// someday/maybe items — and no exemption for snoozed items, because the
// snooze date is one of the claims the review checks (design.md, "Weekly
// review").
func TestReviewOutstanding(t *testing.T) {
	a, now := newTestApp(t)
	p, err := a.CreateProject(
		ProjectFields{Title: "Shed built", DOD: "Roof on"},
		[]ActionFields{{Title: "Buy the boards"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.UpdateProject(p.ID, ProjectFields{Title: p.Title, DOD: p.DOD, SnoozeUntil: "2027-01-01"}); err != nil {
		t.Fatal(err)
	}
	if err := a.SnoozeAction(p.Actions[0].ID, "2027-01-01"); err != nil {
		t.Fatal(err)
	}
	if _, acc, err := a.Capture("Learn the banjo"); err != nil || !acc {
		t.Fatalf("capture: acc=%v err=%v", acc, err)
	}
	items, _ := a.Inbox()
	if _, err := a.ProcessSomeday(items[0].ID, "Learn the banjo", "2027-01-01"); err != nil {
		t.Fatal(err)
	}

	created := *now
	c, err := a.ReviewCounts()
	if err != nil {
		t.Fatal(err)
	}
	if c.OutstandingTotal() != 0 {
		t.Fatalf("fresh items are outstanding: %+v", c)
	}

	// past the week: the snoozed project and its snoozed action are both
	// outstanding, and the someday item is still inside its month
	*now = created.Add(8 * 24 * time.Hour)
	c, _ = a.ReviewCounts()
	if c.Projects != 1 {
		t.Errorf("snoozed project after 8 days: %d outstanding, want 1", c.Projects)
	}
	if c.Next != 1 {
		t.Errorf("snoozed next action after 8 days: %d outstanding, want 1", c.Next)
	}
	if c.Someday != 0 {
		t.Errorf("someday item after 8 days: %d outstanding, want 0 — its cadence is a month", c.Someday)
	}

	// past the month: the someday item joins them, snoozed or not
	*now = created.Add(31 * 24 * time.Hour)
	c, _ = a.ReviewCounts()
	if c.Someday != 1 {
		t.Errorf("someday item after 31 days: %d outstanding, want 1", c.Someday)
	}

	// the cadence is the settings file's to change
	a.SetSomedayReviewDays(90)
	c, _ = a.ReviewCounts()
	if c.Someday != 0 {
		t.Errorf("someday item after 31 days of a 90-day cadence: %d outstanding, want 0", c.Someday)
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
	mk(ActionFields{Title: "Think of a gift"}) // no context: found under "all"

	acts, _ := a.NextActions(Filters{Contexts: []string{"grocery(Selver)"}})
	if len(acts) != 2 { // milk (bare grocery) and cheese (exact); not the gift
		t.Fatalf("Selver filter: got %d actions, want 2: %v", len(acts), titles(acts))
	}
	acts, _ = a.NextActions(Filters{Contexts: []string{"grocery"}})
	if len(acts) != 1 { // milk only; the parameterised form is narrower
		t.Fatalf("bare grocery filter: got %d, want 1: %v", len(acts), titles(acts))
	}
	// asking for nothing is asking for all of them, the gift included
	if acts, _ = a.NextActions(Filters{}); len(acts) != 4 {
		t.Fatalf("no context filter: got %d, want all 4: %v", len(acts), titles(acts))
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

// The Action branch of Inbox Zero can file into an existing project, and the
// name that picks one out is matched the way the name filter matches.
func TestProcessActionIntoProject(t *testing.T) {
	a, _ := newTestApp(t)
	p, err := a.CreateProject(ProjectFields{Title: "Winter-proof the car", DOD: "car survives winter"},
		[]ActionFields{{Title: "Check the battery"}})
	if err != nil {
		t.Fatal(err)
	}
	done, _ := a.CreateProject(ProjectFields{Title: "Sail licence", DOD: "licence in hand"},
		[]ActionFields{{Title: "Book the exam"}})
	if err := a.CompleteAction(done.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(done.ID); err != nil {
		t.Fatal(err)
	}

	// matched on the title, and every word must appear — "winter car" hits
	// the one project, "sail" hits none because a completed one is not active
	if hits, _ := a.MatchProjects("winter car"); len(hits) != 1 || hits[0].ID != p.ID {
		t.Fatalf("matching a project by name: got %d hits", len(hits))
	}
	if hits, _ := a.MatchProjects("sail"); len(hits) != 0 {
		t.Fatalf("a completed project is not a target: got %d hits", len(hits))
	}
	if hits, _ := a.MatchProjects("   "); len(hits) != 0 {
		t.Fatal("an empty name means standalone, not every project")
	}

	it, _, _ := a.Capture("Book the winter tyre change")
	act, err := a.ProcessAction("inbox", it.ID, ActionFields{Title: "Book the winter tyre change"}, p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if act.ProjectID != p.ID {
		t.Fatalf("action landed on project %d, want %d", act.ProjectID, p.ID)
	}
	if act.BecameNextAt == nil {
		t.Fatal("an action filed into a project is its next action unless parked")
	}
	if items, _ := a.Inbox(); len(items) != 0 {
		t.Fatal("the branch must consume the inbox item")
	}
}

// Parking is the deliberate act, and it is only meaningful inside a project.
func TestProcessActionParkedAndStandalone(t *testing.T) {
	a, _ := newTestApp(t)
	p, _ := a.CreateProject(ProjectFields{Title: "Kitchen renovation", DOD: "kitchen usable"},
		[]ActionFields{{Title: "Measure the wall"}})

	it, _, _ := a.Capture("Price the worktop")
	act, err := a.ProcessAction("inbox", it.ID, ActionFields{Title: "Price the worktop"}, p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if act.BecameNextAt != nil {
		t.Fatal("a parked action is not a next action")
	}

	it2, _, _ := a.Capture("Pay the rent")
	if _, err := a.ProcessAction("inbox", it2.ID, ActionFields{Title: "Pay the rent"}, 0, true); err == nil {
		t.Fatal("parking a standalone action must be refused")
	}
	if items, _ := a.Inbox(); len(items) != 1 {
		t.Fatal("a refused branch must leave the item in the inbox")
	}
	act2, err := a.ProcessAction("inbox", it2.ID, ActionFields{Title: "Pay the rent"}, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if act2.ProjectID != 0 || act2.BecameNextAt == nil {
		t.Fatal("a standalone action is a next action from the moment it exists")
	}
}

// A completed project can not take a new action; a snoozed one can, because a
// snooze is about not being bugged, not about being closed to new work.
func TestProcessActionRejectsCompletedProject(t *testing.T) {
	a, _ := newTestApp(t)
	p, _ := a.CreateProject(ProjectFields{Title: "Q3 accounts", DOD: "filed"},
		[]ActionFields{{Title: "Collect receipts"}})
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteProject(p.ID); err != nil {
		t.Fatal(err)
	}
	it, _, _ := a.Capture("File the VAT return")
	if _, err := a.ProcessAction("inbox", it.ID, ActionFields{Title: "File the VAT return"}, p.ID, false); err == nil {
		t.Fatal("a completed project must not take a new action")
	}
	if items, _ := a.Inbox(); len(items) != 1 {
		t.Fatal("a refused branch must leave the item in the inbox")
	}
}

// The picker offered when an action is being given a home: matched by the
// name filter's rule, ordered by recent activity, capped with the true total
// reported so the screen can say what it is not showing.
func TestProjectCandidates(t *testing.T) {
	a, now := newTestApp(t)
	mk := func(title, action string) *Project {
		p, err := a.CreateProject(ProjectFields{Title: title, DOD: "done when done"},
			[]ActionFields{{Title: action}})
		if err != nil {
			t.Fatal(err)
		}
		*now = now.Add(time.Minute) // each one is newer than the last
		return p
	}
	mk("Kitchen renovation", "Measure the wall")
	mk("Winter-proof the car", "Check the battery")
	mk("Kitchen lighting", "Choose the fittings")
	newest := mk("Sail licence", "Book the exam")

	// no query: everything, newest activity first
	hits, total, err := a.ProjectCandidates("", 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 4 || len(hits) != 4 {
		t.Fatalf("got %d hits of %d, want 4 of 4", len(hits), total)
	}
	if hits[0].ID != newest.ID {
		t.Fatalf("most recently active first: got %q", hits[0].Title)
	}

	// the cap reports the true total, so the screen can say what it hides
	hits, total, _ = a.ProjectCandidates("", 2)
	if len(hits) != 2 || total != 4 {
		t.Fatalf("capped: got %d hits of %d, want 2 of 4", len(hits), total)
	}

	// the name filter's rule: every word a substring, in any order
	if hits, _, _ = a.ProjectCandidates("kit", 0); len(hits) != 2 {
		t.Fatalf("kit should match both kitchens: got %d", len(hits))
	}
	if hits, _, _ = a.ProjectCandidates("winter car", 0); len(hits) != 1 {
		t.Fatalf("words match in any order: got %d", len(hits))
	}
	if hits, _, _ = a.ProjectCandidates("zzz", 0); len(hits) != 0 {
		t.Fatal("a name matching nothing offers nothing")
	}

	// filing an action into a project makes it the most recent again
	it, _, _ := a.Capture("Order the worktop")
	kitchen, _, _ := a.ProjectCandidates("Kitchen renovation", 0)
	*now = now.Add(time.Minute)
	if _, err := a.ProcessAction("inbox", it.ID, ActionFields{Title: "Order the worktop"}, kitchen[0].ID, false); err != nil {
		t.Fatal(err)
	}
	hits, _, _ = a.ProjectCandidates("", 0)
	if hits[0].Title != "Kitchen renovation" {
		t.Fatalf("the project just filed into leads: got %q", hits[0].Title)
	}
	if hits[0].OpenCount != 2 {
		t.Fatalf("open count: got %d, want 2", hits[0].OpenCount)
	}
}

// The filter line is the only way filters are written now, so what it means
// is pinned here: which tokens are fields, which are names off the remembered
// lists, and what counts as something to ask about before it can be applied.
func TestParseQuery(t *testing.T) {
	v := &Vocabulary{
		Contexts: map[string]bool{"home": true, "grocery": true},
		Tags:     map[string]bool{"car": true, "house": true},
	}

	f, problems := ParseQuery("@grocery(Selver) #car #house #short #focus buy milk", v)
	if len(problems) != 0 {
		t.Fatalf("a line of known names had problems: %+v", problems)
	}
	if !reflect.DeepEqual(f.Contexts, []string{"grocery(Selver)"}) {
		t.Errorf("contexts = %v", f.Contexts)
	}
	if !reflect.DeepEqual(f.Tags, []string{"car", "house"}) {
		t.Errorf("tags = %v", f.Tags)
	}
	if !reflect.DeepEqual(f.Durations, []Duration{DurShort}) {
		t.Errorf("durations = %v", f.Durations)
	}
	if f.Focus != "only" {
		t.Errorf("focus = %q, want only", f.Focus)
	}
	if f.Name != "buy milk" { // the words left over, in the order they were typed
		t.Errorf("name = %q, want the leftover words", f.Name)
	}

	// the line is written back out of the filters, in one fixed order
	if got := f.Query(); got != "@grocery(Selver) #car #house #short #focus buy milk" {
		t.Errorf("round trip = %q", got)
	}

	for _, tc := range []struct{ q, kind string }{
		{"@hoem", ProblemContext},
		{"#kar", ProblemTag},
		{"@home @grocery", ProblemSecondContext},
		{"#parked", ProblemNotAFilter},
	} {
		_, problems := ParseQuery(tc.q, v)
		if len(problems) != 1 || problems[0].Kind != tc.kind {
			t.Errorf("%q gave %+v, want one %s problem", tc.q, problems, tc.kind)
		}
	}

	// what is right about a line still parses while the rest is asked about
	f, problems = ParseQuery("@home #kar milk", v)
	if len(problems) != 1 || f.Name != "milk" || len(f.Contexts) != 1 {
		t.Errorf("a line with one bad token lost the good ones: %+v %+v", f, problems)
	}
}

// The windows are the one part of the filter line that is not a name: words
// rather than dates, because "what is coming at me" moves with the day.
func TestParseQueryWindows(t *testing.T) {
	v := &Vocabulary{Tags: map[string]bool{"car": true}}

	f, problems := ParseQuery("due:thisweek #car", v)
	if len(problems) != 0 {
		t.Fatalf("a good window had problems: %+v", problems)
	}
	if f.Due != "thisweek" || f.Name != "" {
		t.Errorf("due = %q, name = %q — the window must not be left as a word", f.Due, f.Name)
	}
	if got := f.Query(); got != "#car due:thisweek" {
		t.Errorf("round trip = %q", got)
	}

	if _, problems = ParseQuery("due:sometime", v); len(problems) != 1 || problems[0].Kind != ProblemWindow {
		t.Errorf("a window that is not one gave %+v", problems)
	}
	// snooze is a meta-line notation, and is refused here rather than being
	// quietly matched as a word
	if _, problems = ParseQuery("snooze:2026-09-20", v); len(problems) != 1 || problems[0].Kind != ProblemNotAFilter {
		t.Errorf("snooze: on a filter line gave %+v", problems)
	}
	if f, _ = ParseQuery("completed:lastweek", v); f.Completed != "lastweek" {
		t.Errorf("completed = %q", f.Completed)
	}
}
