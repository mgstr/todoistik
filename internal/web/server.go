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
	"todoistik/internal/conf"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

type Server struct {
	app   *app.App
	token string
	conf  conf.Config
	tmpl  *template.Template
	mux   *http.ServeMux
}

func New(a *app.App, token string, c conf.Config) (*Server, error) {
	s := &Server{app: a, token: token, conf: c, mux: http.NewServeMux()}
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
		// item text with its links live, and the links on their own for the
		// screens that cannot make the text itself live — see links.go
		"linkify":   linkify,
		"links":     links,
		"linkLabel": linkLabel,
		"qesc":      url.QueryEscape,
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
	if err := checkZenViews(c); err != nil {
		return nil, err
	}
	s.tmpl = t
	s.routes()
	return s, nil
}

// humanAge writes an age the way it would be said out loud, rather than as a
// code to decode: "3 weeks ago", not "3w". Deliberately rounded — a month is
// 30 days and a year 365, because calendar-accurate arithmetic would make "2
// months ago" cover different spans in different seasons for no gain on a
// label that is approximate by design.
func humanAge(t time.Time) string {
	days := int(time.Since(t).Hours() / 24)
	switch {
	case days <= 0: // includes a future date, which no caller should pass
		return "today"
	case days == 1:
		return "yesterday"
	case days < 7:
		return strconv.Itoa(days) + " days ago"
	case days < 14:
		return "a week ago"
	case days < 28:
		return strconv.Itoa(days/7) + " weeks ago"
	case days < 60: // runs to just under two months, so nothing falls between
		return "a month ago"
	case days < 365:
		// 360 days is 12 thirty-day months but not yet a year, so the count
		// stops at 11 rather than saying "12 months ago" for four days
		months := days / 30
		if months > 11 {
			months = 11
		}
		return strconv.Itoa(months) + " months ago"
	case days < 730:
		return "a year ago"
	default:
		return strconv.Itoa(days/365) + " years ago"
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
	m.HandleFunc("POST /process/{id}/{branch}", s.processBranch)

	// actions
	m.HandleFunc("GET /action/{id}", s.actionPage)
	m.HandleFunc("GET /action/{id}/promote", s.promotePage)
	m.HandleFunc("POST /action/{id}", s.actionUpdate)
	m.HandleFunc("POST /action/{id}/{verb}", s.actionVerb)

	// projects
	m.HandleFunc("GET /project/{id}", s.projectPage)
	m.HandleFunc("GET /project/{id}/addaction", s.projectAddAction)
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
	m.HandleFunc("POST /somedayitem/{id}/inbox", s.somedayItemToInbox)

	// weekly review
	m.HandleFunc("GET /review", s.reviewPage)
	m.HandleFunc("GET /review/{step}", s.reviewStepPage)
	m.HandleFunc("POST /review/{type}/{id}/done", s.reviewDone)

	// doing: one action, alone on the screen
	m.HandleFunc("GET /doing/{id}", s.doingPage)

	// the one display flag, toggled from anywhere by ctrl-t
	m.HandleFunc("POST /ages", s.agesToggle)

	// the panels around a view, toggled from the ctrl-v dialog
	m.HandleFunc("POST /panels/{which}", s.panelsToggle)

	// settings: tag / context list management
	m.HandleFunc("GET /settings", s.settingsPage)
	m.HandleFunc("POST /settings/{kind}/add", s.settingsAdd)
	m.HandleFunc("POST /settings/{kind}/remove", s.settingsRemove)
}

// --- filter parsing ------------------------------------------------------

// parseFilters reads the shared filter set from query parameters; used
// identically by the UI views and the read API.
//
// `q` is the filter line (design.md, "Filtering"), and it answers for every
// filter it can say: given one, the discrete parameters are not also read, or
// a caller would have two ways to ask one question and a rule about which
// wins. Sort and order are not in the line and are read either way. What the
// line cannot name — a name that is on no remembered list — is dropped here;
// the screen asks about those before it ever submits (see "Token boxes").
func (s *Server) parseFilters(q url.Values) app.Filters {
	if line := q.Get("q"); strings.TrimSpace(line) != "" {
		v, err := s.app.Vocabulary()
		if err != nil {
			v = &app.Vocabulary{}
		}
		f, _ := app.ParseQuery(line, v)
		f.Sort, f.Desc = q.Get("sort"), q.Get("desc") == "1"
		return f
	}
	return parseFilters(q)
}

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

// render fills in the one thing no handler sets: which panels this screen is
// wearing. It happens here rather than in newPage because a handler adds its
// steps to the trail after that, and the trail is what says whether this is a
// screen the settings file opens in zen mode (see "Panels").
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if p, ok := data.(*page); ok {
		st := s.panelState()
		if next, changed := st.forScreen(zenScreen(p.Trail, s.conf)); changed {
			st = next
			if err := s.savePanels(st); err != nil {
				log.Printf("panels: %v", err)
			}
		}
		p.Panels = st.shown()
	}
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

// back redirects to the page the request came from, defaulting home. A form
// may say where that is with a "back" field, and then it wins: a screen you
// leave by acting on it — doing, where completing the action is the way out —
// knows where it came from, and the header only knows where the press was.
func back(w http.ResponseWriter, r *http.Request) {
	if to := localPath(r.FormValue("back"), ""); to != "" {
		http.Redirect(w, r, to, http.StatusSeeOther)
		return
	}
	ref := r.Header.Get("Referer")
	if ref == "" {
		ref = "/next"
	}
	http.Redirect(w, r, ref, http.StatusSeeOther)
}

// localPath keeps a destination that came in on a request to this app: one
// leading slash and nothing that could turn into another host. Anything else
// falls back, so a hand-edited URL cannot make a link off the site.
func localPath(v, fallback string) string {
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") && !strings.Contains(v, ":") {
		return v
	}
	return fallback
}

// viewOf names the view a local path belongs to, for the screens that sit
// under one without being it. The first segment is the view's own slug
// wherever it is a view at all; the few singular paths that hang under a
// plural view are the only table, and it is short because a detail page is
// named after the item and a view after the pile of them.
func viewOf(path string) string {
	seg := strings.TrimPrefix(path, "/")
	if i := strings.IndexAny(seg, "/?"); i >= 0 {
		seg = seg[:i]
	}
	switch seg {
	case "project":
		seg = "projects"
	case "schedule":
		seg = "scheduler"
	case "somedayitem":
		seg = "someday"
	}
	if _, ok := viewHelp[seg]; ok {
		return seg
	}
	return ""
}

// parentView is where a screen about one item goes back to. It is the view it
// was opened from: said on the URL if the screen was asked to carry it (doing
// does, because it has to survive a reload), and read off the Referer
// otherwise, which is what a list link and a boosted navigation both leave
// behind. A page reached with neither — a reload, a bookmark — falls back to
// where the item lives.
func (s *Server) parentView(r *http.Request, fallback string) string {
	if from := localPath(r.URL.Query().Get("from"), ""); from != "" {
		return from
	}
	if ref := r.Header.Get("Referer"); ref != "" {
		if u, err := url.Parse(ref); err == nil && u.Path != r.URL.Path {
			// the path only: the view remembers its own filters, and carrying
			// a query back would be a second copy of them
			if p := localPath(u.Path, ""); p != "" && viewOf(p) != "" {
				return p
			}
		}
	}
	return fallback
}
