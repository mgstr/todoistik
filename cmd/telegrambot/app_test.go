package main

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"todoistik/internal/apiclient"
	"todoistik/internal/app"
	"todoistik/internal/conf"
	"todoistik/internal/web"
)

// The stubs above answer in the shapes this program expects. This asks the
// real app, so a view that changes its shape fails here and not on the phone.
func TestTheRealAppsViewsReadAsLists(t *testing.T) {
	a, err := app.Open(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	s, err := web.New(a, "", conf.Config{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	b := &bot{app: apiclient.New(srv.URL, "")}

	today := a.Today()
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddContext("home", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Pay rent", DueDate: today}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Change the tyres", Tags: []string{"car"}}); err != nil {
		t.Fatal(err)
	}
	p, err := a.CreateProject(app.ProjectFields{Title: "Move flat", DOD: "keys handed over"}, []app.ActionFields{{Title: "Pack the books"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(p.Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	done, err := a.CreateAction(0, app.ActionFields{Title: "Renew the passport", DueDate: today})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(done.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSchedule("Pay the rent", "1 * *", ""); err != nil {
		t.Fatal(err)
	}
	b.answer("Buy milk\nthe oat one")

	for cmd, want := range map[string][]string{
		"/today":     {"Today · 1", "Out of time · 1", "• Pay rent · due " + today},
		"/next #car": {"Next actions · #car · 1", "• Change the tyres"},
		"/projects":  {"• Move flat · stalled"},
		// a completed action's deadline is not shown: it is not coming
		"/archive":   {"Archive · 1", "• Renew the passport"},
		"/scheduler": {"• Pay the rent · fires "},
		"/inbox":     {"Inbox · 1", "• Buy milk"},
		"/next #cra": {"Not read: #cra is no tag"},
		// each view narrows by its own subset: Tasks has no context filter
		"/tasks @home": {"Not read: @home is not a filter this view has"},
	} {
		o := b.answer(cmd)
		got := strings.Join(o.replies, "\n")
		for _, w := range want {
			if !strings.Contains(got, w) {
				t.Errorf("%s replied:\n%s\nwant it to hold %q", cmd, got, w)
			}
		}
	}
	if got := strings.Join(b.answer("/archive").replies, "\n"); strings.Contains(got, "due") {
		t.Errorf("the archive shows a completed action's deadline:\n%s", got)
	}
	if got := strings.Join(b.answer("/inbox").replies, "\n"); strings.Contains(got, "oat") {
		t.Errorf("the inbox list shows more than an item's first line:\n%s", got)
	}
}
