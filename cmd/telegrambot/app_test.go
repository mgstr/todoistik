package main

import (
	"net/http/httptest"
	"os"
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

// A token reaches this program through a file so that it is not a flag value,
// where any `ps` would read it (implementation.md, "Where a secret lives").
// What is worth pinning is the refusals: a named file that is not there, or
// that holds nothing, has to say so rather than hand back an empty token and
// let the failure surface a layer later as a 401 from the app.
func TestATokenFileIsReadOrRefused(t *testing.T) {
	write := func(body string) string {
		p := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// the trailing newline every editor leaves is not part of the token
	if tok, err := fileToken(write("s3cret\n")); err != nil || tok != "s3cret" {
		t.Errorf("fileToken read %q, %v; want \"s3cret\", nil", tok, err)
	}
	if _, err := fileToken(write("   \n\t\n")); err == nil {
		t.Error("a file holding only whitespace was accepted as a token")
	}
	if _, err := fileToken(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("a file that is not there was accepted as a token")
	}

	// the bot's own token falls back to the environment when no file is named,
	// and only then — a named file never reaches the environment behind it
	t.Setenv("TELEGRAM_TOKEN", "from-the-environment")
	if tok, err := readToken(""); err != nil || tok != "from-the-environment" {
		t.Errorf("readToken(\"\") read %q, %v; want the environment's", tok, err)
	}
	if tok, err := readToken(write("from-the-file")); err != nil || tok != "from-the-file" {
		t.Errorf("readToken(file) read %q, %v; want the file's", tok, err)
	}
	if _, err := readToken(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("a named file that is not there fell back to the environment")
	}
}
