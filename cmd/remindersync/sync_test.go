package main

import (
	"testing"
	"time"
)

var runAt = time.Date(2026, 9, 10, 9, 0, 0, 0, time.Local)

// reminderFor is where an item stops being an item: the name it is read by,
// the marker it is recognised by, and the two fields Reminders can hold.
func TestReminderFor(t *testing.T) {
	cases := []struct {
		name string
		in   viewItem
		want desired
	}{
		{
			"a title carries the marker",
			viewItem{ID: 15, Title: "залогировать кеш 8 сентября"},
			desired{ItemID: 15, Name: "залогировать кеш 8 сентября", Title: "залогировать кеш 8 сентября (::15)"},
		},
		{
			"the description becomes the note and the due date the due date",
			viewItem{ID: 7, Title: "Replace the log", Description: "bring a pencil", DueDate: "2026-09-15"},
			desired{ItemID: 7, Name: "Replace the log", Title: "Replace the log (::7)", Body: "bring a pencil", Due: "2026-09-15"},
		},
		{
			"an item that is text rather than a title still goes across",
			viewItem{ID: 3, Text: "Buy new winter tyres"},
			desired{ItemID: 3, Name: "Buy new winter tyres", Title: "Buy new winter tyres (::3)"},
		},
		{
			"a note's own line breaks do not survive, so two runs compare equal",
			viewItem{ID: 7, Title: "Replace the log", Description: "bring a pencil\nand a bag\n"},
			desired{ItemID: 7, Name: "Replace the log", Title: "Replace the log (::7)", Body: "bring a pencil and a bag"},
		},
		{
			"an item with nothing to call it makes no reminder",
			viewItem{ID: 9, Title: "  "},
			desired{ItemID: 9},
		},
		{
			"an item with no id cannot be recognised later, so it gets no title",
			viewItem{Title: "Replace the log"},
			desired{Name: "Replace the log"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := reminderFor(c.in); got != c.want {
				t.Errorf("reminderFor() = %+v, want %+v", got, c.want)
			}
		})
	}
}

// requestLine is the one thing this program says back to the app, and the app
// reads it strictly — so the moment and the name have to come out right.
func TestRequestLine(t *testing.T) {
	r := reminder{ID: "x", Name: "залогировать кеш (::15)", Completed: true, DoneAt: "2026-09-08T14:30"}
	want := "Completion request from reminders ::15 ::2026-09-08T14:30 залогировать кеш"
	if got := requestLine(r, 15, runAt); got != want {
		t.Errorf("requestLine() = %q, want %q", got, want)
	}

	// ticked, but Reminders kept no moment: the run's own clock is the closest
	// honest answer, and a midnight would be a fiction the Archive repeats
	r.DoneAt = ""
	want = "Completion request from reminders ::15 ::2026-09-10T09:00 залогировать кеш"
	if got := requestLine(r, 15, runAt); got != want {
		t.Errorf("requestLine() with no moment = %q, want %q", got, want)
	}
}

// plan is the whole of the export's behavior, both directions of it: what it
// writes, what it takes away, and what it files back.
func TestPlan(t *testing.T) {
	t.Run("an item the list does not hold is created, with its marker", func(t *testing.T) {
		p := plan([]desired{{ItemID: 15, Name: "A", Title: "A (::15)"}}, nil, runAt)
		if len(p.Create) != 1 || p.Create[0].Title != "A (::15)" {
			t.Fatalf("Create = %+v", p.Create)
		}
		if len(p.Delete) != 0 || len(p.Update) != 0 || len(p.Requests) != 0 {
			t.Errorf("nothing else should have happened: %+v", p)
		}
	})

	t.Run("a second run over the same view changes nothing", func(t *testing.T) {
		want := []desired{{ItemID: 15, Name: "A", Title: "A (::15)", Body: "note", Due: "2026-09-15"}}
		have := []reminder{{ID: "1", Name: "A (::15)", Body: "note", AllDay: "2026-09-15"}}
		p := plan(want, have, runAt)
		if len(p.Create) != 0 || len(p.Update) != 0 || len(p.Delete) != 0 || len(p.Requests) != 0 {
			t.Fatalf("the run should have been idle: %+v", p)
		}
		if len(p.Unchanged) != 1 {
			t.Errorf("Unchanged = %+v, want one", p.Unchanged)
		}
	})

	t.Run("renaming the item renames its reminder instead of stranding it", func(t *testing.T) {
		want := []desired{{ItemID: 15, Name: "B", Title: "B (::15)"}}
		have := []reminder{{ID: "1", Name: "A (::15)"}}
		p := plan(want, have, runAt)
		if len(p.Create) != 0 || len(p.Delete) != 0 {
			t.Fatalf("the reminder should have been kept: %+v", p)
		}
		if len(p.Update) != 1 || p.Update[0].Title != "B (::15)" {
			t.Fatalf("Update = %+v, want the new title", p.Update)
		}
	})

	t.Run("a marker no item in the view holds is deleted", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "A (::15)"}, {ID: "2", Name: "old (::99)"}}
		p := plan([]desired{{ItemID: 15, Name: "A", Title: "A (::15)"}}, have, runAt)
		if len(p.Delete) != 1 || p.Delete[0] != "2" {
			t.Fatalf("Delete = %v, want [2]", p.Delete)
		}
	})

	t.Run("a reminder with no marker is deleted, whoever typed it", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "typed straight into Reminders"}}
		p := plan(nil, have, runAt)
		if len(p.Delete) != 1 || len(p.Requests) != 0 {
			t.Errorf("Delete = %v, Requests = %+v", p.Delete, p.Requests)
		}
	})

	t.Run("a ticked reminder files a request and goes, and is not recreated", func(t *testing.T) {
		want := []desired{{ItemID: 15, Name: "A", Title: "A (::15)"}}
		have := []reminder{{ID: "1", Name: "A (::15)", Completed: true, DoneAt: "2026-09-08T14:30"}}
		p := plan(want, have, runAt)
		if len(p.Requests) != 1 || p.Requests[0].reminder.ID != "1" {
			t.Fatalf("Requests = %+v, want the ticked one", p.Requests)
		}
		want0 := "Completion request from reminders ::15 ::2026-09-08T14:30 A"
		if p.Requests[0].line != want0 {
			t.Errorf("line = %q, want %q", p.Requests[0].line, want0)
		}
		if len(p.Create) != 0 {
			t.Errorf("Create = %+v, want none: the request is in flight and the answer decides", p.Create)
		}
		if len(p.Delete) != 0 {
			t.Errorf("Delete = %v, want none: it goes only once its line has been filed", p.Delete)
		}
	})

	t.Run("a ticked reminder files its request even when its item left the view", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "A (::15)", Completed: true, DoneAt: "2026-09-08T14:30"}}
		p := plan(nil, have, runAt)
		if len(p.Requests) != 1 {
			t.Fatalf("Requests = %+v, want one: a tick is never lost", p.Requests)
		}
		if len(p.Delete) != 0 {
			t.Errorf("Delete = %v, want none: it is filed first, then deleted", p.Delete)
		}
	})

	t.Run("a ticked reminder with no marker names nothing and simply goes", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "typed and ticked", Completed: true}}
		p := plan(nil, have, runAt)
		if len(p.Requests) != 0 {
			t.Errorf("Requests = %+v, want none: there is no item to name", p.Requests)
		}
		if len(p.Delete) != 1 {
			t.Errorf("Delete = %v, want [1]", p.Delete)
		}
	})

	t.Run("an item whose reminder was ticked but which also has an open one is left alone", func(t *testing.T) {
		want := []desired{{ItemID: 15, Name: "A", Title: "A (::15)"}}
		have := []reminder{
			{ID: "1", Name: "A (::15)", Completed: true, DoneAt: "2026-09-08T14:30"},
			{ID: "2", Name: "A (::15)"},
		}
		p := plan(want, have, runAt)
		if len(p.Requests) != 1 {
			t.Errorf("Requests = %+v, want the ticked one", p.Requests)
		}
		if len(p.Create) != 0 || len(p.Delete) != 0 {
			t.Errorf("the open one is still the item's reminder: %+v", p)
		}
	})

	t.Run("a moved due date is written through", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "A (::15)", AllDay: "2026-09-15"}}
		p := plan([]desired{{ItemID: 15, Name: "A", Title: "A (::15)", Due: "2026-09-16"}}, have, runAt)
		if len(p.Update) != 1 || p.Update[0].Due != "2026-09-16" || p.Update[0].ID != "1" {
			t.Fatalf("Update = %+v, want 1 due 2026-09-16", p.Update)
		}
		if p.Update[0].Body != "" || p.Update[0].Title != "" {
			t.Errorf("only the due date moved: %+v", p.Update[0])
		}
	})

	t.Run("a time set on the phone survives a due date on the same day", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "A (::15)", Due: "2026-09-15 14:30"}}
		p := plan([]desired{{ItemID: 15, Name: "A", Title: "A (::15)", Due: "2026-09-15"}}, have, runAt)
		if len(p.Update) != 0 {
			t.Errorf("Update = %+v, want none: the day already agrees", p.Update)
		}
	})

	t.Run("what todoistik does not know is not erased", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "A (::15)", Body: "typed on the phone", AllDay: "2026-09-15"}}
		p := plan([]desired{{ItemID: 15, Name: "A", Title: "A (::15)"}}, have, runAt)
		if len(p.Update) != 0 {
			t.Errorf("Update = %+v, want none: an empty field is not an instruction to clear one", p.Update)
		}
	})

	t.Run("a note that moved is written through", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "A (::15)", Body: "old"}}
		p := plan([]desired{{ItemID: 15, Name: "A", Title: "A (::15)", Body: "new"}}, have, runAt)
		if len(p.Update) != 1 || p.Update[0].Body != "new" {
			t.Fatalf("Update = %+v, want the new note", p.Update)
		}
	})

	t.Run("whitespace is not a difference on either side", func(t *testing.T) {
		have := []reminder{{ID: "1", Name: "Replace  the log   (::7)"}}
		p := plan([]desired{{ItemID: 7, Name: "Replace the log", Title: "Replace the log (::7)"}}, have, runAt)
		if len(p.Create) != 0 || len(p.Delete) != 0 || len(p.Update) != 0 {
			t.Errorf("the same reminder should have matched unchanged: %+v", p)
		}
	})
}

// dueDay is what a reminder's due date is compared as: an action's due date is
// a day, so a reminder due at a moment has to answer with its day.
func TestDueDay(t *testing.T) {
	cases := []struct {
		in   reminder
		want string
	}{
		{reminder{AllDay: "2026-09-15"}, "2026-09-15"},
		{reminder{Due: "2026-09-15 14:30"}, "2026-09-15"},
		{reminder{}, ""},
	}
	for _, c := range cases {
		if got := c.in.dueDay(); got != c.want {
			t.Errorf("dueDay(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}
