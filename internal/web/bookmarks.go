package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"todoistik/internal/app"
)

// Nine bookmarks, kept under a digit each. A filter is typed often enough and
// re-typed often enough that the line worth having back is always one of half
// a dozen — "@home and short", "the car, finished this month" — and what is
// worth keeping is that line together with the view it was read on: a filter
// answers a question, and the view is half of the question (design.md,
// "Bookmarked filters"). So a bookmark is a view and a line, and going to one
// opens that view with that line applied.
//
// Nine, because the digits are what they are pressed with and there are nine
// of them; the tenth key is spent on showing the nine rather than on a tenth
// slot, since a set you cannot see is a set you cannot use.
//
// They are remembered where the panels and the per-view filter sets are —
// app_state, on the server — for the reason those are: a single-user app has
// one place for screen state, and a bookmark that lived in the browser would
// be gone with the tab it was made in, which is not what "bookmark" promises.
const bookmarksState = "bookmarks"

const bookmarkSlots = 9

// slot is what one digit holds. The line is what decides whether the slot is
// empty: it is the half a bookmark is pressed for, and a view with no line is
// not something the keys can put here (see bookmarkSave).
type slot struct {
	View string
	Line string
}

// bookmarks is slot 1 at index 0. An empty slot is still a slot: the dialog
// shows all nine whatever is in them, because the empty ones are what says
// where the next bookmark can go.
type bookmarks [bookmarkSlots]slot

// bookmark is one row of that, for the dialog. The number is carried rather
// than derived in the template, because the template's loop index is not the
// key that presses it and a reader should not have to add one in their head.
// Name is the view's full name — the dialog is read, so it says "Next
// actions" and not the slug the address is built out of.
type bookmark struct {
	N    int    `json:"slot"`
	View string `json:"view"`
	Name string `json:"name"`
	Line string `json:"line"`
}

func (b bookmarks) rows() []bookmark {
	out := make([]bookmark, 0, bookmarkSlots)
	for i, s := range b {
		out = append(out, bookmark{N: i + 1, View: s.View, Name: viewName(s.View), Line: s.Line})
	}
	return out
}

// viewName is the name the nav and the ? panel give a view, which is the one
// a bookmark says too — one name per view, wherever it is read.
func viewName(view string) string {
	if h, ok := viewHelp[view]; ok {
		return h.Name
	}
	return ""
}

// Stored as one value in the same notation the panels use — a query string —
// so that a slot nobody has filled costs nothing and a slot added later reads
// as empty rather than as corrupt. The digit holds the line and `v` plus the
// digit holds the view, side by side rather than one nested inside the other,
// so the stored value stays one flat query string that can be read at a
// glance. A slot written before a bookmark knew its view has no `v` and reads
// as a line with no view, which is exactly what it is.
func (b bookmarks) encode() string {
	q := url.Values{}
	for i, s := range b {
		if s.Line == "" {
			continue
		}
		n := strconv.Itoa(i + 1)
		q.Set(n, s.Line)
		if s.View != "" {
			q.Set("v"+n, s.View)
		}
	}
	return q.Encode()
}

func decodeBookmarks(s string) bookmarks {
	var b bookmarks
	q, err := url.ParseQuery(s)
	if err != nil {
		return b
	}
	for i := range b {
		n := strconv.Itoa(i + 1)
		b[i] = slot{View: q.Get("v" + n), Line: q.Get(n)}
	}
	return b
}

func (s *Server) bookmarks() bookmarks {
	v, err := s.app.GetState(bookmarksState)
	if err != nil {
		return bookmarks{}
	}
	return decodeBookmarks(v)
}

// bookmarkSave answers one press of ctrl-N with a filter on the screen, and
// the same key from inside the dialog: slot, the view being looked at, and
// the line to keep in it. An empty line clears the slot, which is what the
// dialog's delete key sends — one endpoint, because storing and clearing are
// the same write with a different value, and a second route would be a second
// place to get the numbering wrong.
//
// Overwriting a full slot is not asked about, for the reason nothing else in
// the app is (design.md, "The protocol is followed, not enforced"): the
// dialog says what is in each slot, and it is one key to put the old line
// back if the wrong digit was pressed while it is still on the screen.
func (s *Server) bookmarkSave(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.FormValue("slot"))
	if err != nil || n < 1 || n > bookmarkSlots {
		http.NotFound(w, r)
		return
	}
	// A view the app does not filter by cannot be half of a bookmark: going to
	// one opens its view, so a slot holding a view with no filter line would
	// be an address that leads nowhere. Refused rather than stored half-made,
	// for the reason a slot outside 1–9 is refused — and asked before the line
	// is read, since narrowing to such a view would empty any line at all and
	// a refusal must not arrive looking like a slot being cleared.
	view := r.FormValue("view")
	if strings.TrimSpace(r.FormValue("q")) != "" && !app.Filterable(view) {
		http.NotFound(w, r)
		return
	}
	line := s.readableLine(view, r.FormValue("q"))
	b := s.bookmarks()
	if line == "" {
		b[n-1] = slot{}
	} else {
		b[n-1] = slot{View: view, Line: line}
	}
	if err := s.app.SetState(bookmarksState, b.encode()); err != nil {
		httpError(w, err)
		return
	}
	// The caller is the key layer, mid-filter, and it must not be navigated
	// away from the line it is standing in — the same answer the token box
	// gets when it learns a name (see "The remembered lists"). It is given
	// back the slot as stored, since that is what the dialog now says.
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		kept := b[n-1]
		json.NewEncoder(w).Encode(bookmark{N: n, View: kept.View, Name: viewName(kept.View), Line: kept.Line})
		return
	}
	back(w, r)
}

// readableLine is the codec rule applied to a bookmark: what is kept is the
// filter set the line spells, narrowed to the view it was read on and written
// back out of it (design.md, "The filter line"). A token the app cannot read,
// or one that view does not filter by, is dropped here rather than stored to
// be dropped on every use — so a bookmark can never hold something that does
// nothing when it is pressed, and the same line always reads back in the same
// order, whatever order it was typed in.
func (s *Server) readableLine(view, line string) string {
	if strings.TrimSpace(line) == "" {
		return ""
	}
	f, _ := app.NarrowToView(view, s.parseFilters(url.Values{"q": {line}}))
	return f.Query()
}
