package app

import (
	"reflect"
	"strings"
	"testing"
)

// Reference material is kept in the app now, and this is the whole of what the
// branch does: the capture becomes an item with the words and the area it was
// filed under, the pile narrows by both, and the material stays editable
// afterwards because material is added to as more of it is found out
// (design.md, "Reference item").
func TestReferenceIsKeptTaggedAndEditable(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.AddTag("house"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("The boiler is a Vaillant ecoTEC 832", SourceApp); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("An article on soldering", SourceApp); err != nil {
		t.Fatal(err)
	}
	inbox, _ := a.Inbox()

	it, err := a.ProcessReference(inbox[0].ID, ReferenceFields{
		Text: "The boiler is a Vaillant ecoTEC 832", Tags: []string{"house"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProcessReference(inbox[1].ID, ReferenceFields{Text: "An article on soldering"}); err != nil {
		t.Fatal(err)
	}
	// the inbox is emptied by the answer, like every other branch
	if left, _ := a.Inbox(); len(left) != 0 {
		t.Fatalf("%d captures left in the inbox", len(left))
	}

	got, err := a.ReferenceItem(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Tags, []string{"house"}) {
		t.Fatalf("filed as %+v", got)
	}
	// newest first: the pile is read to find one thing in it, and what was
	// filed last is what is most often looked for again
	all, err := a.ReferenceItems(Filters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 || all[0].Text != "An article on soldering" {
		t.Fatalf("order: %+v", all)
	}
	// the two things the view narrows by, which is everything an item carries
	tagged, err := a.ReferenceItems(Filters{Tags: []string{"house"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tagged) != 1 || tagged[0].ID != it.ID {
		t.Fatalf("#house matched %d items, want the boiler", len(tagged))
	}
	named, err := a.ReferenceItems(Filters{Name: "soldering"})
	if err != nil {
		t.Fatal(err)
	}
	if len(named) != 1 || named[0].Text != "An article on soldering" {
		t.Fatalf("the name filter matched %+v", named)
	}

	// material is added to: the account number today, the portal it is typed
	// into next month
	if err := a.EditReference(it.ID, ReferenceFields{
		Text: "The boiler is a Vaillant ecoTEC 832\nservice code 3-2-1", Tags: []string{"house"}}); err != nil {
		t.Fatal(err)
	}
	got, _ = a.ReferenceItem(it.ID)
	if !strings.Contains(got.Text, "service code") {
		t.Fatalf("edit did not take: %+v", got)
	}
}

// The one way out, and it is a delete: material that is not worth keeping is
// not a decision waiting to be made, so there is no trip back to the inbox for
// it to take (design.md, "Reference"). The audit entry keeps the snapshot,
// which is what makes the press recoverable — and the tag rows go with it, or
// the name could never be removed again.
func TestDeletingReferenceLetsTheTagGo(t *testing.T) {
	a, _ := newTestApp(t)
	if err := a.AddTag("house"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("A manual nobody will read again", SourceApp); err != nil {
		t.Fatal(err)
	}
	inbox, _ := a.Inbox()
	it, err := a.ProcessReference(inbox[0].ID, ReferenceFields{
		Text: "A manual nobody will read again", Tags: []string{"house"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteReference(it.ID); err != nil {
		t.Fatal(err)
	}
	if left, _ := a.ReferenceItems(Filters{}); len(left) != 0 {
		t.Fatalf("%d items left", len(left))
	}
	if err := a.RemoveTag("house"); err != nil {
		t.Fatalf("the tag is still held by something: %v", err)
	}

	log, err := a.AuditLog(1000)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range log {
		if e.ItemType == "reference" && e.Event == EvDeleted {
			found = true
			if !strings.Contains(e.Snapshot, "A manual nobody will read again") {
				t.Fatalf("the snapshot is what recovery reads: %q", e.Snapshot)
			}
		}
	}
	if !found {
		t.Fatal("the delete is not in the log")
	}
}

// An empty box is refused, the way it is on every other form: the material is
// the item, and an item with no words is nothing kept.
func TestReferenceNeedsWords(t *testing.T) {
	a, _ := newTestApp(t)
	if _, _, err := a.Capture("Something", SourceApp); err != nil {
		t.Fatal(err)
	}
	inbox, _ := a.Inbox()
	if _, err := a.ProcessReference(inbox[0].ID, ReferenceFields{Text: "  "}); err == nil {
		t.Fatal("an empty reference item should have been refused")
	}
	// and the capture is still there to be answered again
	if left, _ := a.Inbox(); len(left) != 1 {
		t.Fatalf("the refused branch consumed the capture: %d left", len(left))
	}
}
