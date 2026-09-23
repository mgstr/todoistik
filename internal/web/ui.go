package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"todoistik/internal/app"
	"todoistik/internal/conf"
)

// viewHelp is what the ? panel shows: the view's full name and one line on
// what it is for. Keyed by the nav slug, so a detail page shows the help of
// the view it sits under. A page with no entry gets no panel, and the key bar
// drops ? accordingly — see implementation.md, "Screen layout".
var viewHelp = map[string]struct{ Name, Text string }{
	"inbox":     {"Inbox", "raw captures, oldest first — the one view that has to be emptied"},
	"today":     {"Today", "what has run out of time, and what you picked this morning"},
	"next":      {"Next actions", `everything that is yours to act on — "what can I do now?" is this view, narrowed`},
	"projects":  {"Projects", "active outcomes; a project with no next action is marked stalled"},
	"tasks":     {"Tasks", "standalone actions — every one already a next action"},
	"waiting":   {"Waiting for", "the ball is not in your court; age is the delegation date"},
	"calendar":  {"Calendar", "real deadlines, soonest first; overdue is shown whatever the filter says"},
	"someday":   {"Someday/Maybe", "raw ideas, worth revisiting some time — not now"},
	"scheduler": {"Scheduler", "what is going to arrive — nothing here is a commitment yet"},
	"review":    {"Weekly review", "resumable — progress lives on each item's lastReviewedAt"},
	"archive":   {"Archive", "finished commitments, newest first"},
	"audit":     {"Audit log", "every event; trashed things are recovered from here by recapturing"},
	"settings":  {"Settings", "the palette the app is painted in, and the remembered tags and contexts — a name has to be here before #car or @home means anything, which is what stops #car and #Car becoming two. The count is what carries it, and a name still carried cannot be removed. A parameter, @person(Marju), is its own name under its context. Filtering by #bike or @garage that matches nothing is how a name is created."},

	// Reached only from the Inbox, so it has no nav entry — but it is a screen
	// with a name, which the title bar's trail says out loud and zen.views can
	// name (see "Panels"). The process page asks for this one explicitly.
	"processing": {"Processing", "one item, one question — what is it? Every answer files it and takes it off the list it came from. What you have written before is shown beside it, and a number copies a finished one. Esc leaves it exactly as it was."},

	// Doing is a view now, reached from a row rather than from the nav, and it
	// is the other screen zen.views names by default.
	"doing": {"Doing", "one action, alone, and nothing else on the screen — c completes it, esc goes back"},

	// An action's own page sits under no view, so it would have had no panel
	// at all — but it holds the box an action is written in, and that is what
	// the panel now explains.
	"action": {"An action", "one step, and the box below its title is where everything it carries is written."},
}

// notation asks the ? panel to carry the description notation as well as the
// view's own line. Set on every screen where an action is written, because
// that is where the question is asked — and answered in the panel rather than
// beside the box, so that the one place extra explanation lives is the same
// place on every screen (implementation.md, "View help").
func (p *page) notation(s *Server) *page {
	p.Notation = true
	return p
}

// help overrides the entry newPage picked from the view slug, for a screen
// that sits under a view in the nav but is not that view.
func (p *page) help(key string) *page {
	if h, ok := viewHelp[key]; ok {
		p.HelpName, p.HelpText = h.Name, h.Text
	}
	return p
}

// crumb is one step of the title bar's trail: the view, then whatever is
// being done inside it. Slug is the screen's own name where the step is a
// screen — that is what zen.views names (see "Panels") — and empty where the
// step is an item's title or a stage that is not a place of its own.
type crumb struct {
	Slug  string
	Name  string
	Count int
	// Alert: the count is the one design.md says loudly. Only the inbox has
	// one, and the title bar says it the way the nav says it — with the nav
	// off, this is the only place left that can (see implementation.md,
	// "Panels").
	Alert bool
}

// step adds one level to the title bar's trail: where you now are inside the
// view you are in. A step that is a screen of its own gives its slug, so that
// zen.views can name it; a step that is an item's own title gives none.
func (p *page) step(name, slug string) *page {
	p.Trail = append(p.Trail, crumb{Slug: slug, Name: name})
	// A step inside a view is a form or a screen of a process, never a plain
	// list — that is what having a second crumb means. It is also the whole
	// set of screens a background refresh must keep its hands off, so the
	// answer is taken from the trail rather than from a second list of names
	// that could fall out of step with it.
	p.Live = false
	return p
}

// page is the data every template gets.
type page struct {
	Title       string
	View        string // active nav entry
	Trail       []crumb
	Panels      panels
	Timer       bool   // this screen counts its own minutes, so ^t is the timer here
	HelpName    string // the view's full name, for the ? panel
	HelpText    string // what this view is for, for the ? panel
	Processing  bool   // the nav slot named by View reads "Processing…" instead
	Notation    bool   // the ? panel also explains how an action is written
	When        bool   // the ? panel also explains what a schedule's When takes
	Vocab       struct{ Contexts, Tags, Verbs []string }
	Filters     app.Filters
	FilterQuery string // current filter query string (for sort/order links)
	Hidden      int    // how many items the filters hide
	Query       string // the filter set as a line, for the filter box
	FilterMode  string // which notation that line may use
	Shown       int    // items on the screen
	Total       int    // items the view holds with no filters at all
	Durations   []app.Duration
	Nav         *app.NavCounts
	// Bookmarks is the nine filter lines, for the ctrl-0 dialog. On every
	// page because the dialog is in the layout, like the panel chooser.
	Bookmarks []bookmark
	Today     string
	Ages      bool // the ages on rows are shown rather than hidden
	// Theme is which palette this page is painted in: one of conf.Themes,
	// resolved from what the Settings screen was last told and falling back to
	// the settings file. On every page because the answer is worn by the whole
	// document, and read on Settings because that is where it is chosen.
	Theme string
	Conf  conf.Config
	Error string
	// SelfURL is this page's own address, filters and all, so the background
	// refresh can ask for exactly the page it is standing on rather than for a
	// fragment endpoint that would have to be kept in step with it (see
	// implementation.md, "Keeping an open page current").
	SelfURL string
	// Live says this screen is a plain list, and so may be replaced wholesale
	// by that refresh. Every screen that is a step inside a view clears it in
	// step(): a form holds half-written words, and a refresh that threw them
	// away would be a worse bug than the stale list it fixed.
	Live bool
	Data any
}

// liveViews are the screens the background refresh may replace: the plain
// lists, where every row on screen is server state and nothing is half-typed.
// Review, Settings and every detail screen are left out — Review is a stepper
// and Settings is a form, and neither gains a row because something arrived.
// A screen that is a step inside one of these clears the flag anyway (see
// step()), which is what keeps Processing off the list while it borrows the
// Inbox's slot.
var liveViews = map[string]bool{
	"inbox":     true,
	"today":     true,
	"next":      true,
	"projects":  true,
	"tasks":     true,
	"waiting":   true,
	"calendar":  true,
	"someday":   true,
	"scheduler": true,
	"archive":   true,
	"audit":     true,
}

func (s *Server) newPage(title, view string, r *http.Request) *page {
	p := &page{Title: title, View: view, Today: s.app.Today(), Conf: s.conf, Error: r.URL.Query().Get("err")}
	p.FilterMode = "filter"
	p.SelfURL = r.URL.RequestURI()
	p.Live = liveViews[view]
	if v, err := s.app.GetState(agesState); err == nil {
		p.Ages = v == "1"
	}
	p.Theme = s.theme()
	if h, ok := viewHelp[view]; ok {
		p.HelpName, p.HelpText = h.Name, h.Text
	}
	p.Nav, _ = s.app.NavCounts()
	if p.Nav == nil {
		p.Nav = &app.NavCounts{}
	}
	p.Bookmarks = s.bookmarks().rows()
	// the trail starts at the view, with the same count the nav badge shows —
	// one number, one rule, whichever panel you are reading it off. A screen
	// under no view starts at its own title instead, which is all it has.
	if h, ok := viewHelp[view]; ok {
		c := crumb{Slug: view, Name: h.Name, Count: p.Nav.For(view)}
		c.Alert = view == "inbox" && c.Count > 0
		p.Trail = []crumb{c}
	} else {
		p.Trail = []crumb{{Name: title}}
	}
	p.Durations = app.Durations
	// the remembered lists ride on every page: any screen may hold a box that
	// completes a name as it is typed (see implementation.md, "Token boxes"),
	// and two short lists are cheaper to carry than to ask for
	p.Vocab.Contexts, _ = s.app.Contexts()
	p.Vocab.Tags, _ = s.app.Tags()
	// the verbs ride along for the same reason: the title box is marked while
	// it is being typed, which is the key layer's job and not the server's, so
	// the list has to be on the page the box is on (see implementation.md,
	// "The verb a title opens with")
	p.Vocab.Verbs, _ = s.app.Verbs()
	return p
}

// viewFilters implements the persistent per-view filter set: a request
// carrying the "f" marker saves its filters for the view; a bare request
// gets the remembered set back, still applied.
//
// What the view does not filter by is dropped, the way the read API drops it
// (see api.go): every view offers its own subset and answers by that subset
// (design.md, "The filter line"). The screen itself never sends a token
// outside the subset — the box refuses it as it is typed — so this bites on
// exactly two things, a URL written by hand and a bookmark, which is
// view-agnostic by design and so is the one thing that routinely arrives
// holding more than this view can use (design.md, "Bookmarked filters"). It
// is dropped rather than refused: the line is written back out of the
// filters, so what the box shows afterwards is what the list is narrowed by
// and nothing is left sitting in it doing nothing.
func (s *Server) viewFilters(view string, r *http.Request) app.Filters {
	q := r.URL.Query()
	if q.Get("f") == "1" {
		s.app.SetState("filters:"+view, r.URL.RawQuery)
	} else if r.URL.RawQuery == "" {
		if saved, _ := s.app.GetState("filters:" + view); saved != "" {
			if sq, err := url.ParseQuery(saved); err == nil {
				q = sq
			}
		}
	}
	f, _ := app.NarrowToView(view, s.parseFilters(q))
	return f
}

// agesState is the one display flag the app carries, kept where the per-view
// filter sets are kept: it is remembered UI state of exactly the same kind,
// and a single-user app has one place for that.
const agesState = "ages"

// agesToggle flips the flag and comes back to the page it was pressed on. The
// new value is written from what was stored rather than from the request, so
// two presses in flight cannot leave the flag saying the opposite of what the
// last press meant.
func (s *Server) agesToggle(w http.ResponseWriter, r *http.Request) {
	v, err := s.app.GetState(agesState)
	if err != nil {
		httpError(w, err)
		return
	}
	next := "1"
	if v == "1" {
		next = "0"
	}
	if err := s.app.SetState(agesState, next); err != nil {
		httpError(w, err)
		return
	}
	back(w, r)
}

// themeState is which palette was last chosen by hand, kept beside the ages
// flag and the panel state: it is remembered display state of exactly the same
// kind, and a single-user app has one place for that. Empty means nothing has
// been chosen, and the settings file is still the answer.
const themeState = "theme"

// theme resolves the two answers into one. The file says what to open with and
// the screen says what was asked for since, which is the same arrangement
// zen.views and the panel state have: a file cannot be rewritten by a keypress
// (see implementation.md, "Settings file"), so the thing that is pressed is
// remembered where everything pressed is remembered. A stored word the app no
// longer knows falls back to the file rather than painting nothing.
func (s *Server) theme() string {
	v, err := s.app.GetState(themeState)
	if err == nil {
		for _, t := range conf.Themes {
			if v == t {
				return t
			}
		}
	}
	for _, t := range conf.Themes {
		if s.conf.Theme == t {
			return t
		}
	}
	// A Config built by hand rather than read from a file — a test, a tool —
	// has no theme in it at all, and an empty word is not one of the three.
	// Auto is the only answer that could be right for "nobody has said".
	return conf.ThemeAuto
}

// themeSet answers one press on the Settings screen's theme row. The answer is
// named in the path rather than cycled here, because the screen offers all
// three and the pointer may pick any of them — the key presses the next one by
// standing on it (implementation.md, "Theme"), so there is one control per
// answer and no second idea of what "next" means.
func (s *Server) themeSet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	known := false
	for _, t := range conf.Themes {
		known = known || t == name
	}
	if !known {
		http.NotFound(w, r)
		return
	}
	if err := s.app.SetState(themeState, name); err != nil {
		httpError(w, err)
		return
	}
	back(w, r)
}

// panelsToggle answers one press in the ctrl-v dialog: a panel, or zen. Like
// the ages flag it writes what was stored rather than what the page sent, so
// two presses in flight cannot leave the screen saying the opposite of what
// the last one meant — and it comes straight back to the page it was pressed
// on, which is what closes the dialog.
func (s *Server) panelsToggle(w http.ResponseWriter, r *http.Request) {
	which := r.PathValue("which")
	if which != "zen" && !isPanel(which) {
		http.NotFound(w, r)
		return
	}
	if err := s.savePanels(s.panelState().toggle(which)); err != nil {
		httpError(w, err)
		return
	}
	back(w, r)
}

func isPanel(name string) bool {
	for _, p := range panelNames {
		if p == name {
			return true
		}
	}
	return false
}

// doingData is the whole of the doing screen: the action, and where leaving it
// goes. Nothing is queried for it that the action does not already hold —
// design.md, "Doing one action" is explicit that the title is all it shows.
type doingData struct {
	Action *app.Action
	Back   string
}

// doingPage is one action, alone. It is a view like any other now: its own
// URL, so it survives a reload and a back button, and it takes part in the
// panel rules rather than having a set of furniture settings of its own (see
// design.md, "Doing one action").
//
// Where it goes back to rides on the URL rather than on the Referer, because
// the URL is the thing that survives the reload. An action that is already
// done has nothing left to do, so the screen refuses to open on it.
func (s *Server) doingPage(w http.ResponseWriter, r *http.Request) {
	act, err := s.app.Action(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	home := localPath(r.URL.Query().Get("from"), "/next")
	if act.CompletedAt != nil {
		http.Redirect(w, r, home, http.StatusSeeOther)
		return
	}
	p := s.newPage(act.Title, viewOf(home), r).help("doing").step("Doing", "doing")
	p.Timer = true
	p.Data = doingData{Action: act, Back: home}
	s.render(w, "doing.html", p)
}

func filterQuery(f app.Filters) string {
	q := url.Values{}
	if f.Name != "" {
		q.Set("name", f.Name)
	}
	for _, t := range f.Tags {
		q.Add("tag", t)
	}
	for _, c := range f.Contexts {
		q.Add("context", c)
	}
	for _, d := range f.Durations {
		q.Add("duration", string(d))
	}
	if f.Focus != "" {
		q.Set("focus", f.Focus)
	}
	if f.Due != "" {
		q.Set("due", f.Due)
	}
	if f.Completed != "" {
		q.Set("completed", f.Completed)
	}
	if f.Sort != "" {
		q.Set("sort", f.Sort)
	}
	if f.Desc {
		q.Set("desc", "1")
	}
	q.Set("f", "1")
	return q.Encode()
}

// --- login ---------------------------------------------------------------

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "login.html", &page{Title: "unlock"})
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name: "token", Value: strings.TrimSpace(r.FormValue("token")), Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		MaxAge: 3600 * 24 * 365,
	})
	http.Redirect(w, r, "/next", http.StatusSeeOther)
}

// --- simple view pages ---------------------------------------------------

func (s *Server) inboxPage(w http.ResponseWriter, r *http.Request) {
	items, err := s.app.Inbox()
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Inbox", "inbox", r)
	p.Data = items
	s.render(w, "inbox.html", p)
}

func (s *Server) capturePost(w http.ResponseWriter, r *http.Request) {
	if _, _, err := s.app.Capture(r.FormValue("text"), app.SourceApp); err != nil {
		httpError(w, err)
		return
	}
	back(w, r)
}

func (s *Server) somedayPage(w http.ResponseWriter, r *http.Request) {
	f := s.viewFilters("someday", r)
	items, err := s.app.SomedayItems(f)
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Someday/Maybe", "someday", r)
	p.Filters, p.FilterQuery = f, filterQuery(f)
	if f.Active() {
		if all, err := s.app.SomedayItems(app.Filters{}); err == nil {
			p.Hidden = len(all) - len(items)
		}
	}
	// an idea carries the area of responsibility it belongs to and nothing
	// else, so its line asks about tags and words (design.md, "Someday/Maybe")
	p.FilterMode = "filter-tags"
	p.Shown, p.Total = len(items), len(items)+p.Hidden
	p.Query = f.Query()
	p.Data = items
	s.render(w, "someday.html", p)
}

func (s *Server) projectsPage(w http.ResponseWriter, r *http.Request) {
	f := s.viewFilters("projects", r)
	projects, err := s.app.ProjectList(f)
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Projects", "projects", r)
	p.Filters, p.FilterQuery = f, filterQuery(f)
	if f.Active() {
		if all, err := s.app.ProjectList(app.Filters{}); err == nil {
			p.Hidden = len(all) - len(projects)
		}
	}
	// Projects filters by tag and by name and by nothing else (design.md,
	// "Projects"), so its line may not say the rest (see "Token boxes")
	p.FilterMode = "filter-tags"
	p.Shown, p.Total = len(projects), len(projects)+p.Hidden
	p.Query = f.Query()
	p.Data = projects
	s.render(w, "projects.html", p)
}

type actionListPage func(app.Filters) ([]*app.Action, error)

func (s *Server) actionListView(w http.ResponseWriter, r *http.Request, title, view, tmpl string, load actionListPage, mode string) {
	f := s.viewFilters(view, r)
	acts, err := load(f)
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage(title, view, r)
	p.Filters, p.FilterQuery = f, filterQuery(f)
	if f.Active() {
		if all, err := load(app.Filters{Sort: f.Sort, Desc: f.Desc}); err == nil {
			p.Hidden = len(all) - len(acts)
		}
	}
	// what the box says: how many are on the screen, out of how many there
	// are. One number when nothing is filtered, because "6 of 6" is a sum
	// nobody asked for (design.md, "Filtering")
	p.Shown, p.Total = len(acts), len(acts)+p.Hidden
	p.Query = f.Query()
	p.FilterMode = mode
	p.Data = acts
	s.render(w, tmpl, p)
}

// Tasks and Waiting for filter by tag and by name only — design.md gives each
// view the subset it offers, and neither of these two asks "what can I do
// now", which is the question a context or a size answers.
func (s *Server) tasksPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Tasks", "tasks", "tasks.html", s.app.Tasks, "filter-tags")
}

func (s *Server) nextPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Next actions", "next", "next.html", s.app.NextActions, "filter")
}

// filterVocab is what the filter box completes from: the remembered lists,
// plus the names that stand for fields. It is put on the page rather than
// fetched, the way the project picker is (see "Stage two") — the lists are a
// few dozen short words, and a round trip per keystroke to filter them would
// be slower than the typing.

func (s *Server) waitingPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Waiting for", "waiting", "waiting.html", s.app.WaitingFor, "filter-tags")
}

func (s *Server) calendarPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Calendar", "calendar", "calendar.html", s.app.Calendar, "filter-due")
}

func (s *Server) todayPage(w http.ResponseWriter, r *http.Request) {
	tv, err := s.app.TodayItems()
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Today", "today", r)
	p.Data = tv
	s.render(w, "today.html", p)
}

func (s *Server) archivePage(w http.ResponseWriter, r *http.Request) {
	f := s.viewFilters("archive", r)
	entries, err := s.app.Archive(f)
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Archive", "archive", r)
	p.Filters, p.FilterQuery = f, filterQuery(f)
	if f.Active() {
		if all, err := s.app.Archive(app.Filters{}); err == nil {
			p.Hidden = len(all) - len(entries)
		}
	}
	// the finished are narrowed by when, by tag and by name (design.md,
	// "Archive"), so that is all its line may say
	p.FilterMode = "filter-completed"
	p.Shown, p.Total = len(entries), len(entries)+p.Hidden
	p.Query = f.Query()
	p.Data = entries
	s.render(w, "archive.html", p)
}

func (s *Server) schedulerPage(w http.ResponseWriter, r *http.Request) {
	f := s.viewFilters("scheduler", r)
	ss, err := s.app.Schedules(f.Name)
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Scheduler", "scheduler", r)
	// the rules are on every row here, so this is where the question is asked
	p.When = true
	p.Filters, p.FilterQuery = f, filterQuery(f)
	if f.Name != "" {
		if all, err := s.app.Schedules(""); err == nil {
			p.Hidden = len(all) - len(ss)
		}
	}
	// a schedule carries no context and no tag — it is text and a rule — so
	// its line has nothing to say but words (design.md, "Filtering by tag")
	p.FilterMode = "filter-name"
	p.Shown, p.Total = len(ss), len(ss)+p.Hidden
	p.Query = f.Query()
	p.Data = ss
	s.render(w, "scheduler.html", p)
}

type auditRow struct {
	*app.AuditEntry
	Text string // best-effort name pulled from the snapshot, for recapture
}

func (s *Server) auditPage(w http.ResponseWriter, r *http.Request) {
	entries, err := s.app.AuditLog(300)
	if err != nil {
		httpError(w, err)
		return
	}
	rows := make([]auditRow, 0, len(entries))
	for _, e := range entries {
		row := auditRow{AuditEntry: e}
		var snap struct {
			Text  string `json:"text"`
			Title string `json:"title"`
		}
		if json.Unmarshal([]byte(e.Snapshot), &snap) == nil {
			row.Text = snap.Text
			if row.Text == "" {
				row.Text = snap.Title
			}
		}
		rows = append(rows, row)
	}
	p := s.newPage("Audit log", "audit", r)
	p.Data = rows
	s.render(w, "audit.html", p)
}

// --- Inbox Zero ----------------------------------------------------------

type processData struct {
	ID        int64
	Text      string
	Line      string // the capture's first line — what the inbox showed
	Body      string // what it carried under that line, empty for most captures
	Source    string // the way in it arrived by — read here and nowhere else
	CreatedAt time.Time
	Remaining int
	One       bool // processing one named item, not working down the inbox

	// stage two: the branch has been chosen and the form for it is up.
	// Empty As is stage one, the question itself.
	As        string
	Vals      url.Values    // what the fields show — seeded on the way in, echoed back on a bounce
	NeedDOD   bool          // the name matched none, so the project would be a new one
	Note      string        // why the form came back instead of being accepted
	Picker    []pickerData  // every active project, newest activity first, for the picker
	Drafts    []draftAction // the project screen's actions, written before the project exists
	Back      string        // stage one for this item — where "back" and esc go
	AsAction  string
	AsProject string
	AsSomeday string
	Q         string // "?one=1" when a single picked item, to be carried by the form

	Contexts []string
	Tags     []string

	// a completion request replaces stage one rather than adding a branch to
	// it: the line already says what it is, and the only question left is
	// whether it is true (design.md, "Inbox Zero")
	Req   *app.CompletionRequest
	ReqAt string // the exact moment it was finished elsewhere, beside the age word

	// what this capture looks like, shown on stage one and nowhere else
	// (design.md, "Matches while processing"). Open first, since an open one
	// is the warning; the finished ones carry the digits that copy them
	Open     []matchRow
	Done     []matchRow
	OpenMore int // found beyond the nine shown, so the list can say so
	DoneMore int
	Copied   *app.CopySeed // stage two seeded from this finished item, named above the form
}

// matchRow is one line of the match list: the item, and the two things the row
// draws that the item does not carry — its number, and where pressing it goes.
// Built here rather than in the template because the digit and the link are
// one fact said twice, and a template deriving one from the other is where the
// two would drift apart.
type matchRow struct {
	*app.Match
	Key  string // "1".."9" on a finished item; empty on an open one
	Href string // the copy this row opens; empty on an open one
}

// matches fills in the two lists. Only stage one asks — a form that is up has
// been answered, and a list of near-misses beside it would be asking the
// question again after it was settled.
func (s *Server) matches(d *processData) error {
	open, done, openTotal, doneTotal, err := s.app.Matches(d.Text, s.dupRule())
	if err != nil {
		return err
	}
	for _, m := range open {
		// an open match is shown and not pressable: copying something that is
		// already open would write the duplicate this list exists to prevent,
		// and the answer to "it is already on the list" is one of the six
		// below (design.md, "Matches while processing"). It is not a link to
		// the item either — the keys on this screen cannot reach one, and a
		// row only a pointer can open is a control the bar cannot name
		d.Open = append(d.Open, matchRow{Match: m})
	}
	for i, m := range done {
		d.Done = append(d.Done, matchRow{
			Match: m,
			Key:   itoa(int64(i + 1)),
			Href:  d.Back + "&as=" + m.Kind() + "&from=" + itoa(m.ID()),
		})
	}
	d.OpenMore, d.DoneMore = openTotal-len(open), doneTotal-len(done)
	return nil
}

// dupRule is what the settings file says about matching, in the shape the
// domain takes it. One place, so the three keys are read together or not at
// all.
func (s *Server) dupRule() app.DupRule {
	return app.DupRule{Method: s.conf.DupMatch, Overlap: s.conf.DupOverlap, Similar: s.conf.DupSimilar}
}

// copyFrom puts a finished item's seed in the boxes stage two draws, instead
// of the capture's (design.md, "Copying a finished one"). What a copy consists
// of is `App.CopyOf`'s answer; this is only where it lands on the form.
//
// A nil seed means there was nothing to copy — an id that names nothing, or an
// item that is not finished — and the caller then reads the capture as usual,
// because a stale link is not an error the screen has anything to say about.
func (s *Server) copyFrom(d *processData, from int64) error {
	seed, err := s.app.CopyOf(d.As, from)
	if err != nil || seed == nil {
		return err
	}
	d.Copied = seed
	d.Vals.Set("title", seed.Title)
	d.Vals.Set("meta", seed.Meta)
	if d.As == "project" {
		d.Vals.Set("dod", seed.DOD)
		for _, act := range seed.Actions {
			d.Drafts = append(d.Drafts, draftAction{
				Title: act.Title, Meta: act.Meta, Description: act.Description,
			})
		}
		return nil
	}
	// the description the finished action was done from is much of why it is
	// worth copying — the link, the account number, the checklist. The
	// capture's own body goes under it rather than instead of it: both were
	// written about this job, and dropping either would drop it invisibly
	// (design.md, "Copying a finished one")
	d.Vals.Set("description", under(seed.Description, d.Body))
	return nil
}

// under puts the capture's body below what was copied, with a blank line
// between them, and stays out of the way when either is missing.
func under(first, second string) string {
	switch {
	case first == "":
		return second
	case second == "":
		return first
	}
	return first + "\n\n" + second
}

// processItem loads the item a processing screen is about: the named one, or
// the oldest when the Inbox Zero run is working down the list. A nil item with
// no error means the inbox is empty and the run is over.
func (s *Server) processItem(id int64) (*processData, error) {
	d := &processData{ID: id}
	items, err := s.app.Inbox()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	it := items[0]
	if id != 0 {
		it = nil
		for _, cand := range items {
			if cand.ID == id {
				it = cand
				break
			}
		}
		if it == nil {
			return nil, nil // processed already, in another tab or by a back button
		}
	}
	d.ID, d.Text, d.CreatedAt, d.Remaining = it.ID, it.Text, it.CreatedAt, len(items)
	d.Line, d.Body, d.Source = it.Line(), it.Body(), it.Source
	return d, nil
}

// links fills in the URLs the screen needs, now that the item is known. They
// are built here rather than in the template because they all carry the same
// two facts — which item, and whether this is a run or one picked item — and a
// template assembling that four times is four places for it to drift.
func (d *processData) links() {
	one := ""
	if d.One {
		one = "&one=1"
		d.Q = "?one=1"
	}
	base := fmt.Sprintf("/process?item=%d", d.ID)
	d.Back = base + one
	d.AsAction = base + "&as=action" + one
	d.AsProject = base + "&as=project" + one
	d.AsSomeday = base + "&as=someday" + one
}

type pickerData struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Stalled bool   `json:"stalled,omitempty"`
	Open    int    `json:"open,omitempty"`
}

func (s *Server) processPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	d, err := s.processItem(int64Query(r, "item"))
	if err != nil {
		httpError(w, err)
		return
	}
	if d == nil {
		// nothing left to decide about — whether the run just emptied the
		// inbox or the item was decided about in another tab. The empty inbox
		// is the honest answer, and it is the same screen the navigation
		// leads to, so a run ends where the inbox already lives
		http.Redirect(w, r, "/inbox", http.StatusSeeOther)
		return
	}
	d.One = q.Get("one") != ""
	d.As = q.Get("as")
	d.links()
	d.Vals = url.Values{}
	// a request is read before the branches are: the six answers are answers
	// to "what is it?", and this line has already said (design.md, "Inbox
	// Zero"). A branch asked for on the URL is ignored rather than honoured,
	// so a stale link cannot open a form over a request
	req, isReq, rerr := s.app.ReadCompletionRequest(d.Text)
	if rerr != nil {
		httpError(w, rerr)
		return
	}
	if isReq {
		d.As = ""
		d.Req = req
		d.ReqAt = req.At.Format("2 Jan, 15:04")
		s.renderProcess(w, r, d)
		return
	}
	// the captured line is read as notation on the way into a form: what this
	// branch's meta line can hold goes there, and the words that are left are
	// the title it starts from (design.md, "Inbox Zero"). A branch that
	// creates nothing never gets here, which is the whole of why the notation
	// means nothing to Trash, Reference material and the two-minute rule
	v, verr := s.app.Vocabulary()
	if verr != nil {
		httpError(w, verr)
		return
	}
	// only the first line is read that way, and the body goes where the branch
	// keeps material: an action's description, the first draft action's on the
	// project form, and the someday form's one box, which takes the whole
	// capture (design.md, "Inbox Zero"). The DOD is never seeded — it is the
	// one sentence the form exists to force out of you, and a body pre-filled
	// there would satisfy the check that makes a project a project
	switch d.As {
	case "action", "project", "someday":
		// a digit on the match list opens the same form seeded from the
		// finished item instead, since what that answer says is "this again"
		// (design.md, "Copying a finished one"). A `from` that names nothing
		// finished falls back to the capture rather than erroring: it can only
		// be a stale link, and the capture is what the screen is about
		if from := int64Query(r, "from"); from != 0 {
			if err := s.copyFrom(d, from); err != nil {
				httpError(w, err)
				return
			}
		}
		if d.Copied == nil {
			seed := app.SeedCapture(d.As, d.Text, v)
			d.Vals.Set("meta", seed.Meta)
			if d.As == "someday" {
				d.Vals.Set("text", seed.Title)
			} else {
				d.Vals.Set("title", seed.Title)
			}
			// the project form has no description box of its own: its body is
			// written into the first action, by the dialog that adds one
			if d.As == "action" {
				d.Vals.Set("description", seed.Description)
			}
		}
	default:
		d.As = ""
		// the matches belong to the question, not to an answer already given
		if err := s.matches(d); err != nil {
			httpError(w, err)
			return
		}
	}
	s.renderProcess(w, r, d)
}

// renderProcess picks the template the stage calls for and fills in the two
// lists stage two needs. One place, so a form that bounces back comes up
// identical to the one that was submitted.
func (s *Server) renderProcess(w http.ResponseWriter, r *http.Request, d *processData) {
	if d.As != "" {
		d.Contexts, _ = s.app.Contexts()
		d.Tags, _ = s.app.Tags()
	}
	if d.As == "action" {
		cands, _, _ := s.app.ProjectCandidates("", 0)
		d.Picker = make([]pickerData, 0, len(cands))
		for _, c := range cands {
			d.Picker = append(d.Picker, pickerData{c.ID, c.Title, c.Stalled, c.OpenCount})
		}
	}
	tmpl := "process.html"
	if d.Req != nil {
		tmpl = "process_completion.html"
	}
	switch d.As {
	case "action":
		tmpl = "process_action.html"
	case "project":
		tmpl = "process_project.html"
	case "someday":
		tmpl = "process_someday.html"
	}
	p := s.newPage("Processing", "inbox", r).help("processing").step("Processing", "processing")
	switch d.As {
	case "action":
		p.step("Action", "").notation(s)
	case "project":
		p.step("Project", "")
	case "someday":
		p.step("Someday/Maybe", "")
	}
	p.Processing = true
	p.Data = d
	s.render(w, tmpl, p)
}

func int64Query(r *http.Request, key string) int64 {
	return parseID(r.URL.Query().Get(key))
}

func parseID(s string) int64 {
	var id int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		id = id*10 + int64(c-'0')
	}
	return id
}

// writtenAction is an action as the form gives it: a title, a project, a meta
// line and a description. Today does not live on ActionFields — it is a tag
// the app manages — so it is carried alongside and applied by the handler.
type writtenAction struct {
	Fields app.ActionFields
	Today  bool
}

// readAction reads the fields of an action form. The meta line is parsed; the
// description is stored exactly as typed, since nothing is read out of it.
//
// projectID and self are what a `snooze:` naming an action is read against:
// the siblings it may name, and the one it may not, which is this action.
// Both are 0 for an action with no project yet — a capture being filed
// standalone, or the first action of a project that does not exist — and the
// notation then refuses a sibling snooze by itself, because there is nothing
// for it to name.
func (s *Server) readAction(r *http.Request, projectID, self int64) (writtenAction, error) {
	var wa writtenAction
	v, err := s.app.VocabularyIn(projectID, self)
	if err != nil {
		return wa, err
	}
	d, err := app.ParseMeta(r.FormValue("meta"), v)
	if err != nil {
		return wa, err
	}
	wa.Today = d.Today
	wa.Fields = app.ActionFields{
		Title:          strings.TrimSpace(r.FormValue("title")),
		Context:        d.Context,
		ContextParam:   d.ContextParam,
		Duration:       d.Duration,
		NeedsFocus:     d.NeedsFocus,
		Description:    strings.TrimSpace(r.FormValue("description")),
		AssignedTo:     d.AssignedTo,
		DueDate:        d.DueDate,
		SnoozeUntil:    d.SnoozeUntil,
		SnoozeActionID: d.SnoozeActionID,
		Tags:           d.Tags,
	}
	return wa, nil
}

// applyToday makes the #today token mean what the pick dot means. It is not an
// ActionFields value because the tag is the app's to manage — it is cleared
// every morning (design.md, "#today") — so it is set by comparison rather than
// written over.
func (s *Server) applyToday(id int64, want bool) error {
	act, err := s.app.Action(id)
	if err != nil {
		return err
	}
	has := false
	for _, t := range act.Tags {
		if t == app.TodayTag {
			has = true
		}
	}
	if has == want {
		return nil
	}
	return s.app.ToggleTag("action", id, app.TodayTag)
}

func splitContextInput(s string) (name, param string) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "@")
	if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(s, ")") {
		return s[:i], strings.TrimSpace(s[i+1 : len(s)-1])
	}
	return s, ""
}

// draftAction is an action written on the project screen before the project
// exists. It is the three fields an action is written in, carried through the
// form as hidden values so that the project and its actions arrive in one
// submit — until that submit there is nothing for an action to belong to.
type draftAction struct {
	Title       string
	Meta        string
	Description string
}

func draftsFromForm(r *http.Request) []draftAction {
	_ = r.ParseForm()
	at := func(v []string, i int) string {
		if i < len(v) {
			return strings.TrimSpace(v[i])
		}
		return ""
	}
	metas, descs := r.Form["ameta"], r.Form["adescription"]
	var out []draftAction
	for i, t := range r.Form["atitle"] {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}
		out = append(out, draftAction{Title: t, Meta: at(metas, i), Description: at(descs, i)})
	}
	return out
}

// projectMetaFromForm reads the two fields every project-writing form shares:
// a title and a DOD, plus the meta line that carries its tags and its snooze.
func (s *Server) projectMetaFromForm(r *http.Request) (app.ProjectFields, error) {
	pf := app.ProjectFields{
		Title: strings.TrimSpace(r.FormValue("title")),
		DOD:   strings.TrimSpace(r.FormValue("dod")),
	}
	v, err := s.app.Vocabulary()
	if err != nil {
		return pf, err
	}
	pm, err := app.ParseProjectMeta(r.FormValue("meta"), v)
	if err != nil {
		return pf, err
	}
	pf.Tags, pf.SnoozeUntil = pm.Tags, pm.SnoozeUntil
	return pf, nil
}

// projectFromForm reads the project screen: the project, its actions, and
// which of them were picked for today. Today comes back separately because it
// is not an ActionFields value — the tag is the app's to manage, and there is
// no action to hang it on until the project has been created.
func (s *Server) projectFromForm(r *http.Request) (app.ProjectFields, []app.ActionFields, []bool, error) {
	pf, err := s.projectMetaFromForm(r)
	if err != nil {
		return pf, nil, nil, err
	}
	// no project id, so no siblings: an action written here cannot wait on
	// one, because none of them exists yet. `snooze:(…)` refuses itself for
	// that reason rather than needing a rule of its own, and the ordering
	// inside a new plan is written once there is a plan to write it in
	v, err := s.app.VocabularyIn(0, 0)
	if err != nil {
		return pf, nil, nil, err
	}
	var actions []app.ActionFields
	var todays []bool
	for _, d := range draftsFromForm(r) {
		m, err := app.ParseMeta(d.Meta, v)
		if err != nil {
			return pf, nil, nil, fmt.Errorf("%s: %w", d.Title, err)
		}
		actions = append(actions, app.ActionFields{
			Title:        d.Title,
			Description:  d.Description,
			Context:      m.Context,
			ContextParam: m.ContextParam,
			Duration:     m.Duration,
			NeedsFocus:   m.NeedsFocus,
			AssignedTo:   m.AssignedTo,
			DueDate:      m.DueDate,
			SnoozeUntil:  m.SnoozeUntil,
			Tags:         m.Tags,
		})
		todays = append(todays, m.Today)
	}
	return pf, actions, todays, nil
}

// processProjectBranch turns the project form into the project it describes,
// with its actions. Like the action branch it reports whether the branch was
// settled — false means the form has already been sent back, which matters
// more here than anywhere else: the actions live in the form until it is
// accepted, so an error that threw the page away would throw them away too.
func (s *Server) processProjectBranch(w http.ResponseWriter, r *http.Request, id int64) bool {
	pf, actions, todays, err := s.projectFromForm(r)
	if err != nil {
		s.bounce(w, r, id, "project", err.Error(), false)
		return false
	}
	p, err := s.app.ProcessProject(id, pf, actions)
	if err != nil {
		s.bounce(w, r, id, "project", err.Error(), false)
		return false
	}
	// #today on one of them is applied once there is an action to apply it to.
	// The order is the order they were written in, which is the order they
	// were created in.
	for i, want := range todays {
		if !want || i >= len(p.Actions) {
			continue
		}
		if err := s.applyToday(p.Actions[i].ID, true); err != nil {
			httpError(w, err)
			return false
		}
	}
	return true
}

// bounce sends a stage-two form back to the screen instead of accepting it,
// carrying everything that was typed plus the reason. Nothing is written and
// the item is untouched, so this is the same non-answer as leaving.
func (s *Server) bounce(w http.ResponseWriter, r *http.Request, id int64, as, note string, needDOD bool) {
	d, err := s.processItem(id)
	if err != nil || d == nil {
		http.Redirect(w, r, "/inbox", http.StatusSeeOther)
		return
	}
	d.One = r.URL.Query().Get("one") != ""
	d.As, d.Note, d.NeedDOD = as, note, needDOD
	d.Vals = r.Form
	d.Drafts = draftsFromForm(r)
	d.links()
	s.renderProcess(w, r, d)
}

func (s *Server) processBranch(w http.ResponseWriter, r *http.Request) {
	id, branch := idParam(r), r.PathValue("branch")
	var err error
	switch branch {
	case "trash":
		err = s.app.ProcessTrash(id)
	case "reference":
		err = s.app.ProcessReference(id)
	case "twominute":
		err = s.app.ProcessTwoMinute(id)
	case "confirm":
		// yes to a completion request: the action it names is completed at the
		// moment the work was finished, and the request leaves the inbox
		_, err = s.app.ProcessCompletion(id)
	case "action":
		if done := s.processActionBranch(w, r, id); !done {
			return
		}
	case "project":
		if done := s.processProjectBranch(w, r, id); !done {
			return
		}
	case "someday":
		f, ferr := s.readSomeday(r)
		if ferr != nil {
			s.bounce(w, r, id, "someday", ferr.Error(), false)
			return
		}
		_, err = s.app.ProcessSomeday(id, f)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	// one named item goes back to the list; the Inbox Zero run carries on to
	// the next item, and to the done screen when there is none
	if r.URL.Query().Get("one") != "" {
		http.Redirect(w, r, "/inbox", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/process", http.StatusSeeOther)
}

// processActionBranch turns the action form into the thing it describes. The
// project is chosen, not typed, so there is nothing left to resolve: an id
// files it there, a pending new project is created with this action as its
// first, and neither means standalone. It reports whether the branch was
// settled — false means the form has already been sent back.
func (s *Server) processActionBranch(w http.ResponseWriter, r *http.Request, id int64) bool {
	pid := parseID(strings.TrimSpace(r.FormValue("projectid")))
	newProject := strings.TrimSpace(r.FormValue("newproject"))
	wa, err := s.readAction(r, pid, 0)
	if err != nil {
		s.bounce(w, r, id, "action", err.Error(), false)
		return false
	}
	f := wa.Fields

	if newProject != "" {
		// the project and its first action are created together: until the
		// form is submitted there is no action to be its first, and design.md
		// will not have a project without one
		p, err := s.app.ProcessProject(id,
			app.ProjectFields{Title: newProject, DOD: strings.TrimSpace(r.FormValue("newdod"))},
			[]app.ActionFields{f})
		if err != nil {
			s.bounce(w, r, id, "action", err.Error(), false)
			return false
		}
		if len(p.Actions) == 1 {
			return s.finishAction(w, p.Actions[0].ID, wa.Today)
		}
		return true
	}

	act, err := s.app.ProcessAction(id, f, pid)
	if err != nil {
		s.bounce(w, r, id, "action", err.Error(), false)
		return false
	}
	return s.finishAction(w, act.ID, wa.Today)
}

// finishAction applies the one thing that is not written when the action is
// created: whether it was picked for today.
func (s *Server) finishAction(w http.ResponseWriter, id int64, today bool) bool {
	if err := s.applyToday(id, today); err != nil {
		httpError(w, err)
		return false
	}
	return true
}

// --- actions -------------------------------------------------------------

type actionPageData struct {
	Action   *app.Action
	Contexts []string
	Tags     []string
	Back     string // the view this was opened from, for Back and esc
	Siblings string // the actions `snooze:` may name here, as JSON — see siblingsJSON
}

// siblingsJSON is the list the meta box completes a `snooze:` from: this
// project's open actions, less the one being edited, as `[{"id":…,"t":"…"}]`.
//
// It carries the id beside the title because the box has one decision to make
// that the title alone cannot settle: two open actions may share a name, and
// completing to `snooze:(…)` would then write a line the server refuses as
// ambiguous. Knowing the ids, the box writes `snooze:#42` for exactly those
// and the readable form for everything else.
//
// JSON rather than the space-separated attributes the remembered lists use,
// because a title has spaces in it and an id is a second field.
func siblingsJSON(sibs []app.Sibling, self int64) string {
	type entry struct {
		ID    int64  `json:"id"`
		Title string `json:"t"`
	}
	out := []entry{}
	for _, s := range sibs {
		if s.ID == self {
			continue
		}
		out = append(out, entry{ID: s.ID, Title: s.Title})
	}
	if len(out) == 0 {
		return ""
	}
	b, err := json.Marshal(out)
	if err != nil {
		return ""
	}
	return string(b)
}

// homeOf is where an item lives when nothing said where you came from: its
// project, or the pile of standalone ones.
func homeOf(act *app.Action) string {
	if act.ProjectID != 0 {
		return "/project/" + itoa(act.ProjectID)
	}
	return "/tasks"
}

func (s *Server) actionPage(w http.ResponseWriter, r *http.Request) {
	act, err := s.app.Action(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	d := &actionPageData{Action: act, Back: s.parentView(r, homeOf(act))}
	d.Contexts, _ = s.app.Contexts()
	d.Tags, _ = s.app.Tags()
	// what this action's `snooze:` may name. Empty for a standalone one, which
	// has no plan to be ordered inside, and empty on a completed one, whose
	// line is read rather than typed
	if act.CompletedAt == nil {
		sibs, _ := s.app.Siblings(act.ProjectID)
		d.Siblings = siblingsJSON(sibs, act.ID)
	}
	// the title bar says which screen this is, not which item is on it: the
	// item's name is the biggest thing on the page already, and the trail is
	// the one place that answers "where am I" (design.md, "Panels")
	p := s.newPage(act.Title, viewOf(d.Back), r).help("action")
	if act.CompletedAt != nil {
		// a completed action is frozen and this screen only reads it
		// (design.md, "Completion"), so the crumb says so — and the notation
		// panel, which explains how a meta line is typed, is left off: there
		// is no line to type here
		p = p.step("Completed action", "")
	} else {
		p = p.step("Edit action", "").notation(s)
	}
	p.Data = d
	s.render(w, "action.html", p)
}

// promotePage is the project form, seeded from the action that is becoming
// one. It is a screen of its own rather than a fold-out on the action page:
// writing a project is writing a project, and design.md gives that one form
// wherever it happens (see implementation.md, "Writing a project").
func (s *Server) promotePage(w http.ResponseWriter, r *http.Request) {
	act, err := s.app.Action(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	// a completed action is frozen, and promoting it would reshape a finished
	// commitment into a fresh project — bringing it back is the way to that
	// (design.md, "Completion"). The page it came from says so, the way the
	// doing screen turns a finished action away
	if act.ProjectID != 0 || act.CompletedAt != nil {
		http.Redirect(w, r, "/action/"+itoa(act.ID), http.StatusSeeOther)
		return
	}
	d := &promotePageData{
		Action: act,
		Back:   "/action/" + itoa(act.ID),
		From:   s.parentView(r, homeOf(act)),
		// title and description only: design.md, "Promoting an action" sends
		// the tags to the project and leaves everything else with the action,
		// since a context or a size describes doing something
		Drafts: []draftAction{{Title: act.Title, Description: act.Description}},
	}
	p := s.newPage("Promote", viewOf(d.From), r).
		help("action").step("Promote action to project", "").notation(s)
	p.Data = d
	s.render(w, "promote.html", p)
}

type promotePageData struct {
	Action *app.Action
	Back   string // the action, which is where this screen was opened from
	From   string // the view under it, which is where the new project lands
	Drafts []draftAction
}

func (s *Server) actionUpdate(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	act, err := s.app.Action(id)
	if err != nil {
		httpError(w, err)
		return
	}
	wa, err := s.readAction(r, act.ProjectID, id)
	if err != nil {
		httpError(w, err)
		return
	}
	if err := s.app.UpdateAction(id, wa.Fields); err != nil {
		httpError(w, err)
		return
	}
	if err := s.applyToday(id, wa.Today); err != nil {
		httpError(w, err)
		return
	}
	// saving finishes the edit, so it goes back where the screen was opened
	// from — the same place Back and esc go, since the difference between
	// them is only whether the changes were kept
	back(w, r)
}

func (s *Server) actionVerb(w http.ResponseWriter, r *http.Request) {
	id, verb := idParam(r), r.PathValue("verb")
	var err error
	switch verb {
	case "complete":
		act, aerr := s.app.Action(id)
		if aerr != nil {
			httpError(w, aerr)
			return
		}
		if err = s.app.CompleteAction(id); err == nil && act.ProjectID != 0 {
			// completing is the moment with the most context: check the project
			st, serr := s.app.ProjectState(act.ProjectID)
			if serr == nil && !st.HasNext {
				// The project's own page is the ask. It carries no marker of
				// having been reached this way: what it has to say — the
				// Actions heading ringed in red, and Done and Add in the bar
				// under the list — it says however it was arrived at, and a
				// screen that read differently depending on the last key
				// pressed is a second screen to keep true.
				home := "/project/" + itoa(act.ProjectID)
				to := home
				// where the action was opened from goes with it: the screen
				// that asks about the project is still on the way back to
				// that view, and its own Referer is a page that is not one.
				// Unless it is this project — an action opened from its own
				// project comes back to it, and a page cannot be its own way out
				if from := localPath(r.FormValue("back"), ""); from != "" && from != home {
					to += "?from=" + url.QueryEscape(from)
				}
				http.Redirect(w, r, to, http.StatusSeeOther)
				return
			}
		}
	case "uncomplete":
		err = s.app.UncompleteAction(id)
	case "delete":
		err = s.app.DeleteAction(id)
	case "detach":
		err = s.app.Detach(id)
	case "snooze":
		err = s.app.SnoozeAction(id, strings.TrimSpace(r.FormValue("until")))
	case "tag":
		err = s.app.ToggleTag("action", id, r.FormValue("tag"))
	case "pick":
		err = s.app.ToggleTag("action", id, app.TodayTag)
	case "promote":
		// the same form the Project branch of processing reads, because it is
		// the same form (design.md, "Promoting an action")
		pf, actions, todays, ferr := s.projectFromForm(r)
		if err = ferr; err != nil {
			break
		}
		// the action is gone once this succeeds, so the project has to be
		// told where to go back to — the view, not the page that promoted it
		from := localPath(r.FormValue("from"), "")
		var p *app.Project
		if p, err = s.app.Promote(id, pf, actions); err == nil {
			for i, want := range todays {
				if want && i < len(p.Actions) {
					if err = s.applyToday(p.Actions[i].ID, true); err != nil {
						break
					}
				}
			}
			if err == nil {
				to := "/project/" + itoa(p.ID)
				if from != "" {
					to += "?from=" + url.QueryEscape(from)
				}
				http.Redirect(w, r, to, http.StatusSeeOther)
				return
			}
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	back(w, r)
}

func itoa(id int64) string {
	if id == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for id > 0 {
		i--
		buf[i] = byte('0' + id%10)
		id /= 10
	}
	return string(buf[i:])
}

// --- projects ------------------------------------------------------------

type projectPageData struct {
	Project  *app.Project
	Contexts []string
	Tags     []string
	Back     string // the view this was opened from, for Back and esc
}

func (s *Server) projectPage(w http.ResponseWriter, r *http.Request) {
	proj, err := s.app.Project(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	d := &projectPageData{
		Project: proj,
		Back:    s.parentView(r, "/projects"),
	}
	d.Contexts, _ = s.app.Contexts()
	d.Tags, _ = s.app.Tags()
	// the trail says which screen this is, the way the action page does —
	// including that a completed project is only read
	step := "Edit project"
	if proj.CompletedAt != nil {
		step = "Completed project"
	}
	p := s.newPage(proj.Title, viewOf(d.Back), r).step(step, "")
	p.Data = d
	s.render(w, "project.html", p)
}

// projectAddAction is the screen an action is written on for a project that
// already exists. It is the same form the processing screen and an action's
// own page use, with the project answered — see the template for why it is a
// screen and no longer a fold on the project's page.
func (s *Server) projectAddAction(w http.ResponseWriter, r *http.Request) {
	proj, err := s.app.Project(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	// a finished project takes no new work: the form is not offered on its
	// page, and the address is turned away rather than opening a screen whose
	// create button the app would refuse (see internal/app, ErrCompleted)
	if proj.CompletedAt != nil {
		http.Redirect(w, r, "/project/"+itoa(proj.ID), http.StatusSeeOther)
		return
	}
	sibs, _ := s.app.Siblings(proj.ID)
	d := &addActionPageData{
		ProjectID:    proj.ID,
		ProjectTitle: proj.Title,
		Back:         "/project/" + itoa(proj.ID),
		// nothing to exclude: the action being written does not exist yet, so
		// every open action of the project is one it may be filed behind
		Siblings: siblingsJSON(sibs, 0),
	}
	// the trail reads Projects / Add an action, the same shape promoting has:
	// the item this is about is named by the form's own project box, not twice
	p := s.newPage(proj.Title, viewOf(d.Back), r).
		help("action").step("Add an action", "").notation(s)
	p.Data = d
	s.render(w, "action_new.html", p)
}

type addActionPageData struct {
	ProjectID    int64
	ProjectTitle string
	Back         string // the project, which is where this screen was opened from
	Siblings     string // the actions `snooze:` may name here, as JSON
}

func (s *Server) projectUpdate(w http.ResponseWriter, r *http.Request) {
	f, err := s.projectMetaFromForm(r)
	if err != nil {
		httpError(w, err)
		return
	}
	if err := s.app.UpdateProject(idParam(r), f); err != nil {
		httpError(w, err)
		return
	}
	back(w, r)
}

// projectGone is where a project's page goes once the project it is about is
// completed or deleted: the view the page was opened from, which it posts as
// "back". Not back(), because back() falls to the Referer and the Referer here
// is the page of the project that has just stopped being an active one — the
// bug delete on an action's page had. "/projects" is the fallback instead: the
// pile this one was in is the honest answer to "then what".
func projectGone(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, localPath(r.FormValue("back"), "/projects"), http.StatusSeeOther)
}

func (s *Server) projectVerb(w http.ResponseWriter, r *http.Request) {
	id, verb := idParam(r), r.PathValue("verb")
	var err error
	switch verb {
	case "complete":
		if err = s.app.CompleteProject(id); err == nil {
			projectGone(w, r)
			return
		}
	case "uncomplete":
		err = s.app.UncompleteProject(id)
	case "delete":
		if err = s.app.DeleteProject(id); err == nil {
			projectGone(w, r)
			return
		}
	case "tag":
		err = s.app.ToggleTag("project", id, r.FormValue("tag"))
	case "addaction":
		var wa writtenAction
		if wa, err = s.readAction(r, id, 0); err == nil {
			var act *app.Action
			if act, err = s.app.CreateAction(id, wa.Fields); err == nil {
				err = s.applyToday(act.ID, wa.Today)
			}
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/project/"+r.PathValue("id"), http.StatusSeeOther)
}

// --- schedules -----------------------------------------------------------

// typedSchedule reads the boxes of either schedule form. What comes back on a
// refusal is what was typed, not what is stored.
func typedSchedule(r *http.Request) *app.Schedule {
	return &app.Schedule{
		Text:   r.FormValue("text"),
		Rule:   strings.TrimSpace(r.FormValue("rule")),
		Suffix: r.FormValue("suffix"),
	}
}

// renderScheduleNew draws the New-schedule form with the given values in its
// boxes and, on a refusal, the reason above them. A refused schedule cannot
// be a bare 400: When is the one field here you can get wrong by typing
// something perfectly reasonable, and a 400 under hx-boost is not swapped, so
// the screen would sit there looking untouched (implementation.md, "A refused
// schedule comes back").
func (s *Server) renderScheduleNew(w http.ResponseWriter, r *http.Request, sched *app.Schedule, note string) {
	p := s.newPage("New schedule", "scheduler", r).step("New schedule", "")
	p.When = true
	if note != "" {
		p.Error = note
	}
	p.Data = sched
	s.render(w, "schedule_new.html", p)
}

func (s *Server) scheduleNewPage(w http.ResponseWriter, r *http.Request) {
	s.renderScheduleNew(w, r, &app.Schedule{}, "")
}

func (s *Server) scheduleCreate(w http.ResponseWriter, r *http.Request) {
	typed := typedSchedule(r)
	_, err := s.app.CreateSchedule(typed.Text, typed.Rule, typed.Suffix)
	if err != nil {
		s.renderScheduleNew(w, r, typed, err.Error())
		return
	}
	http.Redirect(w, r, "/scheduler", http.StatusSeeOther)
}

func (s *Server) renderSchedule(w http.ResponseWriter, r *http.Request, sched *app.Schedule, note string) {
	p := s.newPage("Schedule", "scheduler", r).step(sched.Text, "")
	p.When = true
	if note != "" {
		p.Error = note
	}
	p.Data = sched
	s.render(w, "schedule.html", p)
}

func (s *Server) schedulePage(w http.ResponseWriter, r *http.Request) {
	sched, err := s.app.Schedule(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	s.renderSchedule(w, r, sched, "")
}

func (s *Server) scheduleUpdate(w http.ResponseWriter, r *http.Request) {
	typed := typedSchedule(r)
	err := s.app.EditSchedule(idParam(r), typed.Text, typed.Rule, typed.Suffix)
	if err != nil {
		sched, loadErr := s.app.Schedule(idParam(r))
		if loadErr != nil {
			httpError(w, loadErr)
			return
		}
		// nothing was written, so the heading goes on reading the stored rule
		// and only the boxes carry what was refused
		sched.Text, sched.Rule, sched.Suffix = typed.Text, typed.Rule, typed.Suffix
		s.renderSchedule(w, r, sched, err.Error())
		return
	}
	http.Redirect(w, r, "/scheduler", http.StatusSeeOther)
}

func (s *Server) scheduleDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteSchedule(idParam(r)); err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/scheduler", http.StatusSeeOther)
}

// --- someday items -------------------------------------------------------

func (s *Server) somedayItemPage(w http.ResponseWriter, r *http.Request) {
	it, err := s.app.SomedayItem(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Someday/Maybe", "someday", r).step(it.Text, "")
	p.Data = it
	s.render(w, "somedayitem.html", p)
}

func (s *Server) somedayItemUpdate(w http.ResponseWriter, r *http.Request) {
	f, err := s.readSomeday(r)
	if err != nil {
		httpError(w, err)
		return
	}
	if err := s.app.EditSomeday(idParam(r), f); err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/someday", http.StatusSeeOther)
}

// readSomeday reads the two fields a someday/maybe item is written in,
// wherever it is being written: the idea, and the tags on its meta line
// (design.md, "Someday/maybe item").
func (s *Server) readSomeday(r *http.Request) (app.SomedayFields, error) {
	f := app.SomedayFields{Text: strings.TrimSpace(r.FormValue("text"))}
	v, err := s.app.Vocabulary()
	if err != nil {
		return f, err
	}
	f.Tags, err = app.ParseSomedayMeta(r.FormValue("meta"), v)
	return f, err
}

// somedayItemToInbox is the only thing this screen does besides saving: the
// idea goes back to be decided about, carrying its tags in its text. It is
// also how an idea is trashed — in the inbox, by the branch that trashes
// everything else (design.md, "Reshaping items").
func (s *Server) somedayItemToInbox(w http.ResponseWriter, r *http.Request) {
	if err := s.app.ReturnToInbox(idParam(r)); err != nil {
		httpError(w, err)
		return
	}
	// the item is a capture again, and the inbox is where it now has to be
	// answered — so that is where the screen goes
	http.Redirect(w, r, "/inbox", http.StatusSeeOther)
}

// --- weekly review -------------------------------------------------------

// reviewStep is one line of the review's running order. Num is both the
// number printed on the line and the key that opens it: one field, because a
// list that numbered its lines in one place and keyed them in another is a
// list where the two eventually disagree (keys.md, "The review's digits").
// The number is the line's position and nothing else — it used to be written
// into the title as well, which put two numberings on every line.
type reviewStep struct {
	Num              int
	Key, Title, Desc string
	// Link is where the number goes. Gather has none: it is a step you do in
	// the calendar and the mail, not a screen this app can open, and a key
	// that pressed nothing is the one thing the key bar may never advertise.
	Link  string
	Count int
}

// The steps, in the order the ritual runs them (design.md, "Weekly review").
func reviewSteps(counts *app.ReviewCounts) []reviewStep {
	steps := []reviewStep{
		{Key: "gather", Title: "Gather", Desc: "Collect open loops from calendar, mail, messengers into the inbox. Check due dates against the external calendar."},
		{Key: "inbox", Title: "Get clear", Desc: "Run Inbox Zero until the inbox is empty. Non-negotiable.", Link: "/process", Count: counts.Inbox},
		{Key: "waiting", Title: "Waiting for", Desc: "Anything stale is chased, or gets a due date / snooze.", Count: counts.WaitingFor},
		{Key: "projects", Title: "Projects", Desc: "Is the DOD still right, and is there a next action?", Count: counts.Projects},
		{Key: "next", Title: "Next actions", Desc: "Still valid, still a real physical next action?", Count: counts.Next},
		{Key: "someday", Title: "Someday/Maybe", Desc: "Promote, re-snooze or trash.", Count: counts.Someday},
		{Key: "scheduler", Title: "Scheduler", Desc: "Still wanted, rule still right?", Count: counts.Schedules},
	}
	for i := range steps {
		steps[i].Num = i + 1
		if steps[i].Link == "" && steps[i].Key != "gather" {
			steps[i].Link = "/review/" + steps[i].Key
		}
	}
	return steps
}

// reviewStepTitle is what the step's own screen is called, read off the same
// list the index draws, so a step is named the same in both places.
func reviewStepTitle(key string) string {
	for _, st := range reviewSteps(&app.ReviewCounts{}) {
		if st.Key == key {
			return st.Title
		}
	}
	return key
}

func (s *Server) reviewPage(w http.ResponseWriter, r *http.Request) {
	counts, err := s.app.ReviewCounts()
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Weekly review", "review", r)
	p.Data = reviewSteps(counts)
	s.render(w, "review.html", p)
}

// reviewItem is one item in a review step, walked or not. SnoozeUntil is set
// only while the snooze is live: a snoozed item is walked like any other,
// and its date is one of the things the walk checks (design.md, "Weekly
// review"), so the row has to show it.
type reviewItem struct {
	Type, Name, Link string
	ID               int64
	LastReviewedAt   time.Time
	Detail           string
	SnoozeUntil      string
	// Reviewed is the mark at the head of the row: this one has been walked
	// this cycle. The step lists the walked ones too, because a mark that can
	// be taken off again has to stay somewhere you can reach it.
	Reviewed bool
}

func (s *Server) reviewStepPage(w http.ResponseWriter, r *http.Request) {
	step := r.PathValue("step")
	today := s.app.Today()
	var items []reviewItem
	outstanding := 0
	add := func(typ, name, link string, id int64, reviewed time.Time, snooze, detail string) {
		it := reviewItem{Type: typ, Name: name, Link: link, ID: id, LastReviewedAt: reviewed,
			Detail: detail, Reviewed: !s.app.OutstandingFor(typ)(reviewed)}
		if !it.Reviewed {
			outstanding++
		}
		if snooze > today {
			it.SnoozeUntil = snooze
		}
		items = append(items, it)
	}
	switch step {
	case "waiting":
		acts, _ := s.app.WaitingFor(app.Filters{})
		for _, a := range acts {
			add("action", a.Title, "/action/"+itoa(a.ID), a.ID, a.LastReviewedAt, a.SnoozeUntil, "waiting on "+a.AssignedTo)
		}
	case "projects":
		projects, _ := s.app.ProjectList(app.Filters{})
		for _, pr := range projects {
			detail := "DOD: " + pr.DOD
			if pr.Stalled {
				detail = "STALLED · " + detail
			}
			add("project", pr.Title, "/project/"+itoa(pr.ID), pr.ID, pr.LastReviewedAt, pr.SnoozeUntil, detail)
		}
	case "next":
		// the review walks the snoozed ones too: their date is one of the
		// claims being checked (design.md, "Weekly review"), which is why
		// this is not the same call the view makes
		acts, _ := s.app.NextActionsWithSnoozed(app.Filters{})
		for _, a := range acts {
			add("action", a.Title, "/action/"+itoa(a.ID), a.ID, a.LastReviewedAt, a.SnoozeUntil, a.ProjectTitle)
		}
	case "someday":
		its, _ := s.app.SomedayItems(app.Filters{})
		for _, it := range its {
			add("someday", it.Text, "/somedayitem/"+itoa(it.ID), it.ID, it.LastReviewedAt, "", "")
		}
	case "scheduler":
		ss, _ := s.app.Schedules("")
		for _, sc := range ss {
			add("schedule", sc.Text, "/schedule/"+itoa(sc.ID), sc.ID, sc.LastReviewedAt, "", sc.RuleReadable)
		}
	default:
		http.NotFound(w, r)
		return
	}
	// Oldest first, which puts what has waited longest at the top and carries
	// the walked ones to the bottom on their own: a stamp made a minute ago
	// is the newest date in the step.
	sort.SliceStable(items, func(i, j int) bool { return items[i].LastReviewedAt.Before(items[j].LastReviewedAt) })
	title := reviewStepTitle(step)
	p := s.newPage("Review · "+title, "review", r).step(title, "")
	p.Data = map[string]any{"Step": step, "Title": title, "Items": items, "Outstanding": outstanding}
	s.render(w, "review_step.html", p)
}

// reviewMark flips one item's mark and says which way it went. The answer is
// JSON where the key layer asked for it: the step is a list being walked down,
// and a page that reloaded and re-sorted at every press would move the next
// row out from under the hands (implementation.md, "The weekly review screens").
func (s *Server) reviewMark(w http.ResponseWriter, r *http.Request) {
	on, err := s.app.ToggleReviewed(r.PathValue("type"), idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"reviewed": on})
		return
	}
	step := r.URL.Query().Get("step")
	if step == "" {
		back(w, r)
		return
	}
	http.Redirect(w, r, "/review/"+step, http.StatusSeeOther)
}

// --- settings ------------------------------------------------------------

type settingsData struct {
	Cloud app.NameCloud
	// Names is every name on both lists as the line writes it, one per line,
	// whatever the filter hides: the key layer offers Create only for a line
	// that is none of them (implementation.md, "The remembered lists")
	Names string
	// Made is the name just created, which the page selects on arrival so
	// that the next key can already be about it
	Made string
	// Themes is the three answers in the order they are offered, each saying
	// whether it is the one in force and which of them carries the key — see
	// implementation.md, "Theme".
	Themes []themeChoice
}

// themeChoice is one answer on the theme row.
type themeChoice struct {
	Name string
	On   bool
	// Key: this is the answer one press gives, so it is the one wearing the
	// letter. Exactly one of the three has it, which is what keeps the bar
	// honest — it says the answer the press lands on rather than the name of
	// the row.
	Key bool
}

// themeChoices is the row: every answer, the one in force marked, and the
// letter on the next one round.
func themeChoices(cur string) []themeChoice {
	next := conf.NextTheme(cur)
	out := make([]themeChoice, 0, len(conf.Themes))
	for _, t := range conf.Themes {
		out = append(out, themeChoice{Name: t, On: t == cur, Key: t == next})
	}
	return out
}

const settingsState = "filters:settings"

// settingsLine is the Settings filter line, remembered the way every view's
// filter set is. Only the line is kept: the address also carries err and made,
// which say what just happened rather than what is being looked at.
func (s *Server) settingsLine(r *http.Request) string {
	q := r.URL.Query()
	if q.Get("f") == "1" {
		line := strings.TrimSpace(q.Get("q"))
		s.app.SetState(settingsState, line)
		return line
	}
	line, _ := s.app.GetState(settingsState)
	return line
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	tags, err := s.app.TagList()
	if err != nil {
		httpError(w, err)
		return
	}
	contexts, err := s.app.ContextList()
	if err != nil {
		httpError(w, err)
		return
	}
	verbs, err := s.app.VerbList()
	if err != nil {
		httpError(w, err)
		return
	}
	line := s.settingsLine(r)
	d := &settingsData{Cloud: app.NarrowNames(tags, contexts, verbs, line), Made: r.URL.Query().Get("made")}
	var names []string
	for _, t := range tags {
		names = append(names, "#"+t.Name)
	}
	for _, c := range contexts {
		names = append(names, "@"+c.Name)
		for _, v := range c.Params {
			names = append(names, "@"+c.Name+"("+v.Name+")")
		}
	}
	d.Names = strings.Join(names, "\n")
	p := s.newPage("Settings", "settings", r)
	d.Themes = themeChoices(p.Theme)
	// the line narrows names rather than items, so it is carried as the name
	// filter: that is what opens the bar on a filtered visit
	p.Filters = app.Filters{Name: line}
	p.FilterMode = "filter-names"
	p.Query = line
	p.Shown, p.Total = d.Cloud.Shown, d.Cloud.Total
	p.Data = d
	s.render(w, "settings.html", p)
}

// settingsCreate is the Settings screen's Create: the line itself is the name,
// and the screen comes back filtered by it, so the one name on it is the one
// just made — or, refused, the line is still there to be corrected.
func (s *Server) settingsCreate(w http.ResponseWriter, r *http.Request) {
	line := strings.TrimSpace(r.FormValue("q"))
	to := "/settings?f=1&q=" + url.QueryEscape(line)
	if err := s.app.CreateName(line); err != nil {
		to += "&err=" + url.QueryEscape(err.Error())
	} else {
		to += "&made=" + url.QueryEscape(line)
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// settingsAdd is where a name is learned. Nothing else teaches the app one:
// a `@name` or `#name` written on a meta line is metadata only if it is
// already on the list, which is what stops @home and @Home drifting apart
// (design.md, "Contexts"). A name that is not on it gets the line refused.
func (s *Server) settingsAdd(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.PathValue("kind") {
	case "tags":
		err = s.app.AddTag(r.FormValue("name"))
	case "contexts":
		err = s.app.AddContext(r.FormValue("name"), r.FormValue("param"))
	default:
		http.NotFound(w, r)
		return
	}
	s.backToSettings(w, r, err)
}

// backToSettings answers a change to the remembered lists. A caller that says
// it wants JSON gets no page at all, only whether it worked: that is the token
// box learning a name in the middle of a line being typed (see "Token boxes"),
// where navigating away would throw away the form it is standing in.
func (s *Server) backToSettings(w http.ResponseWriter, r *http.Request, err error) {
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		http.Redirect(w, r, "/settings?err="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

func (s *Server) settingsRemove(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.PathValue("kind") {
	case "tags":
		err = s.app.RemoveTag(r.FormValue("name"))
	case "contexts":
		err = s.app.RemoveContext(r.FormValue("name"))
	case "params":
		err = s.app.RemoveContextParam(r.FormValue("context"), r.FormValue("value"))
	case "verbs":
		err = s.app.RemoveVerb(r.FormValue("word"))
	default:
		http.NotFound(w, r)
		return
	}
	s.backToSettings(w, r, err)
}
