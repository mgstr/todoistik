package web

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"todoistik/internal/app"
)

// undoAsk answers what `u` would take back: the last step, as the lines the
// audit log wrote about it. Asked for at the press rather than drawn into
// every page, because a page is a snapshot of the moment it was rendered and
// some writes do not redraw it — a review mark is flipped where it stands — so
// a question drawn in advance could read out one step and take back another
// (implementation.md, "Undo").
//
// Nothing to undo is an answer with nothing in it, and the key layer does
// nothing with that.
func (s *Server) undoAsk(w http.ResponseWriter, r *http.Request) {
	step, err := s.app.LastUndo()
	if err != nil {
		httpError(w, err)
		return
	}
	if step == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "undostep", step); err != nil {
		log.Printf("render undostep: %v", err)
	}
}

// undoDo takes back the step the question read out, and comes back to the
// screen it was asked on — which now shows the item where it was.
//
// A step that is no longer the last one is not an error to show: the screen
// is drawn again, and the next press asks about whatever is last now.
func (s *Server) undoDo(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.FormValue("step"), 10, 64)
	_, err := s.app.Undo(id)
	if err != nil && !errors.Is(err, app.ErrUndoMoved) && !errors.Is(err, app.ErrNothingToUndo) {
		httpError(w, err)
		return
	}
	to := localPath(r.FormValue("back"), "/next")
	// The screen may have been the page of the very item the step made, and
	// that item is gone again. The pile it would have been in is the honest
	// place to land, as it is when a project is deleted from its own page.
	if s.itemGone(to) {
		to = "/" + viewOf(to)
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// itemGone reports whether a local path is the page of an item that does not
// exist. Only the paths that name one item can be; a view is never gone.
func (s *Server) itemGone(path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		return false
	}
	seg := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(seg) < 2 {
		return false
	}
	id := parseID(seg[1])
	switch seg[0] {
	case "action", "doing":
		_, err = s.app.Action(id)
	case "project":
		_, err = s.app.Project(id)
	case "schedule":
		_, err = s.app.Schedule(id)
	case "somedayitem":
		_, err = s.app.SomedayItem(id)
	case "referenceitem":
		_, err = s.app.ReferenceItem(id)
	default:
		return false
	}
	return err != nil
}
