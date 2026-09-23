package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// An inbox item says which way in it arrived by, and that is the whole point
// of the field: it is written at capture time and nowhere else (design.md,
// "Inbox item").
func TestCaptureRecordsTheWayItArrivedBy(t *testing.T) {
	a, _ := newTestApp(t)
	it, acc, err := a.Capture("Milk, bread, the good coffee", "reminders")
	if err != nil || !acc {
		t.Fatalf("capture: %v %v", acc, err)
	}
	if it.Source != "reminders" {
		t.Fatalf("captured under %q, want reminders", it.Source)
	}
	items, err := a.Inbox()
	if err != nil || len(items) != 1 {
		t.Fatalf("inbox: %v %v", len(items), err)
	}
	if items[0].Source != "reminders" {
		t.Fatalf("the inbox reads it back as %q", items[0].Source)
	}
	one, err := a.InboxItem(it.ID)
	if err != nil || one.Source != "reminders" {
		t.Fatalf("one item reads it back as %q (%v)", one.Source, err)
	}
}

// The stats this field exists for are read off the audit log, not off the
// inbox: an item is answered and gone within the week, and "which channel do I
// use most" is a question about a year. The snapshot is what makes that
// answerable, and it is written by the ordinary create entry.
func TestTheCaptureAuditEntryCarriesTheSource(t *testing.T) {
	a, _ := newTestApp(t)
	it, _, err := a.Capture("Chase the insurance quote", "telegram")
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ProcessTrash(it.ID); err != nil {
		t.Fatal(err)
	}
	if items, _ := a.Inbox(); len(items) != 0 {
		t.Fatalf("the item should be gone from the inbox, %d left", len(items))
	}
	entries, err := a.AuditLog(0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Event == EvCreated && e.ItemType == "inbox" && strings.Contains(e.Snapshot, `"source":"telegram"`) {
			found = true
		}
	}
	if !found {
		t.Fatal("the channel has to survive the item, or a year of captures cannot be counted")
	}
}

// Every way in the app itself writes names one channel, and those four are the
// ones the domain owns. The other four live in the programs that talk to the
// app over HTTP, which never import this package.
func TestTheAppsOwnSourcesAreTheOnesItWrites(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.CreateSchedule("Water the plants", "* * *", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.DayStart(); err != nil {
		t.Fatal(err)
	}
	items, _ := a.Inbox()
	if len(items) != 1 {
		t.Fatalf("the schedule should have fired once, %d items", len(items))
	}
	if items[0].Source != SourceSchedule {
		t.Fatalf("a firing is captured as %q, want %q", items[0].Source, SourceSchedule)
	}

	// an idea sent back to the inbox is a capture too, and it is the app's own
	idea, _, err := a.Capture("Learn to sail", SourceApp)
	if err != nil {
		t.Fatal(err)
	}
	sm, err := a.ProcessSomeday(idea.ID, SomedayFields{Text: "Learn to sail"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ReturnToInbox(sm.ID); err != nil {
		t.Fatal(err)
	}
	items, _ = a.Inbox()
	back := items[len(items)-1]
	if back.Source != SourceSomeday {
		t.Fatalf("an idea sent back is captured as %q, want %q", back.Source, SourceSomeday)
	}
}

// Capture may fail for an empty text and for nothing else, so a name this app
// has never seen is written down rather than argued with — and folded to the
// one spelling it is counted under, or the aggregation would show a channel
// twice for having been shouted once.
func TestASourceIsFoldedAndNeverRefused(t *testing.T) {
	for _, c := range []struct{ sent, want string }{
		{"Telegram", "telegram"},
		{"  reminders  ", "reminders"},
		{"shortcuts-ios", "shortcuts-ios"},
		{"watch app!", "watchapp"},
		{"", ""},
		{strings.Repeat("x", 100), strings.Repeat("x", sourceMax)},
	} {
		if got := NormalizeSource(c.sent); got != c.want {
			t.Errorf("%q folded to %q, want %q", c.sent, got, c.want)
		}
	}

	a, _ := newTestApp(t)
	it, acc, err := a.Capture("Ask about the warranty", "A Brand New Thing")
	if err != nil || !acc {
		t.Fatalf("an unknown channel must still get its capture in: %v %v", acc, err)
	}
	if it.Source != "abrandnewthing" {
		t.Fatalf("recorded as %q", it.Source)
	}
}

// A dropped duplicate puts nothing anywhere (design.md, "Duplicate captures"),
// so the second channel's attempt is not recorded and the first item keeps the
// source it was captured under. What the counts therefore say is which channel
// puts items in the inbox, not which one was reached for.
func TestADuplicateFromAnotherChannelChangesNothing(t *testing.T) {
	a, _ := newTestApp(t)
	if _, _, err := a.Capture("Pay the rent", "telegram"); err != nil {
		t.Fatal(err)
	}
	if _, acc, err := a.Capture("Pay the rent", "mail"); err != nil || acc {
		t.Fatalf("still a duplicate, whichever way it arrived: acc=%v err=%v", acc, err)
	}
	items, _ := a.Inbox()
	if len(items) != 1 || items[0].Source != "telegram" {
		t.Fatalf("%d items, the first captured under %q", len(items), items[0].Source)
	}
}

// A database made before the field existed gets the column and keeps its
// items, which say nothing about where they came from rather than claiming a
// channel they never had (implementation.md, "Schema changes").
func TestAnOlderDatabaseGetsTheColumnAndNoBackfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	a, err := Open(path, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Pay the rent", "telegram"); err != nil {
		t.Fatal(err)
	}
	// put the database back the way it was before the field
	if _, err := a.db.Exec(`ALTER TABLE inbox_items DROP COLUMN source`); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	a2, err := Open(path, time.UTC)
	if err != nil {
		t.Fatalf("an older database should open: %v", err)
	}
	defer a2.Close()
	items, err := a2.Inbox()
	if err != nil || len(items) != 1 {
		t.Fatalf("inbox: %d items (%v)", len(items), err)
	}
	if items[0].Source != "" {
		t.Fatalf("an item from before the field says %q, and should say nothing", items[0].Source)
	}
	// and the column is there to be written from now on
	if _, _, err := a2.Capture("Book the tyre change", "mail"); err != nil {
		t.Fatal(err)
	}
	items, _ = a2.Inbox()
	if items[1].Source != "mail" {
		t.Fatalf("a capture after the migration says %q", items[1].Source)
	}
}

// The stamp is the moment of capture and not the day of it: an item captured
// at 17:42 is a thought you can place, and design.md, "Time fields" makes this
// the one field with a time of day on it.
func TestTheCaptureStampKeepsItsHour(t *testing.T) {
	a, now := newTestApp(t)
	*now = now.Add(7*time.Hour + 42*time.Minute) // 17:42 on the test's Friday
	it, _, err := a.Capture("Ask the neighbour about the fence", SourceApp)
	if err != nil {
		t.Fatal(err)
	}
	if got := it.CreatedAt.Format("15:04"); got != "17:42" {
		t.Fatalf("captured at %q", got)
	}
	items, _ := a.Inbox()
	if got := items[0].CreatedAt.Format("15:04"); got != "17:42" {
		t.Fatalf("read back as %q — the hour has to survive the round trip", got)
	}
}
