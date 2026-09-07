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
	"settings":  {"Settings", "the remembered tags and contexts — a name still in use cannot be removed"},

	// Reached only from the Inbox, so it has no nav entry and no slug of its
	// own to be keyed by — the process page asks for this one explicitly.
	"process": {"Processing", "one item, one question — what is it? Every answer files it and takes it off the list it came from. Esc leaves it exactly as it was."},

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
	p.Vocab.Contexts, _ = s.app.Contexts()
	p.Vocab.Tags, _ = s.app.Tags()
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

// page is the data every template gets.
type page struct {
	Title        string
	View         string // active nav entry
	HelpName     string // the view's full name, for the ? panel
	HelpText     string // what this view is for, for the ? panel
	Processing   bool   // the nav slot named by View reads "Processing…" instead
	Notation     bool   // the ? panel also explains how an action is written
	Vocab        struct{ Contexts, Tags []string }
	Filters      app.Filters
	FilterQuery  string // current filter query string (for sort/order links)
	Hidden       int    // how many items the filters hide
	TagCloud     []string
	ContextCloud []string
	Durations    []app.Duration
	Nav          *app.NavCounts
	Today        string
	Ages         bool // the ages on rows are shown rather than hidden
	Error        string
	Data         any
}

func (s *Server) newPage(title, view string, r *http.Request) *page {
	p := &page{Title: title, View: view, Today: s.app.Today(), Error: r.URL.Query().Get("err")}
	if v, err := s.app.GetState(agesState); err == nil {
		p.Ages = v == "1"
	}
	if h, ok := viewHelp[view]; ok {
		p.HelpName, p.HelpText = h.Name, h.Text
	}
	p.Nav, _ = s.app.NavCounts()
	if p.Nav == nil {
		p.Nav = &app.NavCounts{}
	}
	p.TagCloud, _ = s.app.TagsInUse()
	p.ContextCloud, _ = s.app.ContextsInUse()
	p.Durations = app.Durations
	return p
}

// viewFilters implements the persistent per-view filter set: a request
// carrying the "f" marker saves its filters for the view; a bare request
// gets the remembered set back, still applied.
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
	return parseFilters(q)
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
	if _, _, err := s.app.Capture(r.FormValue("text")); err != nil {
		httpError(w, err)
		return
	}
	back(w, r)
}

func (s *Server) somedayPage(w http.ResponseWriter, r *http.Request) {
	f := s.viewFilters("someday", r)
	items, err := s.app.SomedayItems(f.Name)
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Someday/Maybe", "someday", r)
	p.Filters, p.FilterQuery = f, filterQuery(f)
	if f.Name != "" {
		if all, err := s.app.SomedayItems(""); err == nil {
			p.Hidden = len(all) - len(items)
		}
	}
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
	p.Data = projects
	s.render(w, "projects.html", p)
}

type actionListPage func(app.Filters) ([]*app.Action, error)

func (s *Server) actionListView(w http.ResponseWriter, r *http.Request, title, view, tmpl string, load actionListPage) {
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
	p.Data = acts
	s.render(w, tmpl, p)
}

func (s *Server) tasksPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Tasks", "tasks", "tasks.html", s.app.Tasks)
}

func (s *Server) nextPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Next actions", "next", "next.html", s.app.NextActions)
}

func (s *Server) waitingPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Waiting for", "waiting", "waiting.html", s.app.WaitingFor)
}

func (s *Server) calendarPage(w http.ResponseWriter, r *http.Request) {
	s.actionListView(w, r, "Calendar", "calendar", "calendar.html", s.app.Calendar)
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
	p.Filters, p.FilterQuery = f, filterQuery(f)
	if f.Name != "" {
		if all, err := s.app.Schedules(""); err == nil {
			p.Hidden = len(all) - len(ss)
		}
	}
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
	Src       string
	Item      any // *app.InboxItem or *app.SomedayItem
	ID        int64
	Text      string
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
	Q         string // "?one=1" when a single picked item, to be carried by the form

	Contexts []string
	Tags     []string
}

// processItem loads the item a processing screen is about: the named one, or
// the oldest when the Inbox Zero run is working down the list. A nil item with
// no error means the inbox is empty and the run is over.
func (s *Server) processItem(src string, id int64) (*processData, error) {
	d := &processData{Src: src, ID: id}
	switch src {
	case "inbox":
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
		d.Item, d.ID, d.Text, d.CreatedAt, d.Remaining = it, it.ID, it.Text, it.CreatedAt, len(items)
	case "someday":
		it, err := s.app.SomedayItem(id)
		if err != nil {
			return nil, err
		}
		d.Item, d.ID, d.Text, d.CreatedAt, d.Remaining = it, it.ID, it.Text, it.CreatedAt, 1
	default:
		return nil, fmt.Errorf("unknown source %q", src)
	}
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
	base := fmt.Sprintf("/process?src=%s&item=%d", url.QueryEscape(d.Src), d.ID)
	d.Back = base + one
	d.AsAction = base + "&as=action" + one
	d.AsProject = base + "&as=project" + one
}

type pickerData struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Stalled bool   `json:"stalled,omitempty"`
	Open    int    `json:"open,omitempty"`
}

func (s *Server) processPage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	src := q.Get("src")
	if src == "" {
		src = "inbox"
	}
	d, err := s.processItem(src, int64Query(r, "item"))
	if err != nil {
		httpError(w, err)
		return
	}
	if d == nil {
		if src == "someday" || int64Query(r, "item") != 0 {
			// the item was decided about already; the list is the honest answer
			http.Redirect(w, r, listFor(src), http.StatusSeeOther)
			return
		}
		s.render(w, "process_done.html", s.newPage("Inbox Zero", "inbox", r))
		return
	}
	d.One = q.Get("one") != ""
	d.As = q.Get("as")
	d.links()
	d.Vals = url.Values{}
	switch d.As {
	case "action":
		d.Vals.Set("title", d.Text)
	case "project":
		d.Vals.Set("ptitle", d.Text)
		d.Vals.Set("paction", "")
	default:
		d.As = ""
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
	switch d.As {
	case "action":
		tmpl = "process_action.html"
	case "project":
		tmpl = "process_project.html"
	}
	p := s.newPage("Processing", d.Src, r).help("process")
	if d.As == "action" {
		p.notation(s)
	}
	p.Processing = true
	p.Data = d
	s.render(w, tmpl, p)
}

func listFor(src string) string {
	if src == "someday" {
		return "/someday"
	}
	return "/inbox"
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
// line and a description. Parked and Today do not live on ActionFields — one is
// the absence of a timestamp and the other is a tag the app manages — so they
// are carried alongside and applied by the handler.
type writtenAction struct {
	Fields app.ActionFields
	Parked bool
	Today  bool
}

// readAction reads the fields of an action form. The meta line is parsed; the
// description is stored exactly as typed, since nothing is read out of it.
// inProject decides whether #parked means anything, since a standalone action
// is always a next action (design.md, "Standalone actions").
func (s *Server) readAction(r *http.Request, inProject bool) (writtenAction, error) {
	var wa writtenAction
	v, err := s.app.Vocabulary()
	if err != nil {
		return wa, err
	}
	d, err := app.ParseMeta(r.FormValue("meta"), v, inProject)
	if err != nil {
		return wa, err
	}
	wa.Parked, wa.Today = d.Parked, d.Today
	wa.Fields = app.ActionFields{
		Title:        strings.TrimSpace(r.FormValue("title")),
		Context:      d.Context,
		ContextParam: d.ContextParam,
		Duration:     d.Duration,
		NeedsFocus:   d.NeedsFocus,
		Description:  strings.TrimSpace(r.FormValue("description")),
		AssignedTo:   d.AssignedTo,
		DueDate:      d.DueDate,
		SnoozeUntil:  d.SnoozeUntil,
		Tags:         d.Tags,
	}
	return wa, nil
}

// setParked makes the #parked token mean what the Park button means. Like
// today it is set by comparison rather than written over: the underlying field
// is a timestamp, and restamping one that was already set would reset an age
// that nothing asked to reset.
func (s *Server) setParked(id int64, before *app.Action, parked bool) error {
	if before.ProjectID == 0 {
		return nil // a standalone action is always a next action
	}
	if (before.BecameNextAt == nil) == parked {
		return nil
	}
	return s.app.SetNext(id, !parked)
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
		Title: strings.TrimSpace(r.FormValue("ptitle")),
		DOD:   strings.TrimSpace(r.FormValue("dod")),
	}
	v, err := s.app.Vocabulary()
	if err != nil {
		return pf, err
	}
	pm, err := app.ParseProjectMeta(r.FormValue("pmeta"), v)
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
	v, err := s.app.Vocabulary()
	if err != nil {
		return pf, nil, nil, err
	}
	var actions []app.ActionFields
	var todays []bool
	for _, d := range draftsFromForm(r) {
		// inProject, because that is what it is about to be
		m, err := app.ParseMeta(d.Meta, v, true)
		if err != nil {
			return pf, nil, nil, fmt.Errorf("%s: %w", d.Title, err)
		}
		if m.Parked {
			return pf, nil, nil, fmt.Errorf("%s: an action written here becomes a next action of the new project — #%s only means something once the project exists", d.Title, app.ParkedTag)
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

// promoteFromForm reads the promote form, which writes a project the same way
// but still types its actions as plain titles: promoting is one screen away
// from the action being promoted, and the actions it opens with are a list to
// sketch rather than a set of items to write in full.
func (s *Server) promoteFromForm(r *http.Request) (app.ProjectFields, []app.ActionFields, error) {
	pf, err := s.projectMetaFromForm(r)
	if err != nil {
		return pf, nil, err
	}
	var actions []app.ActionFields
	for i, t := range r.Form["paction"] {
		if t = strings.TrimSpace(t); t == "" {
			continue
		}
		af := app.ActionFields{Title: t}
		// a project has no description of its own any more, so material that
		// came with the item lands on the first action — the same commitment,
		// and where design.md now says such material lives
		if i == 0 {
			af.Description = strings.TrimSpace(r.FormValue("adescription"))
		}
		actions = append(actions, af)
	}
	return pf, actions, nil
}

// processProjectBranch turns the project form into the project it describes,
// with its actions. Like the action branch it reports whether the branch was
// settled — false means the form has already been sent back, which matters
// more here than anywhere else: the actions live in the form until it is
// accepted, so an error that threw the page away would throw them away too.
func (s *Server) processProjectBranch(w http.ResponseWriter, r *http.Request, src string, id int64) bool {
	pf, actions, todays, err := s.projectFromForm(r)
	if err != nil {
		s.bounce(w, r, src, id, "project", err.Error(), false)
		return false
	}
	p, err := s.app.ProcessProject(src, id, pf, actions)
	if err != nil {
		s.bounce(w, r, src, id, "project", err.Error(), false)
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
func (s *Server) bounce(w http.ResponseWriter, r *http.Request, src string, id int64, as, note string, needDOD bool) {
	d, err := s.processItem(src, id)
	if err != nil || d == nil {
		http.Redirect(w, r, listFor(src), http.StatusSeeOther)
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
	src, id, branch := r.PathValue("src"), idParam(r), r.PathValue("branch")
	var err error
	switch branch {
	case "trash":
		err = s.app.ProcessTrash(src, id)
	case "reference":
		err = s.app.ProcessReference(src, id)
	case "twominute":
		err = s.app.ProcessTwoMinute(src, id)
	case "action":
		if done := s.processActionBranch(w, r, src, id); !done {
			return
		}
	case "project":
		if done := s.processProjectBranch(w, r, src, id); !done {
			return
		}
	case "someday":
		text := strings.TrimSpace(r.FormValue("text"))
		_, err = s.app.ProcessSomeday(id, text, strings.TrimSpace(r.FormValue("snooze")))
	case "keep":
		err = s.app.KeepIncubating(id, strings.TrimSpace(r.FormValue("snooze")))
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	if src == "someday" {
		http.Redirect(w, r, "/someday", http.StatusSeeOther)
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
func (s *Server) processActionBranch(w http.ResponseWriter, r *http.Request, src string, id int64) bool {
	pid := parseID(strings.TrimSpace(r.FormValue("projectid")))
	newProject := strings.TrimSpace(r.FormValue("newproject"))
	wa, err := s.readAction(r, pid != 0 || newProject != "")
	if err != nil {
		s.bounce(w, r, src, id, "action", err.Error(), false)
		return false
	}
	f := wa.Fields

	if newProject != "" {
		// the project and its first action are created together: until the
		// form is submitted there is no action to be its first, and design.md
		// will not have a project without one
		p, err := s.app.ProcessProject(src, id,
			app.ProjectFields{Title: newProject, DOD: strings.TrimSpace(r.FormValue("newdod"))},
			[]app.ActionFields{f})
		if err != nil {
			s.bounce(w, r, src, id, "action", err.Error(), false)
			return false
		}
		if len(p.Actions) == 1 {
			return s.finishAction(w, p.Actions[0].ID, wa.Today)
		}
		return true
	}

	act, err := s.app.ProcessAction(src, id, f, pid, wa.Parked && pid != 0)
	if err != nil {
		s.bounce(w, r, src, id, "action", err.Error(), false)
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
}

func (s *Server) actionPage(w http.ResponseWriter, r *http.Request) {
	act, err := s.app.Action(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	d := &actionPageData{Action: act}
	d.Contexts, _ = s.app.Contexts()
	d.Tags, _ = s.app.Tags()
	p := s.newPage(act.Title, "", r).help("action").notation(s)
	p.Data = d
	s.render(w, "action.html", p)
}

func (s *Server) actionUpdate(w http.ResponseWriter, r *http.Request) {
	id := idParam(r)
	act, err := s.app.Action(id)
	if err != nil {
		httpError(w, err)
		return
	}
	wa, err := s.readAction(r, act.ProjectID != 0)
	if err != nil {
		httpError(w, err)
		return
	}
	if err := s.app.UpdateAction(id, wa.Fields); err != nil {
		httpError(w, err)
		return
	}
	if err := s.setParked(id, act, wa.Parked); err != nil {
		httpError(w, err)
		return
	}
	if err := s.applyToday(id, wa.Today); err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/action/"+r.PathValue("id"), http.StatusSeeOther)
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
				http.Redirect(w, r, "/project/"+itoa(act.ProjectID)+"?ask=1", http.StatusSeeOther)
				return
			}
		}
	case "uncomplete":
		err = s.app.UncompleteAction(id)
	case "delete":
		err = s.app.DeleteAction(id)
	case "detach":
		err = s.app.Detach(id)
	case "next":
		err = s.app.SetNext(id, true)
	case "park":
		err = s.app.SetNext(id, false)
	case "snooze":
		err = s.app.SnoozeAction(id, strings.TrimSpace(r.FormValue("until")))
	case "tag":
		err = s.app.ToggleTag("action", id, r.FormValue("tag"))
	case "pick":
		err = s.app.ToggleTag("action", id, app.TodayTag)
	case "promote":
		var pf app.ProjectFields
		var actions []app.ActionFields
		if pf, actions, err = s.promoteFromForm(r); err != nil {
			break
		}
		var p *app.Project
		if p, err = s.app.Promote(id, pf, actions); err == nil {
			http.Redirect(w, r, "/project/"+itoa(p.ID), http.StatusSeeOther)
			return
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
	Ask      bool // show the after-completion prompt
	Contexts []string
	Tags     []string
}

func (s *Server) projectPage(w http.ResponseWriter, r *http.Request) {
	proj, err := s.app.Project(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	d := &projectPageData{Project: proj, Ask: r.URL.Query().Get("ask") == "1"}
	d.Contexts, _ = s.app.Contexts()
	d.Tags, _ = s.app.Tags()
	p := s.newPage(proj.Title, "projects", r)
	p.Data = d
	s.render(w, "project.html", p)
}

func (s *Server) projectUpdate(w http.ResponseWriter, r *http.Request) {
	f := app.ProjectFields{
		Title: strings.TrimSpace(r.FormValue("title")),
		DOD:   strings.TrimSpace(r.FormValue("dod")),
	}
	v, err := s.app.Vocabulary()
	if err != nil {
		httpError(w, err)
		return
	}
	pm, err := app.ParseProjectMeta(r.FormValue("meta"), v)
	if err != nil {
		httpError(w, err)
		return
	}
	f.Tags, f.SnoozeUntil = pm.Tags, pm.SnoozeUntil
	if err := s.app.UpdateProject(idParam(r), f); err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/project/"+r.PathValue("id"), http.StatusSeeOther)
}

func (s *Server) projectVerb(w http.ResponseWriter, r *http.Request) {
	id, verb := idParam(r), r.PathValue("verb")
	var err error
	switch verb {
	case "complete":
		if err = s.app.CompleteProject(id); err == nil {
			http.Redirect(w, r, "/projects", http.StatusSeeOther)
			return
		}
	case "uncomplete":
		err = s.app.UncompleteProject(id)
	case "delete":
		if err = s.app.DeleteProject(id); err == nil {
			http.Redirect(w, r, "/projects", http.StatusSeeOther)
			return
		}
	case "tag":
		err = s.app.ToggleTag("project", id, r.FormValue("tag"))
	case "addaction":
		var wa writtenAction
		if wa, err = s.readAction(r, true); err == nil {
			var act *app.Action
			if act, err = s.app.CreateAction(id, wa.Fields, wa.Parked); err == nil {
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

func (s *Server) scheduleNewPage(w http.ResponseWriter, r *http.Request) {
	p := s.newPage("New schedule", "scheduler", r)
	s.render(w, "schedule_new.html", p)
}

func (s *Server) scheduleCreate(w http.ResponseWriter, r *http.Request) {
	_, err := s.app.CreateSchedule(r.FormValue("text"), strings.TrimSpace(r.FormValue("rule")), r.FormValue("suffix"))
	if err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/scheduler", http.StatusSeeOther)
}

func (s *Server) schedulePage(w http.ResponseWriter, r *http.Request) {
	sched, err := s.app.Schedule(idParam(r))
	if err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Schedule", "scheduler", r)
	p.Data = sched
	s.render(w, "schedule.html", p)
}

func (s *Server) scheduleUpdate(w http.ResponseWriter, r *http.Request) {
	err := s.app.EditSchedule(idParam(r), r.FormValue("text"), strings.TrimSpace(r.FormValue("rule")), r.FormValue("suffix"))
	if err != nil {
		httpError(w, err)
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
	p := s.newPage("Someday/Maybe", "someday", r)
	p.Data = it
	s.render(w, "somedayitem.html", p)
}

func (s *Server) somedayItemUpdate(w http.ResponseWriter, r *http.Request) {
	err := s.app.EditSomeday(idParam(r), r.FormValue("text"), strings.TrimSpace(r.FormValue("snooze")))
	if err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/someday", http.StatusSeeOther)
}

func (s *Server) somedayItemVerb(w http.ResponseWriter, r *http.Request) {
	id, verb := idParam(r), r.PathValue("verb")
	var err error
	switch verb {
	case "trash":
		err = s.app.TrashSomeday(id)
	case "keep":
		err = s.app.KeepIncubating(id, strings.TrimSpace(r.FormValue("snooze")))
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		httpError(w, err)
		return
	}
	http.Redirect(w, r, "/someday", http.StatusSeeOther)
}

// --- weekly review -------------------------------------------------------

type reviewStep struct {
	Key, Title, Desc string
	Count            int
}

func (s *Server) reviewPage(w http.ResponseWriter, r *http.Request) {
	counts, err := s.app.ReviewCounts()
	if err != nil {
		httpError(w, err)
		return
	}
	steps := []reviewStep{
		{"gather", "0 · Gather", "Collect open loops from calendar, mail, messengers into the inbox. Check due dates against the external calendar.", 0},
		{"inbox", "1 · Get clear", "Run Inbox Zero until the inbox is empty. Non-negotiable.", counts.Inbox},
		{"waiting", "2 · Waiting for", "Anything stale is chased, or gets a due date / snooze.", counts.WaitingFor},
		{"projects", "3 · Projects", "Is the DOD still right, and is there a next action?", counts.Projects},
		{"next", "4 · Next actions", "Still valid, still a real physical next action?", counts.Next},
		{"someday", "5 · Someday/Maybe", "Promote, re-snooze or trash.", counts.Someday},
		{"scheduler", "6 · Scheduler", "Still wanted, rule still right?", counts.Schedules},
	}
	p := s.newPage("Weekly review", "review", r)
	p.Data = steps
	s.render(w, "review.html", p)
}

// reviewItem is one outstanding item in a review step.
type reviewItem struct {
	Type, Name, Link string
	ID               int64
	LastReviewedAt   time.Time
	Detail           string
}

func (s *Server) reviewStepPage(w http.ResponseWriter, r *http.Request) {
	step := r.PathValue("step")
	var items []reviewItem
	add := func(typ, name, link string, id int64, reviewed time.Time, snooze, detail string) {
		if s.app.Outstanding(reviewed, snooze) {
			items = append(items, reviewItem{Type: typ, Name: name, Link: link, ID: id, LastReviewedAt: reviewed, Detail: detail})
		}
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
		acts, _ := s.app.NextActions(app.Filters{})
		for _, a := range acts {
			add("action", a.Title, "/action/"+itoa(a.ID), a.ID, a.LastReviewedAt, a.SnoozeUntil, a.ProjectTitle)
		}
	case "someday":
		its, _ := s.app.SomedayItems("")
		for _, it := range its {
			add("someday", it.Text, "/somedayitem/"+itoa(it.ID), it.ID, it.LastReviewedAt, it.SnoozeUntil, "")
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
	sort.SliceStable(items, func(i, j int) bool { return items[i].LastReviewedAt.Before(items[j].LastReviewedAt) })
	p := s.newPage("Review · "+step, "review", r)
	p.Data = map[string]any{"Step": step, "Items": items}
	s.render(w, "review_step.html", p)
}

func (s *Server) reviewDone(w http.ResponseWriter, r *http.Request) {
	if err := s.app.MarkReviewed(r.PathValue("type"), idParam(r)); err != nil {
		httpError(w, err)
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
	Tags     []app.NameUse
	Contexts []app.NameUse
}

func (s *Server) settingsPage(w http.ResponseWriter, r *http.Request) {
	d := &settingsData{}
	var err error
	if d.Tags, err = s.app.TagList(); err != nil {
		httpError(w, err)
		return
	}
	if d.Contexts, err = s.app.ContextList(); err != nil {
		httpError(w, err)
		return
	}
	p := s.newPage("Settings", "settings", r)
	p.Data = d
	s.render(w, "settings.html", p)
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

func (s *Server) backToSettings(w http.ResponseWriter, r *http.Request, err error) {
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
	default:
		http.NotFound(w, r)
		return
	}
	s.backToSettings(w, r, err)
}
