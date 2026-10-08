package app

import (
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Undo is the one place where being nearly right loses something, so these
// pin the rules design.md, "Undo" states: a step goes back whole, steps go
// back newest first, and what was not done at the keyboard is never a step.

func mustUndo(t *testing.T, a *App) *UndoStep {
	t.Helper()
	last, err := a.LastUndo()
	if err != nil || last == nil {
		t.Fatalf("nothing to undo: %v", err)
	}
	step, err := a.Undo(last.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	return step
}

// do is one keypress: what it writes is one step.
func do(t *testing.T, a *App, fn func() error) {
	t.Helper()
	a.Gesture(func() {
		if err := fn(); err != nil {
			t.Fatal(err)
		}
	})
}

func shed(t *testing.T, a *App) *Project {
	t.Helper()
	var p *Project
	do(t, a, func() (err error) {
		p, err = a.CreateProject(ProjectFields{Title: "Shed built", DOD: "Roof on", Tags: []string{"garden"}},
			[]ActionFields{{Title: "Pour the base", Tags: []string{"concrete"}, Context: "home"}, {Title: "Paint it"}})
		return err
	})
	return p
}

// A deleted action comes back as it was: its fields, its tags, and the
// project's pointer at it as the next action — which the delete cleared
// through a foreign key rather than through any statement of this package's.
func TestUndoPutsADeletedActionBackWhole(t *testing.T) {
	a, _ := newTestApp(t)
	p := shed(t, a)
	base := p.Actions[0].ID
	do(t, a, func() error { return a.MakeNext(base) })
	before, err := a.Action(base)
	if err != nil {
		t.Fatal(err)
	}
	do(t, a, func() error { return a.DeleteAction(base) })
	if _, err := a.Action(base); err == nil {
		t.Fatal("the action is still there after its delete")
	}

	step := mustUndo(t, a)
	if len(step.Entries) != 1 || step.Entries[0].Event != EvDeleted || step.Entries[0].Name != "Pour the base" {
		t.Fatalf("the step read out as %+v", step.Entries)
	}
	after, err := a.Action(base)
	if err != nil {
		t.Fatalf("the action did not come back: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("came back changed:\n was %+v\n  is %+v", before, after)
	}
	var next sql.NullInt64
	a.db.QueryRow(`SELECT next_action_id FROM projects WHERE id=?`, p.ID).Scan(&next)
	if next.Int64 != base {
		t.Fatalf("the project points at %v as next, not at the action that came back", next)
	}
}

// Done and Save, the other two the feature was asked for by name.
func TestUndoTakesBackACompletionAndAnEdit(t *testing.T) {
	a, _ := newTestApp(t)
	p := shed(t, a)
	id := p.Actions[0].ID
	saved, _ := a.Action(id)

	do(t, a, func() error {
		return a.UpdateAction(id, ActionFields{Title: "Pour the slab", Tags: []string{"cement"}, Context: "yard"})
	})
	do(t, a, func() error { return a.CompleteAction(id) })

	mustUndo(t, a)
	act, _ := a.Action(id)
	if act.CompletedAt != nil || act.Title != "Pour the slab" {
		t.Fatalf("after undoing the completion: %+v", act)
	}
	mustUndo(t, a)
	act, _ = a.Action(id)
	if !reflect.DeepEqual(saved, act) {
		t.Fatalf("after undoing the save:\n was %+v\n  is %+v", saved, act)
	}
	// the names the save taught the lists go back with it: they were written
	// by the step, and nothing else carries them
	tags, _ := a.Tags()
	if contains(tags, "cement") {
		t.Fatalf("the tag the undone save created is still remembered: %v", tags)
	}
}

// More than one level, newest first, down to nothing — and then it says so.
func TestUndoWalksBackStepByStep(t *testing.T) {
	a, _ := newTestApp(t)
	var one, two *InboxItem
	do(t, a, func() (err error) { one, _, err = a.Capture("Call the garage", SourceApp); return })
	do(t, a, func() (err error) { two, _, err = a.Capture("Buy milk", SourceApp); return })
	do(t, a, func() error { return a.ProcessTrash(one.ID) })
	do(t, a, func() error { return a.ProcessTwoMinute(two.ID) })

	texts := func() []string {
		items, _ := a.Inbox()
		var out []string
		for _, it := range items {
			out = append(out, it.Text)
		}
		return out
	}
	for i, want := range [][]string{
		{"Buy milk"},
		{"Call the garage", "Buy milk"},
		{"Call the garage"},
		nil,
	} {
		mustUndo(t, a)
		if got := texts(); !reflect.DeepEqual(got, want) {
			t.Fatalf("after %d undo(s) the inbox holds %v, want %v", i+1, got, want)
		}
	}
	if a.CanUndo() {
		t.Fatal("four steps taken back and there is still one offered")
	}
	if _, err := a.Undo(1); !errors.Is(err, ErrNothingToUndo) {
		t.Fatalf("undoing nothing: %v", err)
	}
}

// One press is one step, however many transactions it took: a Save that
// writes the project and then its action is taken back together.
func TestAGestureOfSeveralWritesIsOneStep(t *testing.T) {
	a, _ := newTestApp(t)
	p := shed(t, a)
	id := p.Actions[0].ID
	do(t, a, func() error {
		if err := a.UpdateProject(p.ID, ProjectFields{Title: "Shed finished", DOD: "Roof on"}); err != nil {
			return err
		}
		if err := a.UpdateAction(id, ActionFields{Title: "Pour the slab"}); err != nil {
			return err
		}
		// a pick is never audited and never a step of its own, and here it
		// rides with the step the same press made
		return a.ToggleTag("action", id, TodayTag)
	})
	step := mustUndo(t, a)
	if len(step.Entries) != 2 {
		t.Fatalf("the step read out %d entries, want the two edits", len(step.Entries))
	}
	proj, _ := a.Project(p.ID)
	act, _ := a.Action(id)
	if proj.Title != "Shed built" || act.Title != "Pour the base" || contains(act.Tags, TodayTag) {
		t.Fatalf("half of the press is still there: %q, %q, %v", proj.Title, act.Title, act.Tags)
	}
}

// What was not done at the keyboard is not a step: a capture through the API
// is written outside any gesture, and `u` must reach past it to the last thing
// that was done — without the arrival taking the id of what is coming back.
func TestWritesOutsideAGestureAreNeverUndone(t *testing.T) {
	a, _ := newTestApp(t)
	var mine *InboxItem
	do(t, a, func() (err error) { mine, _, err = a.Capture("Call the garage", SourceApp); return })
	do(t, a, func() error { return a.ProcessTrash(mine.ID) })
	// the inbox is empty, which is exactly when a plain rowid would be reused
	mail, _, err := a.Capture("Re: the quote", "mail")
	if err != nil {
		t.Fatal(err)
	}
	if mail.ID == mine.ID {
		t.Fatal("the arrival took the id of the capture that was just trashed")
	}

	step := mustUndo(t, a)
	if step.Entries[0].Event != EvTrashed {
		t.Fatalf("undo took back %q, not the trashing", step.Entries[0].Event)
	}
	items, _ := a.Inbox()
	if len(items) != 2 {
		t.Fatalf("the inbox holds %d items, want the mail and the capture that came back", len(items))
	}
	// and the day boundary is nobody's gesture either
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	last, _ := a.LastUndo()
	if last == nil || last.Entries[0].Event != EvCreated || last.Entries[0].Name != "Call the garage" {
		t.Fatalf("the last step is %+v, want my own capture", last)
	}
}

// A pick for today is never audited, so it is never a step — and one made on
// an item whose creation is then undone must not be left for the next item
// that is given the id.
func TestAPickIsNotAStepAndLeavesNothingBehind(t *testing.T) {
	a, _ := newTestApp(t)
	var act *Action
	do(t, a, func() (err error) { act, err = a.CreateAction(0, ActionFields{Title: "Buy milk"}); return })
	do(t, a, func() error { return a.ToggleTag("action", act.ID, TodayTag) })

	last, _ := a.LastUndo()
	if last.Entries[0].Event != EvCreated {
		t.Fatalf("the pick became a step: %+v", last.Entries[0])
	}
	mustUndo(t, a)
	var next *Action
	do(t, a, func() (err error) { next, err = a.CreateAction(0, ActionFields{Title: "Buy bread"}); return })
	got, _ := a.Action(next.ID)
	if contains(got.Tags, TodayTag) {
		t.Fatal("a new action was born picked for today, off a pick made on one that no longer exists")
	}
}

// The question names the step it read out, and an answer about a step that is
// no longer the last one is refused: something was done in between.
func TestUndoRefusesAStepThatIsNoLongerLast(t *testing.T) {
	a, _ := newTestApp(t)
	var it *InboxItem
	do(t, a, func() (err error) { it, _, err = a.Capture("Buy milk", SourceApp); return })
	asked, _ := a.LastUndo()
	do(t, a, func() error { return a.EditInboxText(it.ID, "Buy oat milk") })
	if _, err := a.Undo(asked.ID); !errors.Is(err, ErrUndoMoved) {
		t.Fatalf("answering about a stale step: %v", err)
	}
	if got, _ := a.InboxItem(it.ID); got == nil || got.Text != "Buy oat milk" {
		t.Fatal("the refusal changed something")
	}
}

// The log is added to and never rewritten: the step's entries stay, and one
// more says it was taken back, naming them (design.md, "Undo").
func TestUndoIsWrittenIntoTheAuditLog(t *testing.T) {
	a, _ := newTestApp(t)
	var it *InboxItem
	do(t, a, func() (err error) { it, _, err = a.Capture("Buy milk", SourceApp); return })
	do(t, a, func() error { return a.ProcessTrash(it.ID) })
	mustUndo(t, a)

	log, _ := a.AuditLog(10)
	var events []string
	for _, e := range log {
		events = append(events, e.Event)
	}
	if !reflect.DeepEqual(events, []string{EvUndo, EvTrashed, EvCreated}) {
		t.Fatalf("the log reads %v", events)
	}
	if got := log[0].Text(); got != "trashed: Buy milk" {
		t.Fatalf("the undo entry says %q", got)
	}
	// and the undo is not itself a step: the next one back is the capture
	last, _ := a.LastUndo()
	if last == nil || last.Entries[0].Event != EvCreated {
		t.Fatalf("after an undo the last step is %+v", last)
	}
}

// A database made before inbox ids were AUTOINCREMENT is rebuilt on opening,
// with every row where it was (implementation.md, "Undo").
func TestAnOlderInboxKeepsItsRowsAndStopsReusingIds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	a, err := Open(path, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	// Close first: it is what takes the triggers off, and the table below is
	// then rebuilt in the old shape by hand
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`DROP TABLE inbox_items`,
		`CREATE TABLE inbox_items (id INTEGER PRIMARY KEY, text TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL)`,
		`INSERT INTO inbox_items (id, text, source, created_at) VALUES (7, 'Pay the rent', 'mail', '2026-09-01T08:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	db.Close()

	a2, err := Open(path, time.UTC)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer a2.Close()
	items, _ := a2.Inbox()
	if len(items) != 1 || items[0].ID != 7 || items[0].Source != "mail" {
		t.Fatalf("the inbox came back as %+v", items)
	}
	if err := a2.ProcessTrash(7); err != nil {
		t.Fatal(err)
	}
	next, _, _ := a2.Capture("Buy milk", SourceApp)
	if next.ID == 7 {
		t.Fatal("the id of a deleted capture was handed out again")
	}
}
