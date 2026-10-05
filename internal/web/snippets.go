package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"todoistik/internal/app"
)

// Nine snippets, kept under a digit each, and stamped onto the meta line by
// one press of that digit's chord (design.md, "Snippets"). The same names go
// onto item after item — "@home #short", "@calls #car" — and a line that is
// typed daily is worth keeping, which is the argument the bookmarks were
// built on one screen over. This is that argument applied to the other line
// the app has: a bookmark keeps a filter and asks a question with it, a
// snippet keeps a run of notation and writes it down.
//
// Nine and a tenth key, for the reason there are nine bookmarks: the digits
// are what they are pressed with, and the tenth is spent on showing the nine
// rather than on a tenth slot.
//
// They are remembered in app_state beside the bookmarks and the panels,
// because a snippet that went with the tab it was made in would not be worth
// making — design.md, "Bookmarked filters" makes the same point about the
// other nine.
const snippetsState = "snippets"

const snippetSlots = 9

// snippets is slot 1 at index 0, and a slot is one line of notation. Unlike a
// bookmark there is no second half: a snippet has no view to belong to,
// because it is written into whatever screen you are standing on rather than
// opening one.
type snippets [snippetSlots]string

// snippet is one row of that, for the dialog — the digit carried rather than
// derived in the template, for the reason bookmark carries it.
type snippet struct {
	N    int    `json:"slot"`
	Line string `json:"line"`
}

func (s snippets) rows() []snippet {
	out := make([]snippet, 0, snippetSlots)
	for i, line := range s {
		out = append(out, snippet{N: i + 1, Line: line})
	}
	return out
}

// Stored as one query string, the way the bookmarks and the panels are: a
// slot nobody has filled costs nothing, and a slot added later reads as empty
// rather than as corrupt.
func (s snippets) encode() string {
	q := url.Values{}
	for i, line := range s {
		if line == "" {
			continue
		}
		q.Set(strconv.Itoa(i+1), line)
	}
	return q.Encode()
}

func decodeSnippets(s string) snippets {
	var out snippets
	q, err := url.ParseQuery(s)
	if err != nil {
		return out
	}
	for i := range out {
		out[i] = q.Get(strconv.Itoa(i + 1))
	}
	return out
}

func (s *Server) snippets() snippets {
	v, err := s.app.GetState(snippetsState)
	if err != nil {
		return snippets{}
	}
	return decodeSnippets(v)
}

// snippetSave answers one row of the `0` dialog being written: the slot and
// the line to keep in it. An empty line clears the slot, which is what the
// dialog's delete key sends — one endpoint, for the reason the bookmarks have
// one.
//
// A line the app cannot read is refused rather than stored: a snippet is
// pressed on a screen that is being typed into, so a slot holding a name
// nothing knows would put an underlined mistake into a form and leave you to
// find out why. That is the other half of design.md's rule that names are
// never typed fresh — the box asks about an unknown name as it is typed, and
// this makes sure the answer cannot be got round by keeping one.
func (s *Server) snippetSave(w http.ResponseWriter, r *http.Request) {
	n, err := strconv.Atoi(r.FormValue("slot"))
	if err != nil || n < 1 || n > snippetSlots {
		http.NotFound(w, r)
		return
	}
	line, lerr := s.readableSnippet(r.FormValue("q"))
	if lerr != nil {
		http.Error(w, lerr.Error(), http.StatusBadRequest)
		return
	}
	all := s.snippets()
	all[n-1] = line
	if err := s.app.SetState(snippetsState, all.encode()); err != nil {
		httpError(w, err)
		return
	}
	// The caller is the key layer, standing in a form full of unsaved words,
	// and it must not be navigated away from them — the same answer the
	// bookmark write gives, and for the same reason.
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snippet{N: n, Line: line})
		return
	}
	back(w, r)
}

// readableSnippet is the codec rule applied to a snippet: what is kept is the
// notation the line spells, written back out of the fields it parsed to. So a
// slot always reads in one fixed order whatever order it was typed in, and a
// name the app does not know never reaches a slot at all (see ParseSnippet for
// what the notation here takes, and why the dates are not part of it).
func (s *Server) readableSnippet(line string) (string, error) {
	if strings.TrimSpace(line) == "" {
		return "", nil
	}
	v, err := s.app.Vocabulary()
	if err != nil {
		return "", err
	}
	f, err := app.ParseSnippet(line, v)
	if err != nil {
		return "", err
	}
	return f.String(), nil
}
