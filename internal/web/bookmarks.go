package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Nine filter lines, kept under a digit each. A filter is typed often enough
// and re-typed often enough that the line worth having back is always one of
// half a dozen — "@home and short", "the car, finished this month" — and the
// line is the only part of it worth keeping: it is already the whole filter
// set written down (design.md, "The filter line"), so a bookmark is that
// string and nothing else.
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

// bookmarks is slot 1 at index 0. A slot holding "" is empty and is still a
// slot: the dialog shows all nine whatever is in them, because the empty ones
// are what says where the next bookmark can go.
type bookmarks [bookmarkSlots]string

// bookmark is one row of that, for the dialog. The number is carried rather
// than derived in the template, because the template's loop index is not the
// key that presses it and a reader should not have to add one in their head.
type bookmark struct {
	N    int    `json:"slot"`
	Line string `json:"line"`
}

func (b bookmarks) rows() []bookmark {
	out := make([]bookmark, 0, bookmarkSlots)
	for i, line := range b {
		out = append(out, bookmark{N: i + 1, Line: line})
	}
	return out
}

// Stored as one value in the same notation the panels use — a query string —
// so that a slot nobody has filled costs nothing and a slot added later reads
// as empty rather than as corrupt.
func (b bookmarks) encode() string {
	q := url.Values{}
	for i, line := range b {
		if line != "" {
			q.Set(strconv.Itoa(i+1), line)
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
		b[i] = q.Get(strconv.Itoa(i + 1))
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
// the same key from inside the dialog: slot, and the line to keep in it. An
// empty line clears the slot, which is what the dialog's delete key sends —
// one endpoint, because storing and clearing are the same write with a
// different value, and a second route would be a second place to get the
// numbering wrong.
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
	b := s.bookmarks()
	b[n-1] = s.readableLine(r.FormValue("q"))
	if err := s.app.SetState(bookmarksState, b.encode()); err != nil {
		httpError(w, err)
		return
	}
	// The caller is the key layer, mid-filter, and it must not be navigated
	// away from the line it is standing in — the same answer the token box
	// gets when it learns a name (see "The remembered lists"). It is given
	// back the line as stored, since that is what the dialog now says.
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bookmark{N: n, Line: b[n-1]})
		return
	}
	back(w, r)
}

// readableLine is the codec rule applied to a bookmark: what is kept is the
// filter set the line spells, written back out of it (design.md, "The filter
// line"). A token the app cannot read is dropped here rather than stored to
// be dropped on every use, so a bookmark can never hold a name that means
// nothing — and the same line always reads back in the same order, whatever
// order it was typed in.
func (s *Server) readableLine(line string) string {
	if strings.TrimSpace(line) == "" {
		return ""
	}
	return s.parseFilters(url.Values{"q": {line}}).Query()
}
