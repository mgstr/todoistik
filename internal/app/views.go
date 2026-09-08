package app

import (
	"database/sql"
	"sort"
	"strings"
	"time"
)

// Filters is the one set of filters views understand; each view uses the
// subset design.md gives it. All zero values mean "off" — the complete list.
type Filters struct {
	Name      string     `json:"name,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	Contexts  []string   `json:"contexts,omitempty"` // "grocery" or "grocery(Selver)"
	Durations []Duration `json:"durations,omitempty"`
	Focus     string     `json:"focus,omitempty"`     // "", "exclude", "only"
	Due       string     `json:"due,omitempty"`       // "", today, tomorrow, thisweek, nextweek
	Completed string     `json:"completed,omitempty"` // "", today, yesterday, thisweek, lastweek
	Sort      string     `json:"sort,omitempty"`      // "age" (default) or "title"
	Desc      bool       `json:"desc,omitempty"`
}

func (f Filters) Active() bool {
	return f.Name != "" || len(f.Tags) > 0 || len(f.Contexts) > 0 ||
		len(f.Durations) > 0 || f.Focus != "" || f.Due != "" || f.Completed != ""
}

// matchName: every whitespace-separated word must appear as a
// case-insensitive substring somewhere in one of the names.
func matchName(filter string, names ...string) bool {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true
	}
	hay := strings.ToLower(strings.Join(names, "\n"))
	for _, w := range strings.Fields(strings.ToLower(filter)) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// matchTags: selected tags combine with OR; an item with no tags is
// excluded as soon as any tag is selected.
func matchTags(selected, own []string) bool {
	if len(selected) == 0 {
		return true
	}
	for _, s := range selected {
		for _, t := range own {
			if s == t {
				return true
			}
		}
	}
	return false
}

// matchContexts: the question is "is this action's context among the ones I am
// asking for". Asking for none is asking for all of them, and asking for one
// is asking for the actions that carry it — an action with no context is not
// one of those, and is found under "all" (design.md, "Filtering by context").
// Selecting "grocery(Selver)" satisfies @grocery(Selver) and bare @grocery;
// selecting bare "grocery" satisfies only bare @grocery (the parameterised
// form is narrower and needs the specific place).
func matchContexts(selected []string, ctx, param string) bool {
	if len(selected) == 0 {
		return true
	}
	if ctx == "" {
		return false
	}
	for _, s := range selected {
		sName, sParam := splitContext(s)
		if sName != ctx {
			continue
		}
		if param == "" || param == sParam {
			return true
		}
	}
	return false
}

func splitContext(s string) (name, param string) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "@")
	if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(s, ")") {
		return s[:i], s[i+1 : len(s)-1]
	}
	return s, ""
}

func matchDuration(selected []Duration, d Duration) bool {
	if len(selected) == 0 {
		return true
	}
	for _, s := range selected {
		if s == d {
			return true
		}
	}
	return false
}

func matchFocus(mode string, needsFocus bool) bool {
	switch mode {
	case "exclude":
		return !needsFocus
	case "only":
		return needsFocus
	}
	return true
}

// weekOf returns the Monday..Sunday range containing d.
func weekOf(d time.Time) (from, to string) {
	shift := (int(d.Weekday()) + 6) % 7 // Monday=0
	mon := d.AddDate(0, 0, -shift)
	return mon.Format(DateFormat), mon.AddDate(0, 0, 6).Format(DateFormat)
}

// dueBucket: calendar periods, not rolling windows. Overdue items are shown
// whatever the due filter says — already late is not "coming".
func (a *App) matchDue(bucket, due, today string) bool {
	if bucket == "" || due == "" {
		return true
	}
	if due < today {
		return true // overdue always shown
	}
	now, _ := time.Parse(DateFormat, today)
	switch bucket {
	case "today":
		return due == today
	case "tomorrow":
		return due == now.AddDate(0, 0, 1).Format(DateFormat)
	case "thisweek":
		from, to := weekOf(now)
		return due >= from && due <= to
	case "nextweek":
		from, to := weekOf(now.AddDate(0, 0, 7))
		return due >= from && due <= to
	}
	return true
}

func (a *App) matchCompleted(bucket string, completedAt *time.Time) bool {
	if bucket == "" {
		return true
	}
	if completedAt == nil {
		return false
	}
	day := completedAt.In(a.loc).Format(DateFormat)
	today := a.Today()
	now, _ := time.Parse(DateFormat, today)
	switch bucket {
	case "today":
		return day == today
	case "yesterday":
		return day == now.AddDate(0, 0, -1).Format(DateFormat)
	case "thisweek":
		from, to := weekOf(now)
		return day >= from && day <= to
	case "lastweek":
		from, to := weekOf(now.AddDate(0, 0, -7))
		return day >= from && day <= to
	}
	return true
}

// --- loading -------------------------------------------------------------

// loadActions loads actions matching a SQL condition, with tags and project
// titles attached.
func (a *App) loadActions(where string, args ...any) ([]*Action, error) {
	rows, err := a.db.Query(`SELECT `+actionCols+`, COALESCE(p.title,'')
		FROM actions a LEFT JOIN projects p ON p.id = a.project_id
		WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Action
	byID := map[int64]*Action{}
	for rows.Next() {
		act := &Action{}
		var pid sql.NullInt64
		var created, reviewed, dur string
		var next, completed sql.NullString
		if err := rows.Scan(&act.ID, &pid, &act.Title, &act.Context, &act.ContextParam, &dur,
			&act.NeedsFocus, &act.Description, &act.AssignedTo, &act.DueDate, &created,
			&reviewed, &next, &act.SnoozeUntil, &completed, &act.ProjectTitle); err != nil {
			return nil, err
		}
		act.ProjectID = pid.Int64
		act.Duration = Duration(dur)
		act.CreatedAt = parseTS(created)
		act.LastReviewedAt = parseTS(reviewed)
		act.BecameNextAt = parseTSPtr(next)
		act.CompletedAt = parseTSPtr(completed)
		out = append(out, act)
		byID[act.ID] = act
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	trows, err := a.db.Query(`SELECT item_id, tag FROM item_tags WHERE item_type='action' ORDER BY tag`)
	if err != nil {
		return nil, err
	}
	defer trows.Close()
	for trows.Next() {
		var id int64
		var tag string
		if err := trows.Scan(&id, &tag); err != nil {
			return nil, err
		}
		if act, ok := byID[id]; ok {
			act.Tags = append(act.Tags, tag)
		}
	}
	return out, trows.Err()
}

func (a *App) filterActions(acts []*Action, f Filters, useContexts, useDuration, useFocus bool) []*Action {
	out := acts[:0]
	for _, act := range acts {
		if !matchName(f.Name, act.Title) {
			continue
		}
		if !matchTags(f.Tags, act.Tags) {
			continue
		}
		if useContexts && !matchContexts(f.Contexts, act.Context, act.ContextParam) {
			continue
		}
		if useDuration && !matchDuration(f.Durations, act.Duration) {
			continue
		}
		if useFocus && !matchFocus(f.Focus, act.NeedsFocus) {
			continue
		}
		out = append(out, act)
	}
	return out
}

// sortActions orders by age (becameNextActionAt, oldest first) or title.
func sortActions(acts []*Action, f Filters) {
	sort.SliceStable(acts, func(i, j int) bool {
		var less bool
		if f.Sort == "title" {
			less = strings.ToLower(acts[i].Title) < strings.ToLower(acts[j].Title)
		} else {
			ti, tj := acts[i].CreatedAt, acts[j].CreatedAt
			if acts[i].BecameNextAt != nil {
				ti = *acts[i].BecameNextAt
			}
			if acts[j].BecameNextAt != nil {
				tj = *acts[j].BecameNextAt
			}
			less = ti.Before(tj)
		}
		if f.Desc {
			return !less
		}
		return less
	})
}

// --- the views -----------------------------------------------------------

// NextActions: becameNextActionAt set, not completed, not assigned. The
// main working view; all its filters apply.
func (a *App) NextActions(f Filters) ([]*Action, error) {
	acts, err := a.loadActions(`a.became_next_at IS NOT NULL AND a.completed_at IS NULL AND a.assigned_to = ''`)
	if err != nil {
		return nil, err
	}
	acts = a.filterActions(acts, f, true, true, true)
	sortActions(acts, f)
	return acts, nil
}

// Tasks: the standalone actions, whatever their state — Tasks answers
// "where does this action live", not "is it mine to act on".
func (a *App) Tasks(f Filters) ([]*Action, error) {
	acts, err := a.loadActions(`a.project_id IS NULL AND a.completed_at IS NULL`)
	if err != nil {
		return nil, err
	}
	acts = a.filterActions(acts, f, false, false, false)
	sortActions(acts, f)
	return acts, nil
}

// WaitingFor: next actions with "assigned to" set — tracked, but the ball
// is not in your court.
func (a *App) WaitingFor(f Filters) ([]*Action, error) {
	acts, err := a.loadActions(`a.became_next_at IS NOT NULL AND a.completed_at IS NULL AND a.assigned_to != ''`)
	if err != nil {
		return nil, err
	}
	acts = a.filterActions(acts, f, false, false, false)
	sortActions(acts, f)
	return acts, nil
}

// Calendar: everything with a real deadline, soonest first.
func (a *App) Calendar(f Filters) ([]*Action, error) {
	acts, err := a.loadActions(`a.due_date != '' AND a.completed_at IS NULL`)
	if err != nil {
		return nil, err
	}
	today := a.Today()
	out := acts[:0]
	for _, act := range acts {
		if !matchName(f.Name, act.Title) || !matchTags(f.Tags, act.Tags) || !a.matchDue(f.Due, act.DueDate, today) {
			continue
		}
		out = append(out, act)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DueDate < out[j].DueDate })
	return out, nil
}

// TodayView: what has run out of time, and what was picked this morning.
type TodayView struct {
	OutOfTime []*Action `json:"outOfTime"`
	Picked    []*Action `json:"picked"`
}

// Today: the Calendar's today-and-earlier slice unchanged, plus the actions
// carrying #today. No filters, by design.
func (a *App) TodayItems() (*TodayView, error) {
	today := a.Today()
	out, err := a.loadActions(`a.due_date != '' AND a.due_date <= ? AND a.completed_at IS NULL`, today)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].DueDate < out[j].DueDate })
	picked, err := a.loadActions(`a.completed_at IS NULL AND a.id IN
		(SELECT item_id FROM item_tags WHERE item_type='action' AND tag=?)`, TodayTag)
	if err != nil {
		return nil, err
	}
	sortActions(picked, Filters{})
	return &TodayView{OutOfTime: out, Picked: picked}, nil
}

// ProjectList: the active projects with their actions, stalled ones marked.
// The name filter matches a project by its title or the title of any action
// under it; the tag cloud matches the project's own tags only.
func (a *App) ProjectList(f Filters) ([]*Project, error) {
	return a.projectsWhere(`completed_at IS NULL`, f, false)
}

// MatchProjects: the active projects a typed name picks out, by the rule the
// name filter uses — every whitespace-separated word a case-insensitive
// substring (design.md, "Filtering by name"). Matched on the title alone,
// unlike ProjectList's filter, which also looks inside a project's actions:
// that is right when you are searching for a project and wrong when you are
// naming the one an action should join, where a stray match on some action's
// wording would file it somewhere you never named. Empty picks out nothing —
// a standalone action is what an empty box means, not "every project".
func (a *App) MatchProjects(q string) ([]*Project, error) {
	if strings.TrimSpace(q) == "" {
		return nil, nil
	}
	all, err := a.ProjectList(Filters{})
	if err != nil {
		return nil, err
	}
	var hits []*Project
	for _, p := range all {
		if matchName(q, p.Title) {
			hits = append(hits, p)
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Title < hits[j].Title })
	return hits, nil
}

// ProjectCandidate is one row of the picker offered when an action is being
// given a home: the project, plus the two things worth knowing before filing
// into it — how much is open there, and whether it is stalled (the Project
// itself carries that, and design.md says filing a next action into a stalled
// project is exactly what resolves the stall).
type ProjectCandidate struct {
	*Project
	OpenCount int
	LastUsed  time.Time
}

// ProjectCandidates: the active projects offered while naming a home for an
// action. An empty query offers all of them, a non-empty one narrows by the
// rule the name filter uses (see MatchProjects).
//
// Ordered by recent activity, which during a run is the useful order: several
// captures in a row usually belong to the same outcome, so the project wanted
// next is very often the one just used. Alphabetical would put an arbitrary
// few at the top of a capped list and stalled-first would push the wanted one
// down. "Recent" needs nothing stored — it is the newest of the project's own
// creation and its newest action's.
//
// Capped at limit, and the count before capping is returned alongside so the
// screen can say what it is not showing: a cap that does not announce itself
// reads as a complete list, which is the same failure as a filtered view that
// does not say so (design.md, "Views").
func (a *App) ProjectCandidates(q string, limit int) ([]*ProjectCandidate, int, error) {
	all, err := a.ProjectList(Filters{})
	if err != nil {
		return nil, 0, err
	}
	var hits []*ProjectCandidate
	for _, p := range all {
		if !matchName(q, p.Title) {
			continue
		}
		c := &ProjectCandidate{Project: p, LastUsed: p.CreatedAt}
		for _, act := range p.Actions {
			if act.CompletedAt == nil {
				c.OpenCount++
			}
			if act.CreatedAt.After(c.LastUsed) {
				c.LastUsed = act.CreatedAt
			}
		}
		hits = append(hits, c)
	}
	total := len(hits)
	sort.SliceStable(hits, func(i, j int) bool {
		if !hits[i].LastUsed.Equal(hits[j].LastUsed) {
			return hits[i].LastUsed.After(hits[j].LastUsed)
		}
		return hits[i].Title < hits[j].Title
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits, total, nil
}

func (a *App) projectsWhere(where string, f Filters, completed bool) ([]*Project, error) {
	rows, err := a.db.Query(`SELECT id, title, dod, created_at, last_reviewed_at, snooze_until, completed_at
		FROM projects WHERE ` + where)
	if err != nil {
		return nil, err
	}
	var projects []*Project
	for rows.Next() {
		p := &Project{}
		var created, reviewed string
		var comp sql.NullString
		if err := rows.Scan(&p.ID, &p.Title, &p.DOD, &created, &reviewed, &p.SnoozeUntil, &comp); err != nil {
			rows.Close()
			return nil, err
		}
		p.CreatedAt = parseTS(created)
		p.LastReviewedAt = parseTS(reviewed)
		p.CompletedAt = parseTSPtr(comp)
		projects = append(projects, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	today := a.Today()
	var out []*Project
	for _, p := range projects {
		acts, err := a.loadActions(`a.project_id = ?`, p.ID)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(acts, func(i, j int) bool {
			ci, cj := acts[i].CompletedAt != nil, acts[j].CompletedAt != nil
			if ci != cj {
				return !ci
			}
			return acts[i].ID < acts[j].ID
		})
		p.Actions = acts
		p.Tags, err = a.tagsFor("project", p.ID)
		if err != nil {
			return nil, err
		}
		names := []string{p.Title}
		for _, act := range p.Actions {
			names = append(names, act.Title)
		}
		if !matchName(f.Name, names...) || !matchTags(f.Tags, p.Tags) {
			continue
		}
		p.Stalled = p.ComputeStalled(today)
		out = append(out, p)
	}
	if completed {
		sort.SliceStable(out, func(i, j int) bool {
			return out[i].CompletedAt.After(*out[j].CompletedAt)
		})
	}
	return out, nil
}

func (a *App) tagsFor(itemType string, id int64) ([]string, error) {
	rows, err := a.db.Query(`SELECT tag FROM item_tags WHERE item_type=? AND item_id=? ORDER BY tag`, itemType, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ArchiveEntry is one finished commitment: a project or standalone action.
type ArchiveEntry struct {
	Project     *Project  `json:"project,omitempty"`
	Action      *Action   `json:"action,omitempty"`
	CompletedAt time.Time `json:"completedAt"`
}

// Archive: the completed commitments, newest first. A completed project
// keeps the actions it was completed with; a completed project action is
// found with its project, never on its own.
func (a *App) Archive(f Filters) ([]*ArchiveEntry, error) {
	var entries []*ArchiveEntry
	projects, err := a.projectsWhere(`completed_at IS NOT NULL`, f, true)
	if err != nil {
		return nil, err
	}
	for _, p := range projects {
		if !a.matchCompleted(f.Completed, p.CompletedAt) {
			continue
		}
		entries = append(entries, &ArchiveEntry{Project: p, CompletedAt: *p.CompletedAt})
	}
	acts, err := a.loadActions(`a.project_id IS NULL AND a.completed_at IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	for _, act := range acts {
		if !matchName(f.Name, act.Title) || !matchTags(f.Tags, act.Tags) || !a.matchCompleted(f.Completed, act.CompletedAt) {
			continue
		}
		entries = append(entries, &ArchiveEntry{Action: act, CompletedAt: *act.CompletedAt})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].CompletedAt.After(entries[j].CompletedAt) })
	return entries, nil
}

// TagsInUse returns every tag currently carried by at least one item — the
// tag cloud (without #today).
func (a *App) TagsInUse() ([]string, error) {
	return a.stringList(`SELECT DISTINCT tag FROM item_tags WHERE tag != '` + TodayTag + `' ORDER BY tag`)
}
