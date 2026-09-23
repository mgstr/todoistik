package app

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// The parked state is gone, and an existing database still has rows that were
// written under it: a project action with no becameNextActionAt. Opening such
// a database has to leave every action a next action, because that is now the
// only thing an action can be (design.md, "Standalone actions").
//
// The test stands a database up with the current schema, puts it back into the
// old shape by hand, and reopens it — which is exactly what migrate() has to
// survive, and cheaper than keeping an old binary around to write one.
func TestOpeningADatabaseWrittenWhenActionsCouldBeParked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	a, err := Open(path, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	p, err := a.CreateProject(ProjectFields{Title: "Shed built", DOD: "Roof on"},
		[]ActionFields{{Title: "Pour the base"}, {Title: "Paint it"}})
	if err != nil {
		t.Fatal(err)
	}
	base, paint := p.Actions[0].ID, p.Actions[1].ID
	if err := a.CompleteAction(base); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	// back to the old shape: no snooze_action_id, and the two ways a row could
	// arrive with no becameNextActionAt — parked and open, parked and completed
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`ALTER TABLE actions DROP COLUMN snooze_action_id`,
		`UPDATE actions SET became_next_at=NULL`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	a2, err := Open(path, time.UTC)
	if err != nil {
		t.Fatalf("reopening a database written under the old rules: %v", err)
	}
	defer a2.Close()

	open, err := a2.Action(paint)
	if err != nil {
		t.Fatal(err)
	}
	if open.BecameNextAt == nil {
		t.Fatal("an action that was parked must come back as a next action")
	}
	done, err := a2.Action(base)
	if err != nil {
		t.Fatal(err)
	}
	if done.BecameNextAt == nil {
		t.Fatal("a completed action must have a stamp too, so nothing reads as parked")
	}
	if done.CompletedAt != nil && done.BecameNextAt.After(*done.CompletedAt) {
		t.Fatal("a completed action cannot have become next after it was finished")
	}
	// the project reads as it should: one open action, and not stalled
	got, err := a2.Project(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Stalled {
		t.Fatal("a project whose formerly parked action is now next is not stalled")
	}
	// and the new column is there and usable
	if err := a2.UpdateAction(paint, ActionFields{Title: "Paint it"}); err != nil {
		t.Fatal(err)
	}
}
