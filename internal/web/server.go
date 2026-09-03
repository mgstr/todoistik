// Package web serves the UI, the capture API and the read API.
package web

import (
	"crypto/subtle"
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"todoistik/internal/app"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	app   *app.App
	token string
	tmpl  *template.Template
	mux   *http.ServeMux
}

func New(a *app.App, token string) (*Server, error) {
	s := &Server{app: a, token: token, mux: http.NewServeMux()}
	funcs := template.FuncMap{
		"age": func(v any) string {
			switch t := v.(type) {
			case time.Time:
				return humanAge(t)
			case *time.Time:
				if t != nil {
					return humanAge(*t)
				}
			}
			return ""
		},
		"dateAge": humanDateAge,
		"joinTags": func(v any) string {
			tags, ok := v.([]string)
			if !ok {
				return ""
			}
			out := make([]string, 0, len(tags))
			for _, t := range tags {
				if t != app.TodayTag {
					out = append(out, "#"+t)
				}
			}
			return strings.Join(out, " ")
		},
		"has": func(list any, item any) bool {
			want := ""
			switch v := item.(type) {
			case string:
				want = v
			case app.Duration:
				want = string(v)
			}
			switch l := list.(type) {
			case []string:
				for _, s := range l {
					if s == want {
						return true
					}
				}
			case []app.Duration:
				for _, d := range l {
					if string(d) == want {
						return true
					}
				}
			}
			return false
		},
		"hasTag": func(tags []string, tag string) bool {
			for _, t := range tags {
				if t == tag {
					return true
				}
			}
			return false
		},
		"qesc": url.QueryEscape,
		"dict": func(pairs ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(pairs); i += 2 {
				if k, ok := pairs[i].(string); ok {
					m[k] = pairs[i+1]
				}
			}
			return m
		},
	}
	t, err := template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s.tmpl = t
	s.routes()
	return s, nil
}

func humanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return "today"
	case d < 24*time.Hour:
		return "today"
	case d < 48*time.Hour:
		return "1d"
	case d < 14*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + "d"
	case d < 60*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24/7)) + "w"
	default:
		return strconv.Itoa(int(d.Hours()/24/30)) + "mo"
	}
}

func humanDateAge(day string) string {
	t, err := time.Parse(app.DateFormat, day)
	if err != nil {
		return ""
	}
	return humanAge(t)
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// static files and the login flow are reachable unauthenticated
		if strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/login" {
			s.mux.ServeHTTP(w, r)
			return
		}
		if !s.authed(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				jsonError(w, http.StatusUnauthorized, "missing or wrong bearer token")
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		// any authenticated use of the app counts as use of the day: the
		// lazy day boundary (clear #today, fire schedules) runs here.
		if err := s.app.DayStart(); err != nil {
			log.Printf("day start: %v", err)
		}
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Server) authed(r *http.Request) bool {
	if s.token == "" {
		return true // no token configured: bind to localhost and trust it
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), []byte(s.token)) == 1
	}
	if c, err := r.Cookie("token"); err == nil {
		return subtle.ConstantTimeCompare([]byte(c.Value), []byte(s.token)) == 1
	}
	return false
}

func (s *Server) routes() {
	m := s.mux
	m.Handle("GET /static/", http.FileServerFS(staticFS))

	// login (the one unauthenticated page)
	m.HandleFunc("GET /login", s.loginPage)
	m.HandleFunc("POST /login", s.loginSubmit)

	// APIs
	m.HandleFunc("POST /api/capture", s.apiCapture)
	m.HandleFunc("GET /api/view/{name}", s.apiView)

	// views
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/next", http.StatusSeeOther)
	})
	m.HandleFunc("GET /inbox", s.inboxPage)
	m.HandleFunc("POST /capture", s.capturePost)
	m.HandleFunc("GET /someday", s.somedayPage)
	m.HandleFunc("GET /projects", s.projectsPage)
	m.HandleFunc("GET /tasks", s.tasksPage)
	m.HandleFunc("GET /next", s.nextPage)
	m.HandleFunc("GET /today", s.todayPage)
	m.HandleFunc("GET /waiting", s.waitingPage)
	m.HandleFunc("GET /calendar", s.calendarPage)
	m.HandleFunc("GET /archive", s.archivePage)
	m.HandleFunc("GET /scheduler", s.schedulerPage)
	m.HandleFunc("GET /audit", s.auditPage)

	// Inbox Zero / processing
	m.HandleFunc("GET /process", s.processPage)
	m.HandleFunc("POST /process/{src}/{id}/{branch}", s.processBranch)

	// actions
	m.HandleFunc("GET /action/{id}", s.actionPage)
	m.HandleFunc("POST /action/{id}", s.actionUpdate)
	m.HandleFunc("POST /action/{id}/{verb}", s.actionVerb)

	// projects
	m.HandleFunc("GET /project/{id}", s.projectPage)
	m.HandleFunc("POST /project/{id}", s.projectUpdate)
	m.HandleFunc("POST /project/{id}/{verb}", s.projectVerb)

	// schedules
	m.HandleFunc("GET /schedule/new", s.scheduleNewPage)
	m.HandleFunc("POST /schedule", s.scheduleCreate)
	m.HandleFunc("GET /schedule/{id}", s.schedulePage)
	m.HandleFunc("POST /schedule/{id}", s.scheduleUpdate)
	m.HandleFunc("POST /schedule/{id}/delete", s.scheduleDelete)

	// someday item editing
	m.HandleFunc("GET /somedayitem/{id}", s.somedayItemPage)
	m.HandleFunc("POST /somedayitem/{id}", s.somedayItemUpdate)
	m.HandleFunc("POST /somedayitem/{id}/{verb}", s.somedayItemVerb)

	// weekly review
	m.HandleFunc("GET /review", s.reviewPage)
	m.HandleFunc("GET /review/{step}", s.reviewStepPage)
	m.HandleFunc("POST /review/{type}/{id}/done", s.reviewDone)

	// settings: tag / context list management
	m.HandleFunc("GET /settings", s.settingsPage)
	m.HandleFunc("POST /settings/{kind}/remove", s.settingsRemove)
}

// --- filter parsing ------------------------------------------------------

// parseFilters reads the shared filter set from query parameters; used
// identically by the UI views and the read API.
func parseFilters(q url.Values) app.Filters {
	f := app.Filters{
		Name:      strings.TrimSpace(q.Get("name")),
		Focus:     q.Get("focus"),
		Due:       q.Get("due"),
		Completed: q.Get("completed"),
		Sort:      q.Get("sort"),
		Desc:      q.Get("desc") == "1",
	}
	for _, t := range q["tag"] {
		if t != "" {
			f.Tags = append(f.Tags, t)
		}
	}
	for _, c := range q["context"] {
		if c != "" {
			f.Contexts = append(f.Contexts, c)
		}
	}
	for _, d := range q["duration"] {
		if dd := app.Duration(d); dd.Valid() && dd != app.DurNone {
			f.Durations = append(f.Durations, dd)
		}
	}
	if f.Focus != "exclude" && f.Focus != "only" {
		f.Focus = ""
	}
	return f
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func httpError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func idParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id
}

// back redirects to the page the request came from, defaulting home.
func back(w http.ResponseWriter, r *http.Request) {
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/next"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}
