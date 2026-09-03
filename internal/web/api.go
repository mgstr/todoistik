package web

import (
	"encoding/json"
	"io"
	"net/http"
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
	f := parseFilters(r.URL.Query())
	name := r.PathValue("name")
	var (
		data any
		err  error
	)
	switch name {
	case "inbox":
		data, err = s.app.Inbox()
	case "someday":
		data, err = s.app.SomedayItems(f.Name)
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
	writeJSON(w, map[string]any{
		"view":    name,
		"filters": f,
		"items":   data,
	})
}

var _ = app.Filters{} // keep the import when the switch changes
