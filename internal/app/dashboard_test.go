package app

import (
	"testing"
	"time"
)

// The Dashboard counts what the app did, so the test does some and checks the
// counting. Every figure on that screen is a claim about the record, and the
// ones that are easy to get subtly wrong are the ones with a window on them:
// the week, the month, and the pairing that measures how long something took.
func TestDashboardCounts(t *testing.T) {
	a, _ := newTestApp(t)

	// four captures today, two of them answered
	for _, text := range []string{"Buy tyres", "Ask about the roof", "A newsletter", "The boiler's serial"} {
		if _, _, err := a.Capture(text, SourceApp); err != nil {
			t.Fatal(err)
		}
	}
	inbox, _ := a.Inbox()
	if len(inbox) != 4 {
		t.Fatalf("inbox holds %d, want 4", len(inbox))
	}
	if _, err := a.ProcessAction(inbox[0].ID, ActionFields{Title: "Book the tyre change"}, 0); err != nil {
		t.Fatal(err)
	}
	if err := a.ProcessTrash(inbox[2].ID); err != nil {
		t.Fatal(err)
	}

	d, err := a.Dashboard()
	if err != nil {
		t.Fatal(err)
	}

	// --- inbound: four today, and today is the last of the seven rows
	if len(d.Inbound.Days) != 7 {
		t.Fatalf("the week has %d rows, want 7", len(d.Inbound.Days))
	}
	today := d.Inbound.Days[6]
	if !today.Now {
		t.Error("the last row of the week is not marked as today")
	}
	if today.Count != 4 {
		t.Errorf("today captured %d, want 4", today.Count)
	}
	if d.Inbound.WeekTotal != 4 {
		t.Errorf("the week totals %d, want 4", d.Inbound.WeekTotal)
	}
	if len(d.Inbound.Months) != 12 || !d.Inbound.Months[11].Now {
		t.Errorf("the year has %d rows and ends on %v, want 12 ending on this month",
			len(d.Inbound.Months), d.Inbound.Months[len(d.Inbound.Months)-1].Now)
	}

	// --- what the inbox became: one of each, and no row for what did not happen
	got := map[string]int{}
	for _, r := range d.Became.Rows {
		got[r.Label] = r.Count
	}
	if got["an action"] != 1 || got["trashed"] != 1 {
		t.Errorf("what the inbox became: %v", got)
	}
	if _, ok := got["a project"]; ok {
		t.Error("a branch nobody took is drawn as a row")
	}
	if d.Became.Total != 2 {
		t.Errorf("the panel totals %d, want 2", d.Became.Total)
	}

	// --- where captures come from, read out of the snapshots
	if len(d.Sources.Rows) != 1 || d.Sources.Rows[0].Label != SourceApp || d.Sources.Rows[0].Count != 4 {
		t.Errorf("sources: %+v", d.Sources.Rows)
	}
	if d.Sources.Rows[0].Pct != 100 {
		t.Errorf("one channel is %d%% of the captures, want 100", d.Sources.Rows[0].Pct)
	}

	// --- the practice: two left in the inbox, so it is not empty now
	if d.Practice.InboxOpen != 2 {
		t.Errorf("the inbox holds %d, want 2", d.Practice.InboxOpen)
	}
	if d.Practice.InboxEverEmpty {
		t.Error("the inbox has never been empty and the panel says it has")
	}

	// --- the oldest thing in each view, one row per view that has anything
	views := map[string]bool{}
	for _, o := range d.Oldest {
		views[o.View] = true
	}
	if !views["inbox"] || !views["next"] {
		t.Errorf("the oldest panel covers %v", views)
	}

	// --- how much of Next is workable
	mix := map[string]int{}
	for _, r := range d.NextMix.Rows {
		mix[r.Label] = r.Count
	}
	if mix["workable now"] != 1 || d.NextMix.Total != 1 {
		t.Errorf("the Next mix: %v of %d", mix, d.NextMix.Total)
	}

	// --- emptying the inbox is what makes it empty, and the panel says when
	rest, _ := a.Inbox()
	for _, it := range rest {
		if err := a.ProcessTrash(it.ID); err != nil {
			t.Fatal(err)
		}
	}
	d, err = a.Dashboard()
	if err != nil {
		t.Fatal(err)
	}
	if !d.Practice.InboxEverEmpty || d.Practice.InboxOpen != 0 {
		t.Errorf("the inbox is empty and the panel says open=%d empty=%v",
			d.Practice.InboxOpen, d.Practice.InboxEverEmpty)
	}
}

// The two durations are the pairing rule, and the pairing rule is the thing
// most easily got wrong: an id is handed back out after a delete, so a leaving
// event has to be matched with the `created` entry immediately before it and
// not with whichever one happens to share the id.
func TestDashboardMeasuresHowLongThingsTook(t *testing.T) {
	a, now := newTestApp(t)

	it, _, err := a.Capture("Buy tyres", SourceApp)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(6 * time.Hour)
	act, err := a.ProcessAction(it.ID, ActionFields{Title: "Book the tyre change"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(48 * time.Hour)
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}

	d, err := a.Dashboard()
	if err != nil {
		t.Fatal(err)
	}
	if d.Ages.InInboxN != 1 || d.Ages.InInbox != 6*time.Hour {
		t.Errorf("time in the inbox: %v over %d items, want 6h over 1",
			d.Ages.InInbox, d.Ages.InInboxN)
	}
	if d.Ages.ToCompletionN != 1 || d.Ages.ToCompletion != 48*time.Hour {
		t.Errorf("time to completion: %v over %d items, want 48h over 1",
			d.Ages.ToCompletion, d.Ages.ToCompletionN)
	}

	// and the same id, handed out again, does not borrow the first one's clock
	it2, _, err := a.Capture("Ask about the roof", SourceApp)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Hour)
	if err := a.ProcessTrash(it2.ID); err != nil {
		t.Fatal(err)
	}
	d, err = a.Dashboard()
	if err != nil {
		t.Fatal(err)
	}
	if d.Ages.InInboxN != 2 {
		t.Fatalf("two items have left the inbox, the panel measured %d", d.Ages.InInboxN)
	}
	// 6h and 1h: the median of two is their mean, which is 3h30m
	if want := 3*time.Hour + 30*time.Minute; d.Ages.InInbox != want {
		t.Errorf("the median time in the inbox is %v, want %v", d.Ages.InInbox, want)
	}
}

func TestHumanDuration(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{0, "—"},
		{30 * time.Second, "under a minute"},
		{25 * time.Minute, "25 minutes"},
		{time.Minute, "1 minute"},
		{time.Hour, "1 hour"},
		{9 * time.Hour, "9 hours"},
		{3 * 24 * time.Hour, "3 days"},
		{21 * 24 * time.Hour, "3 weeks"},
		{90 * 24 * time.Hour, "3 months"},
		{400 * 24 * time.Hour, "1 year"},
	} {
		if got := HumanDuration(c.d); got != c.want {
			t.Errorf("HumanDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
