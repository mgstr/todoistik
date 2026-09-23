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

// A bookmark is a view and a filter line under a digit, and the rules worth
// pinning are the ones a person notices the moment they are wrong: what is
// kept is the filter set and not the keystrokes, the view it was made on is
// kept with it, and what that view does not filter by never makes it into the
// slot at all.

func keep(t *testing.T, s *Server, slot, view, line string) string {
	t.Helper()
	form := url.Values{"slot": {slot}, "view": {view}, "q": {line}}
	r := httptest.NewRequest(http.MethodPost, "/bookmark", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /bookmark %s=%q on %s: %d\n%s", slot, line, view, rec.Code, rec.Body.String())
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

	answer := keep(t, s, "3", "next", "milk #car @home #cra")
	if got, want := s.bookmarks()[2].Line, "@home #car milk"; got != want {
		t.Errorf("slot 3 holds %q, want %q — the unknown name dropped and the rest in order", got, want)
	}
	// and the key layer is told what was stored rather than what it sent, so
	// the dialog says the line the app would filter by, on the view it opens
	if !strings.Contains(answer, `"slot":3`) || !strings.Contains(answer, `"line":"@home #car milk"`) {
		t.Errorf("the answer to the key layer was %s", strings.TrimSpace(answer))
	}
	if !strings.Contains(answer, `"view":"next"`) || !strings.Contains(answer, `"name":"Next actions"`) {
		t.Errorf("the answer did not say the view: %s", strings.TrimSpace(answer))
	}
}

// A bookmark is a view and a line: going to one opens that view, so the view
// has to be half of what the slot holds (design.md, "Bookmarked filters").
func TestABookmarkKeepsTheViewItWasMadeOn(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	keep(t, s, "1", "archive", "#car")

	if got := s.bookmarks()[0].View; got != "archive" {
		t.Errorf("slot 1 opens %q, want the view it was kept on", got)
	}
	// and the dialog says it by name, which is what is read there
	row := s.bookmarks().rows()[0]
	if row.Name != "Archive" {
		t.Errorf("the dialog would say %q, want the view's own name", row.Name)
	}
}

// The line is narrowed to its own view on the way in, not on every use: a slot
// that opens Tasks cannot hold a context, because pressing it would open Tasks
// and Tasks does not filter by one.
func TestABookmarkHoldsOnlyWhatItsOwnViewFiltersBy(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddContext("home", ""); err != nil {
		t.Fatal(err)
	}

	keep(t, s, "4", "tasks", "@home #car")
	if got, want := s.bookmarks()[3].Line, "#car"; got != want {
		t.Errorf("a bookmark on Tasks holds %q, want %q — Tasks does not filter by context", got, want)
	}
	// the same line kept on the view that does offer it stays whole
	keep(t, s, "5", "next", "@home #car")
	if got, want := s.bookmarks()[4].Line, "@home #car"; got != want {
		t.Errorf("a bookmark on Next actions holds %q, want %q", got, want)
	}
}

// A view with no filter line is not somewhere a bookmark can point: the press
// would open it and have nothing to apply. Refused rather than stored
// half-made, the way a slot outside 1–9 is.
func TestABookmarkOnAViewThatDoesNotFilterIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	for _, view := range []string{"inbox", "today", "review", "", "nosuchview"} {
		form := url.Values{"slot": {"1"}, "view": {view}, "q": {"milk"}}
		r := httptest.NewRequest(http.MethodPost, "/bookmark", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, r)
		if rec.Code != http.StatusNotFound {
			t.Errorf("a bookmark on %q answered %d, want it refused", view, rec.Code)
		}
	}
}

// Nine slots, each one a place: clearing empties the slot rather than moving
// the ones after it, because the digit is the whole address of a bookmark.
func TestClearingASlotLeavesTheOthersWhereTheyAre(t *testing.T) {
	s, _ := newTestServer(t)
	keep(t, s, "1", "tasks", "milk")
	keep(t, s, "2", "tasks", "bread")
	keep(t, s, "1", "tasks", "")

	b := s.bookmarks()
	if b[0] != (slot{}) {
		t.Errorf("slot 1 holds %+v after being cleared", b[0])
	}
	if b[1].Line != "bread" || b[1].View != "tasks" {
		t.Errorf("slot 2 holds %+v, want it untouched", b[1])
	}
}

// A bookmark outlives the tab it was made in, which is the whole of what the
// word promises: it is in the database with the panels and the per-view filter
// sets, and not in the browser.
func TestABookmarkSurvivesTheApp(t *testing.T) {
	s, a := newTestServer(t)
	keep(t, s, "9", "someday", "milk")

	again, err := New(a, "", conf.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got := again.bookmarks()[8]; got.Line != "milk" || got.View != "someday" {
		t.Errorf("slot 9 came back as %+v after a restart, want the view and the line", got)
	}
}

// A slot kept before a bookmark knew its view holds a line and no view. It
// reads as exactly that rather than as corrupt: the stored value gains a key
// per slot, so an old one is a new one with the view left out.
func TestASlotKeptBeforeViewsReadsAsALineWithNoView(t *testing.T) {
	b := decodeBookmarks("1=%40home+%23car&7=milk")
	if b[0].Line != "@home #car" || b[0].View != "" {
		t.Errorf("the old slot 1 read as %+v, want the line and no view", b[0])
	}
	if b[6].Line != "milk" {
		t.Errorf("the old slot 7 read as %+v", b[6])
	}
}

func TestASlotThatIsNotOneOfTheNineIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	for _, slot := range []string{"0", "10", "-1", "", "three"} {
		form := url.Values{"slot": {slot}, "view": {"tasks"}, "q": {"milk"}}
		r := httptest.NewRequest(http.MethodPost, "/bookmark", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, r)
		if rec.Code != http.StatusNotFound {
			t.Errorf("slot %q answered %d, want it refused", slot, rec.Code)
		}
	}
}

// A line arriving on a view that filters by less than it says is narrowed on
// the way in rather than filtering something it does not mean: the box then
// shows exactly what the list is narrowed by (design.md, "The filter line").
// Nothing the keys send is in this shape any more — a bookmark is narrowed
// where it is kept — but a hand-written URL and a slot kept before views are.
func TestAViewTakesTheHalfOfALineItFiltersBy(t *testing.T) {
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
// to "where does the next bookmark go". Each full one carries the view it
// opens, which is what the key layer goes to and what the row reads as.
func TestTheDialogShowsAllNineSlots(t *testing.T) {
	s, _ := newTestServer(t)
	keep(t, s, "2", "tasks", "milk")

	body := getPage(t, s, "/next")
	for n := 1; n <= bookmarkSlots; n++ {
		if !strings.Contains(body, `data-slot="`+strconv.Itoa(n)+`"`) {
			t.Errorf("slot %d is not on the page", n)
		}
	}
	if !strings.Contains(body, `data-line="milk"`) {
		t.Error("the kept line is not on the page")
	}
	if !strings.Contains(body, `data-view="tasks"`) {
		t.Error("the view the bookmark opens is not on the page")
	}
	if !strings.Contains(body, "Tasks</span>milk") {
		t.Error("the row does not read as the view and then the line")
	}
	// and the page says which view a bookmark kept from it would open, so the
	// key layer never has to work that out for itself
	if !strings.Contains(body, `name="view" value="next"`) {
		t.Error("the save form does not carry the view the page is")
	}
}
