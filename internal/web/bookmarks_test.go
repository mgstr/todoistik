package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"todoistik/internal/conf"
)

// A bookmark is a filter line under a digit, and the two rules worth pinning
// are the ones a person notices the moment they are wrong: what is kept is the
// filter set and not the keystrokes, and a line kept on one view means on
// another view exactly the part of it that view filters by.

func keep(t *testing.T, s *Server, slot, line string) string {
	t.Helper()
	form := url.Values{"slot": {slot}, "q": {line}}
	r := httptest.NewRequest(http.MethodPost, "/bookmark", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /bookmark %s=%q: %d\n%s", slot, line, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// What is stored is the filter set written back out, in the one fixed order,
// with what the app cannot read left out of it — so a bookmark can never hold
// a name that means nothing, and the same filter always reads the same way.
func TestABookmarkKeepsTheFilterSetAndNotTheKeystrokes(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddContext("home", ""); err != nil {
		t.Fatal(err)
	}

	answer := keep(t, s, "3", "milk #car @home #cra")
	if got, want := s.bookmarks()[2], "@home #car milk"; got != want {
		t.Errorf("slot 3 holds %q, want %q — the unknown name dropped and the rest in order", got, want)
	}
	// and the key layer is told what was stored rather than what it sent, so
	// the dialog says the line the app would filter by
	if !strings.Contains(answer, `"slot":3`) || !strings.Contains(answer, `"line":"@home #car milk"`) {
		t.Errorf("the answer to the key layer was %s", strings.TrimSpace(answer))
	}
}

// Nine slots, each one a place: clearing empties the slot rather than moving
// the ones after it, because the digit is the whole address of a bookmark.
func TestClearingASlotLeavesTheOthersWhereTheyAre(t *testing.T) {
	s, _ := newTestServer(t)
	keep(t, s, "1", "milk")
	keep(t, s, "2", "bread")
	keep(t, s, "1", "")

	b := s.bookmarks()
	if b[0] != "" {
		t.Errorf("slot 1 holds %q after being cleared", b[0])
	}
	if b[1] != "bread" {
		t.Errorf("slot 2 holds %q, want it untouched at %q", b[1], "bread")
	}
}

// A bookmark outlives the tab it was made in, which is the whole of what the
// word promises: it is in the database with the panels and the per-view filter
// sets, and not in the browser.
func TestABookmarkSurvivesTheApp(t *testing.T) {
	s, a := newTestServer(t)
	keep(t, s, "9", "milk")

	again, err := New(a, "", conf.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got := again.bookmarks()[8]; got != "milk" {
		t.Errorf("slot 9 came back as %q after a restart, want %q", got, "milk")
	}
}

func TestASlotThatIsNotOneOfTheNineIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	for _, slot := range []string{"0", "10", "-1", "", "three"} {
		form := url.Values{"slot": {slot}, "q": {"milk"}}
		r := httptest.NewRequest(http.MethodPost, "/bookmark", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, r)
		if rec.Code != http.StatusNotFound {
			t.Errorf("slot %q answered %d, want it refused", slot, rec.Code)
		}
	}
}

// The line a bookmark holds is not about a view, so it is pressed on views
// that filter by less than it says. What the view does not offer is dropped on
// the way in rather than filtering something it does not mean: the box then
// shows exactly what the list is narrowed by (design.md, "Bookmarked
// filters").
func TestAViewTakesTheHalfOfABookmarkItFiltersBy(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddContext("home", ""); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodGet, "/tasks?f=1&q="+url.QueryEscape("@home #car"), nil)
	f := s.viewFilters("tasks", r)
	if len(f.Contexts) != 0 {
		t.Errorf("Tasks filtered by %v, and it does not filter by context", f.Contexts)
	}
	if len(f.Tags) != 1 || f.Tags[0] != "car" {
		t.Errorf("Tasks filtered by tags %v, want the one it does filter by", f.Tags)
	}
	if q := f.Query(); q != "#car" {
		t.Errorf("the box would show %q, want %q — a token that filters nothing must not be left in it", q, "#car")
	}

	// and on the view that does offer it, the same line is the whole of it
	r = httptest.NewRequest(http.MethodGet, "/next?f=1&q="+url.QueryEscape("@home #car"), nil)
	if q := s.viewFilters("next", r).Query(); q != "@home #car" {
		t.Errorf("Next read the line as %q, want it whole", q)
	}
}

// The dialog lists all nine whatever is in them: the empty ones are the answer
// to "where does the next bookmark go".
func TestTheDialogShowsAllNineSlots(t *testing.T) {
	s, _ := newTestServer(t)
	keep(t, s, "2", "milk")

	body := getPage(t, s, "/next")
	for n := 1; n <= bookmarkSlots; n++ {
		if !strings.Contains(body, `data-slot="`+strconv.Itoa(n)+`"`) {
			t.Errorf("slot %d is not on the page", n)
		}
	}
	if !strings.Contains(body, `data-line="milk"`) {
		t.Error("the kept line is not on the page")
	}
}
