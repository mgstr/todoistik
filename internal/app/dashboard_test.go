package app

import (
	"testing"
	"time"
)

func segs(s Side) map[string]string {
	out := map[string]string{}
	for _, g := range s.Segs {
		out[g.Name] = g.Text
	}
	return out
}

// Traffic counts what the app did, so the test does some and checks the
// counting. The parts that are easy to get subtly wrong are the ones this
// pins: which day a thing lands on, which side and segment it is, and what a
// row says about a period the record does not reach.
func TestTrafficCounts(t *testing.T) {
	a, _ := newTestApp(t)

	for _, c := range []struct{ text, source string }{
		{"Buy tyres", SourceApp}, {"Ask about the roof", SourceApp},
		{"A newsletter", "telegram"}, {"The boiler's serial", SourceApp},
	} {
		if _, _, err := a.Capture(c.text, c.source); err != nil {
			t.Fatal(err)
		}
	}
	inbox, _ := a.Inbox()
	act, err := a.ProcessAction(inbox[0].ID, ActionFields{Title: "Book the tyre change"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	if err := a.ProcessTrash(inbox[2].ID); err != nil {
		t.Fatal(err)
	}
	if err := a.ProcessTwoMinute(inbox[3].ID); err != nil {
		t.Fatal(err)
	}

	m, err := a.Traffic()
	if err != nil {
		t.Fatal(err)
	}
	// seven days, twelve months, the year
	if len(m.Rows) != 20 {
		t.Fatalf("the form has %d rows, want 20", len(m.Rows))
	}
	today := m.Rows[0]
	if !today.Now || today.Label != "Fri 4 Sep" {
		t.Errorf("the first row is %q (now=%v), want today", today.Label, today.Now)
	}
	if today.Left.Total != "4" {
		t.Errorf("today took in %q, want 4", today.Left.Total)
	}
	if got := segs(today.Left); got[SourceApp] != "3" || got["telegram"] != "1" {
		t.Errorf("today's captures by channel: %v", got)
	}
	// becoming a task is not leaving; finishing it is
	if today.Right.Total != "3" {
		t.Errorf("today sent out %q, want 3", today.Right.Total)
	}
	if got := segs(today.Right); got["done"] != "1" || got["trashed"] != "1" || got["done on the spot"] != "1" {
		t.Errorf("today's leavings by kind: %v", got)
	}
	// the longest bar on the form is the full track, and nothing overruns it
	if w := today.Left.Segs[0].Weight + today.Left.Segs[1].Weight; w != 1000 || today.Left.Rest != 0 {
		t.Errorf("the longest bar weighs %d with %d left over, want 1000 and 0", w, today.Left.Rest)
	}

	// yesterday is before the record: a row with nothing on it, not a zero
	if y := m.Rows[1]; y.Left.Known || y.Left.Total != "" || len(y.Left.Segs) != 0 {
		t.Errorf("a day before the record says %+v", y.Left)
	}
	// the month and the year hold one day of record, so their rate is that day
	month, year := m.Rows[7], m.Rows[19]
	if month.Label != "Sep 2026" || !month.Gap || month.Left.Total != "4.0" {
		t.Errorf("the month's row is %q gap=%v total=%q", month.Label, month.Gap, month.Left.Total)
	}
	if year.Label != "2026" || year.Left.Total != "4.0" {
		t.Errorf("the year's row is %q total=%q", year.Label, year.Left.Total)
	}
	if m.Rows[8].Left.Known {
		t.Error("a month before the record is drawn as though it were known")
	}
	if m.Left.Big != "4" || m.Right.Big != "3" {
		t.Errorf("the headlines are %q in and %q out, want 4 and 3", m.Left.Big, m.Right.Big)
	}
}

// A month's row is a rate over the days the record covers and that have
// happened — not over the whole month, which would draw the month the app was
// first used in as a collapse.
func TestTrafficAveragesOverTheDaysOnRecord(t *testing.T) {
	a, now := newTestApp(t)
	if _, _, err := a.Capture("One", SourceApp); err != nil {
		t.Fatal(err)
	}
	*now = now.AddDate(0, 0, 3) // Mon 7 Sep: four days on record
	if _, _, err := a.Capture("Two", SourceApp); err != nil {
		t.Fatal(err)
	}
	m, err := a.Traffic()
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Rows[7].Left.Total; got != "0.5" {
		t.Errorf("two captures over four days read %q a day, want 0.5", got)
	}
	// the days between are on record and empty, which is a zero and says so
	if y := m.Rows[1]; !y.Left.Known || y.Left.Total != "0" {
		t.Errorf("an empty day on record says %+v", y.Left)
	}
}

// More channels than there are colours for them: the smallest are one segment.
func TestTrafficFoldsTheSmallChannelsIntoOther(t *testing.T) {
	a, _ := newTestApp(t)
	n := 0
	for i, source := range []string{"app", "mail", "telegram", "mcp", "reminders"} {
		for j := 0; j < 5-i; j++ {
			n++
			if _, _, err := a.Capture("Item "+string(rune('a'+n)), source); err != nil {
				t.Fatal(err)
			}
		}
	}
	m, err := a.Traffic()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Left.Series) != maxSources || m.Left.Series[maxSources-1].Name != "other" {
		t.Fatalf("the legend is %+v", m.Left.Series)
	}
	// the three largest keep their names, in alphabetical order, and mcp and
	// reminders are the two folded
	if got := segs(m.Rows[0].Left); got["other"] != "3" || got["app"] != "5" || got["mcp"] != "" {
		t.Errorf("today's captures by channel: %v", got)
	}
	if m.Left.Series[0].Name != "app" || m.Left.Series[1].Name != "mail" || m.Left.Series[2].Name != "telegram" {
		t.Errorf("the channels are not in alphabetical order: %+v", m.Left.Series)
	}
}

// The queue is a level, and the rule that is easy to get wrong is the one
// that makes it a queue: a project is one next action however long its plan.
func TestQueueCountsAProjectOnce(t *testing.T) {
	a, now := newTestApp(t)
	if _, err := a.CreateAction(0, ActionFields{Title: "Call the garage"}); err != nil {
		t.Fatal(err)
	}
	solo, err := a.CreateAction(0, ActionFields{Title: "Pay the rent"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateProject(ProjectFields{Title: "Roof repaired", DOD: "No leak after rain"}, []ActionFields{
		{Title: "Ask for a quote"}, {Title: "Book the roofer"}, {Title: "Pay the roofer"},
	}); err != nil {
		t.Fatal(err)
	}

	m, err := a.Queue()
	if err != nil {
		t.Fatal(err)
	}
	today := m.Rows[0]
	if got := segs(today.Right); got["tasks"] != "2" || got["next actions"] != "1" {
		t.Errorf("the queue today: %v", got)
	}
	if got := segs(today.Left); got["projects"] != "1" {
		t.Errorf("what is kept today: %v", got)
	}
	if m.Right.Big != "3" || m.Right.Unit != "now" {
		t.Errorf("the headline is %q %q, want 3 now", m.Right.Big, m.Right.Unit)
	}

	// a day is counted as it stood at its end: the task finished the next
	// morning was still open when yesterday closed
	*now = now.AddDate(0, 0, 1)
	if err := a.CompleteAction(solo.ID); err != nil {
		t.Fatal(err)
	}
	m, err = a.Queue()
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Rows[0].Right.Total; got != "2" {
		t.Errorf("today's queue is %q, want 2", got)
	}
	if got := m.Rows[1].Right.Total; got != "3" {
		t.Errorf("yesterday's queue is %q, want 3", got)
	}
	// the month is the average of its two days, 2.5, written whole
	if got := m.Rows[7].Right.Total; got != "2" && got != "3" {
		t.Errorf("the month's queue is %q", got)
	}
}

func zeroCell(y *ZeroYear, m time.Month, day int) ZeroDay { return y.Months[m-1].Days[day-1] }

// A day is empty if the inbox held nothing at any moment of it, and that
// includes a day nothing happened on at all.
func TestInboxZeroYear(t *testing.T) {
	a, now := newTestApp(t)

	// Fri 4 Sep: one in at ten, answered at one
	it, _, err := a.Capture("Buy tyres", SourceApp)
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(3 * time.Hour)
	if err := a.ProcessTrash(it.ID); err != nil {
		t.Fatal(err)
	}
	// Sat 5 Sep: nothing. Sun 6 Sep: one in, and it stays
	*now = now.AddDate(0, 0, 2)
	if _, _, err := a.Capture("Ask about the roof", SourceApp); err != nil {
		t.Fatal(err)
	}
	// Mon 7 Sep: still there all day, and a second one joins it
	*now = now.AddDate(0, 0, 1)
	if _, _, err := a.Capture("A newsletter", SourceApp); err != nil {
		t.Fatal(err)
	}

	y, err := a.InboxZeroYear()
	if err != nil {
		t.Fatal(err)
	}
	if y.Year != 2026 || len(y.Months) != 12 || len(y.Months[1].Days) != 31 {
		t.Fatalf("the grid is %d, %d months", y.Year, len(y.Months))
	}
	for _, want := range []struct {
		day          int
		state, title string
		today        bool
	}{
		{3, ZeroNone, "", false}, // before the record
		// it began the day empty, which is empty
		{4, ZeroEmpty, "Fri 4 Sep · empty", false},
		{5, ZeroEmpty, "Sat 5 Sep · empty", false},
		// empty until the capture arrived
		{6, ZeroEmpty, "Sun 6 Sep · empty", false},
		{7, ZeroOpen, "Mon 7 Sep · never below 1", true},
		{8, ZeroNone, "", false}, // still to come
	} {
		c := zeroCell(y, time.September, want.day)
		if c.State != want.state || c.Title != want.title || c.Today != want.today {
			t.Errorf("%d Sep is %+v, want %+v", want.day, c, want)
		}
	}
	// the 30th of February is a hole, not an unknown day
	if c := zeroCell(y, time.February, 30); c.State != "" {
		t.Errorf("30 Feb has a state: %q", c.State)
	}
	if y.Reached != 3 || y.Never != 1 {
		t.Errorf("%d days reached zero and %d did not, want 3 and 1", y.Reached, y.Never)
	}

	// and a day that started full and was emptied says when
	inbox, _ := a.Inbox()
	*now = now.AddDate(0, 0, 1)
	for _, it := range inbox {
		if err := a.ProcessTrash(it.ID); err != nil {
			t.Fatal(err)
		}
	}
	if y, err = a.InboxZeroYear(); err != nil {
		t.Fatal(err)
	}
	if c := zeroCell(y, time.September, 8); c.State != ZeroEmpty || c.Title != "Tue 8 Sep · empty at 13:00" {
		t.Errorf("the day the inbox was emptied is %+v", c)
	}
}
