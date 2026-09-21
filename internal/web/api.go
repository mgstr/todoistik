package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"todoistik/internal/app"
)

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

// apiCapture is the single way in from outside: a text payload becomes an
// inbox item. The response distinguishes accepted from duplicate, so a
// script cannot mistake "silently vanished" for "accepted".
func (s *Server) apiCapture(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "unreadable body")
		return
	}
	text := string(body)
	// accept both raw text and {"text": "..."}
	if strings.HasPrefix(strings.TrimSpace(text), "{") {
		var payload struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.Text != "" {
			text = payload.Text
		}
	}
	item, accepted, err := s.app.Capture(text)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !accepted {
		writeJSON(w, map[string]any{"status": "duplicate"})
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]any{"status": "accepted", "item": item})
}

// apiView is the read API: one endpoint per view, the caller's own filters
// as query parameters, independent of the screen's filter state. Read only.
func (s *Server) apiView(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	f, problems := s.apiFilters(name, r.URL.Query())
	var (
		data any
		err  error
	)
	switch name {
	case "inbox":
		data, err = s.app.Inbox()
	case "someday":
		data, err = s.app.SomedayItems(f)
	case "projects":
		data, err = s.app.ProjectList(f)
	case "tasks":
		data, err = s.app.Tasks(f)
	case "next":
		data, err = s.app.NextActions(f)
	case "today":
		data, err = s.app.TodayItems()
	case "waiting":
		data, err = s.app.WaitingFor(f)
	case "calendar":
		data, err = s.app.Calendar(f)
	case "archive":
		data, err = s.app.Archive(f)
	case "scheduler":
		data, err = s.app.Schedules(f.Name)
	case "review":
		data, err = s.app.ReviewCounts()
	default:
		jsonError(w, http.StatusNotFound, "no such view; one of: inbox someday projects tasks next today waiting calendar archive scheduler review")
		return
	}
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	answer := map[string]any{
		"view":    name,
		"filters": f,
		"items":   data,
	}
	if len(problems) > 0 {
		answer["problems"] = problems
	}
	writeJSON(w, answer)
}

// apiProblem is one part of a filter line a read left out: the token as it was
// written, and why, in app.QueryProblem's kinds.
type apiProblem struct {
	Token string `json:"token"`
	Kind  string `json:"kind"`
}

// apiFilters reads the caller's filters and reports what it could not use.
//
// A read is answered with what the app could make of the request, the way the
// screen narrows by the rest of a line it could only partly read — but the
// screen marks what it left out, and a caller has no mark to see unless it is
// told. Two things get left out: a name the app does not know (`#cra` for
// `#car`), and a filter this view does not offer (`@home` on "Tasks", which
// design.md gives no context filter). Unsaid, either one is a read of a wider
// view that looks exactly like a read of the narrow one.
func (s *Server) apiFilters(view string, q url.Values) (app.Filters, []apiProblem) {
	var problems []app.QueryProblem
	if line := q.Get("q"); strings.TrimSpace(line) != "" {
		v, err := s.app.Vocabulary()
		if err != nil {
			v = &app.Vocabulary{}
		}
		_, problems = app.ParseQuery(line, v)
	}
	// the filters themselves come from the shared reader, so the API and the
	// screen never disagree about what a parameter means
	f, dropped := app.NarrowToView(view, s.parseFilters(q))
	problems = append(problems, dropped...)

	var out []apiProblem
	for _, p := range problems {
		out = append(out, apiProblem{Token: p.Token, Kind: p.Kind})
	}
	return f, out
}

var _ = app.Filters{} // keep the import when the switch changes
