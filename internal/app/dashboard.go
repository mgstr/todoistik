package app

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// The Dashboard's numbers: what the app counts about itself (design.md,
// "Dashboard"). Everything here is a query and nothing is stored — the same
// rule every view obeys — and almost all of it is read off the audit log,
// which is the only place that remembers a capture after the capture is gone.
//
// Every panel is a list of rows, because the screen has one shape and it is
// the row the rest of the app is made of (see research/dashboard-study.html,
// variant B). What the app hands the template is therefore always []Bar.

// Bar is one row of a panel: what it is about, how much, and how long the bar
// beside it is drawn. Value is the number as it is written out, because a row
// counting days and a row counting items are both rows and only one of them
// can be a bare integer.
type Bar struct {
	Label string
	Count int     // what the row counts; 0 where the row measures a duration
	Value string  // what is written at the row's end
	Share float64 // 0..1 of the panel's largest row — the bar's length
	Pct   int     // share of the panel's total, where a total means something
	Now   bool    // the row standing for today, or for the month we are in
}

// Flow is a speed: the last seven days one row each, then the last twelve
// months one row each. Two resolutions and not one, because the week answers
// "what is happening" and the year answers "is this normal" — and a year of
// weeks would be fifty-two rows, which is a list nobody reads.
type Flow struct {
	Days       []Bar
	Months     []Bar
	WeekTotal  int
	WeekPerDay float64
	YearPerDay float64
}

// Dist is a distribution: a ranked list of names with a count each.
type Dist struct {
	Rows  []Bar
	Total int
}

// Oldest is one row of "the oldest thing in each view" — the view, and the one
// item in it that has waited longest. No URL: internal/app has no knowledge of
// HTTP, so the template builds the link out of Type and ID.
type Oldest struct {
	View  string
	Type  string
	ID    int64
	Title string
	At    time.Time
}

// Practice is how the discipline itself is going, as far as the record can
// honestly say. There is no global "last review" record anywhere in the app
// (see review.go), so this cannot say when a review *finished* — only when
// something was last marked reviewed, which is when one was last being done.
type Practice struct {
	InboxOpen         int
	InboxEmptyAt      time.Time // the last moment the inbox held nothing
	InboxEverEmpty    bool
	LastReviewedAt    time.Time // the newest `reviewed` entry; zero if never
	ReviewOutstanding int
}

// Ages holds the two durations the audit log can measure exactly, and the age
// of what is open right now. All three are medians: a mean over ages is
// dragged by the one item that has sat for a year, and that item is the one
// you already know about (design.md, "Dashboard").
type Ages struct {
	InInbox       time.Duration
	InInboxN      int
	ToCompletion  time.Duration
	ToCompletionN int
	Open          []Bar
}

type Dashboard struct {
	Inbound  Flow
	Outbound Flow
	Became   Dist
	Sources  Dist
	Ages     Ages
	Practice Practice
	Oldest   []Oldest
	NextMix  Dist
	Backlog  []Bar
}

// Dashboard reads the lot. It is deliberately one call: every panel is a
// different question about the same record, and a screen that assembled itself
// from nine separately-timed reads could show two panels that disagree.
func (a *App) Dashboard() (*Dashboard, error) {
	d := &Dashboard{}
	yearFrom := a.monthStart(11)

	captures, err := a.auditAts(`SELECT at FROM audit_log
		WHERE item_type='inbox' AND event=? AND at >= ?`, EvCreated, ts(yearFrom))
	if err != nil {
		return nil, err
	}
	d.Inbound = a.flow(captures)

	// Every completion, a project's own and each step inside it. The Archive
	// counts finished *commitments* and deliberately leaves a project's steps
	// off (design.md, "Archive"); this counts work done, which is a different
	// question — a project finished in five actions was five things done, and
	// a speed that called it one would say the quiet weeks and the busy ones
	// were the same week.
	done, err := a.auditAts(`SELECT at FROM audit_log
		WHERE item_type IN ('action','project') AND event=? AND at >= ?`, EvCompleted, ts(yearFrom))
	if err != nil {
		return nil, err
	}
	d.Outbound = a.flow(done)

	if d.Became, err = a.becameDist(a.monthStart(2)); err != nil {
		return nil, err
	}
	if d.Sources, err = a.sourceDist(a.monthStart(2)); err != nil {
		return nil, err
	}
	if d.Ages, err = a.ages(a.monthStart(2)); err != nil {
		return nil, err
	}
	if d.Practice, err = a.practice(); err != nil {
		return nil, err
	}
	if d.Oldest, err = a.oldestPerView(); err != nil {
		return nil, err
	}
	if d.NextMix, err = a.nextMix(); err != nil {
		return nil, err
	}
	if d.Backlog, err = a.backlog(); err != nil {
		return nil, err
	}
	return d, nil
}

// --- the two speeds ------------------------------------------------------

func (a *App) flow(times []time.Time) Flow {
	f := Flow{}
	now := a.now().In(a.loc)

	// the week: one row per day, today first and marked, then backwards. The
	// row you came to read is the one at the top, and the eye does not have to
	// find the end of a list to find now (implementation.md, "The Dashboard").
	perDay := map[string]int{}
	for _, t := range times {
		perDay[t.In(a.loc).Format(DateFormat)]++
	}
	for i := 0; i <= 6; i++ {
		day := now.AddDate(0, 0, -i)
		key := day.Format(DateFormat)
		n := perDay[key]
		f.WeekTotal += n
		f.Days = append(f.Days, Bar{
			Label: day.Format("Mon 2 Jan"),
			Count: n,
			Value: fmt.Sprintf("%d", n),
			Now:   i == 0,
		})
	}
	f.WeekPerDay = float64(f.WeekTotal) / 7

	// the year: one row per month, this month first, the bar being that month's
	// average per day so that a 28-day month is not drawn short for being short
	perMonth := map[string]int{}
	for _, t := range times {
		perMonth[t.In(a.loc).Format("2006-01")]++
	}
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, a.loc)
	rates := make([]float64, 0, 12)
	total := 0
	for i := 0; i <= 11; i++ {
		m := first.AddDate(0, -i, 0)
		n := perMonth[m.Format("2006-01")]
		total += n
		rate := float64(n) / float64(daysIn(m, a.loc, now))
		rates = append(rates, rate)
		f.Months = append(f.Months, Bar{
			Label: m.Format("Jan 2006"),
			Count: n,
			Value: fmt.Sprintf("%.1f a day", rate),
			Now:   i == 0,
		})
	}
	scaleTo(f.Days, func(b Bar) float64 { return float64(b.Count) })
	maxRate := 0.0
	for _, r := range rates {
		if r > maxRate {
			maxRate = r
		}
	}
	for i := range f.Months {
		if maxRate > 0 {
			f.Months[i].Share = rates[i] / maxRate
		}
	}
	// the year's rate counts only the days that have actually happened, so a
	// month three days old does not drag the average down with 28 days of zero
	if days := a.daysSince(first.AddDate(0, -11, 0)); days > 0 {
		f.YearPerDay = float64(total) / float64(days)
	}
	return f
}

// daysIn is how many days of that month have happened: the whole month for
// every month but the one we are standing in.
func daysIn(m time.Time, loc *time.Location, now time.Time) int {
	if m.Year() == now.Year() && m.Month() == now.Month() {
		return now.Day()
	}
	return time.Date(m.Year(), m.Month()+1, 1, 0, 0, 0, 0, loc).AddDate(0, 0, -1).Day()
}

func (a *App) daysSince(t time.Time) int {
	return int(a.now().In(a.loc).Sub(t).Hours()/24) + 1
}

func (a *App) monthStart(back int) time.Time {
	now := a.now().In(a.loc)
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, a.loc).AddDate(0, -back, 0)
}

// --- the two distributions -----------------------------------------------

// becameLabels turns a leaving event into what the row says, and fixes the
// order the rows are drawn in: what became a commitment first, then what
// became nothing, which is the panel read from most answered-with-work to
// least. A fixed order and not a ranking, unlike the channels below — the
// branches are a closed set with a natural order, and a panel that reshuffled
// itself between visits would have to be re-read each time rather than
// recognised.
//
// `deleted` is in here because of what the log used to be: before every branch
// wrote its own event, the ones that create something shared that word, so an
// old log has one bucket where a new one has four (design.md, "Audit entry").
// It is shown under its own name rather than folded into any of them — a
// bucket that silently joined "a task" would be the same lie in a new place.
var becameLabels = []struct{ Event, Label string }{
	{EvBecameTask, "a task"},
	{EvBecameAction, "an action in a project"},
	{EvBecameProject, "a project"},
	{EvBecameSomeday, "someday/maybe"},
	{EvTwoMinute, "done on the spot"},
	{EvTrashed, "trashed"},
	{EvReference, "reference material"},
	{EvConfirmed, "a completion confirmed"},
	{EvDeleted, "filed, before the log said which"},
}

func (a *App) becameDist(since time.Time) (Dist, error) {
	counts, err := a.countBy(`SELECT event, COUNT(*) FROM audit_log
		WHERE item_type='inbox' AND event<>? AND at >= ? GROUP BY event`, EvCreated, ts(since))
	if err != nil {
		return Dist{}, err
	}
	d := Dist{}
	for _, l := range becameLabels {
		n := counts[l.Event]
		if n == 0 {
			continue
		}
		d.Total += n
		d.Rows = append(d.Rows, Bar{Label: l.Label, Count: n, Value: fmt.Sprintf("%d", n)})
	}
	finish(&d)
	return d, nil
}

// sourceDist counts the channel every capture arrived by, read out of the
// snapshot rather than off the items: by the time a capture is anything it has
// stopped being a capture, and the audit log is where the year of them is
// (design.md, "Where it came from").
func (a *App) sourceDist(since time.Time) (Dist, error) {
	counts, err := a.countBy(`SELECT COALESCE(json_extract(snapshot,'$.source'),''), COUNT(*)
		FROM audit_log WHERE item_type='inbox' AND event=? AND at >= ?
		GROUP BY 1`, EvCreated, ts(since))
	if err != nil {
		return Dist{}, err
	}
	d := Dist{}
	for name, n := range counts {
		label := name
		if label == "" {
			label = "unnamed"
		}
		d.Total += n
		d.Rows = append(d.Rows, Bar{Label: label, Count: n, Value: fmt.Sprintf("%d", n)})
	}
	sort.Slice(d.Rows, func(i, j int) bool {
		if d.Rows[i].Count != d.Rows[j].Count {
			return d.Rows[i].Count > d.Rows[j].Count
		}
		return d.Rows[i].Label < d.Rows[j].Label
	})
	finish(&d)
	return d, nil
}

// --- ages ----------------------------------------------------------------

func (a *App) ages(since time.Time) (Ages, error) {
	var ag Ages

	inbox, err := a.durations("inbox", since)
	if err != nil {
		return ag, err
	}
	ag.InInbox, ag.InInboxN = median(inbox), len(inbox)

	var work []time.Duration
	for _, t := range []string{"action", "project"} {
		ds, err := a.completionDurations(t, since)
		if err != nil {
			return ag, err
		}
		work = append(work, ds...)
	}
	ag.ToCompletion, ag.ToCompletionN = median(work), len(work)

	if ag.Open, err = a.openAges(); err != nil {
		return ag, err
	}
	return ag, nil
}

// durations pairs every item of that type that left with the `created` entry
// for the same id immediately before it. Ids are handed back out after a
// delete, so it has to be the *nearest preceding* one — an id is live at most
// once at a time, which is what makes that exact.
func (a *App) durations(itemType string, since time.Time) ([]time.Duration, error) {
	return a.durationsQuery(`SELECT l.at, (SELECT MAX(c.at) FROM audit_log c
			WHERE c.item_type=l.item_type AND c.item_id=l.item_id
			  AND c.event=? AND c.at <= l.at)
		FROM audit_log l
		WHERE l.item_type=? AND l.event<>? AND l.at >= ?`,
		EvCreated, itemType, EvCreated, ts(since))
}

func (a *App) completionDurations(itemType string, since time.Time) ([]time.Duration, error) {
	return a.durationsQuery(`SELECT l.at, (SELECT MAX(c.at) FROM audit_log c
			WHERE c.item_type=l.item_type AND c.item_id=l.item_id
			  AND c.event=? AND c.at <= l.at)
		FROM audit_log l
		WHERE l.item_type=? AND l.event=? AND l.at >= ?`,
		EvCreated, itemType, EvCompleted, ts(since))
}

func (a *App) openAges() ([]Bar, error) {
	inbox, err := a.Inbox()
	if err != nil {
		return nil, err
	}
	next, err := a.NextActions(Filters{})
	if err != nil {
		return nil, err
	}
	waiting, err := a.WaitingFor(Filters{})
	if err != nil {
		return nil, err
	}
	someday, err := a.SomedayItems(Filters{})
	if err != nil {
		return nil, err
	}
	projects, err := a.ProjectList(Filters{})
	if err != nil {
		return nil, err
	}
	now := a.now().UTC()
	age := func(t time.Time) time.Duration { return now.Sub(t.UTC()) }

	var inboxD, nextD, waitD, somedayD, projD []time.Duration
	for _, it := range inbox {
		inboxD = append(inboxD, age(it.CreatedAt))
	}
	for _, act := range next {
		nextD = append(nextD, age(act.CreatedAt))
	}
	// a delegation's age is the delegation date and not the creation date,
	// which is the rule that view is already read by (design.md, "Waiting for")
	for _, act := range waiting {
		at := act.CreatedAt
		if act.BecameNextAt != nil {
			at = *act.BecameNextAt
		}
		waitD = append(waitD, age(at))
	}
	for _, it := range someday {
		somedayD = append(somedayD, age(it.CreatedAt))
	}
	for _, p := range projects {
		projD = append(projD, age(p.CreatedAt))
	}

	rows := []Bar{
		durationBar("inbox", inboxD),
		durationBar("next", nextD),
		durationBar("waiting", waitD),
		durationBar("projects", projD),
		durationBar("someday", somedayD),
	}
	max := time.Duration(0)
	for _, d := range [][]time.Duration{inboxD, nextD, waitD, projD, somedayD} {
		if m := median(d); m > max {
			max = m
		}
	}
	all := [][]time.Duration{inboxD, nextD, waitD, projD, somedayD}
	for i := range rows {
		if max > 0 {
			rows[i].Share = float64(median(all[i])) / float64(max)
		}
	}
	return rows, nil
}

func durationBar(label string, ds []time.Duration) Bar {
	return Bar{Label: label, Count: len(ds), Value: HumanDuration(median(ds))}
}

// --- the practice --------------------------------------------------------

func (a *App) practice() (Practice, error) {
	var p Practice
	inbox, err := a.Inbox()
	if err != nil {
		return p, err
	}
	p.InboxOpen = len(inbox)

	// When the inbox last held nothing, replayed from the log: every capture
	// puts one in and every answer takes one out, so the running count is the
	// inbox as it was at each moment. Nothing records emptiness directly, and
	// nothing should — it is not a thing that happens, it is a thing that is
	// true in between two things that happen.
	rows, err := a.db.Query(`SELECT at, event FROM audit_log
		WHERE item_type='inbox' AND event<>? ORDER BY at, id`, EvEdited)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	open := 0
	for rows.Next() {
		var at, event string
		if err := rows.Scan(&at, &event); err != nil {
			return p, err
		}
		if event == EvCreated {
			open++
			continue
		}
		open--
		if open <= 0 {
			open = 0
			p.InboxEmptyAt, p.InboxEverEmpty = parseTS(at), true
		}
	}
	if err := rows.Err(); err != nil {
		return p, err
	}
	if p.InboxOpen == 0 {
		p.InboxEmptyAt, p.InboxEverEmpty = a.now().UTC(), true
	}

	var last string
	err = a.db.QueryRow(`SELECT MAX(at) FROM audit_log WHERE event=?`, EvReviewed).Scan(&last)
	if err == nil && last != "" {
		p.LastReviewedAt = parseTS(last)
	}

	rc, err := a.ReviewCounts()
	if err != nil {
		return p, err
	}
	p.ReviewOutstanding = rc.OutstandingTotal()
	return p, nil
}

// --- the oldest thing in each view ---------------------------------------

func (a *App) oldestPerView() ([]Oldest, error) {
	var out []Oldest
	add := func(view, itemType string, id int64, title string, at time.Time) {
		if id == 0 {
			return
		}
		out = append(out, Oldest{View: view, Type: itemType, ID: id, Title: title, At: at})
	}

	inbox, err := a.Inbox()
	if err != nil {
		return nil, err
	}
	if len(inbox) > 0 {
		o := inbox[0]
		for _, it := range inbox {
			if it.CreatedAt.Before(o.CreatedAt) {
				o = it
			}
		}
		add("inbox", "inbox", o.ID, o.Line(), o.CreatedAt)
	}

	next, err := a.NextActions(Filters{})
	if err != nil {
		return nil, err
	}
	if o := oldestAction(next, false); o != nil {
		add("next", "action", o.ID, o.Title, o.CreatedAt)
	}

	waiting, err := a.WaitingFor(Filters{})
	if err != nil {
		return nil, err
	}
	if o := oldestAction(waiting, true); o != nil {
		at := o.CreatedAt
		if o.BecameNextAt != nil {
			at = *o.BecameNextAt
		}
		add("waiting", "action", o.ID, o.Title, at)
	}

	projects, err := a.ProjectList(Filters{})
	if err != nil {
		return nil, err
	}
	if len(projects) > 0 {
		o := projects[0]
		for _, p := range projects {
			if p.CreatedAt.Before(o.CreatedAt) {
				o = p
			}
		}
		add("projects", "project", o.ID, o.Title, o.CreatedAt)
	}

	someday, err := a.SomedayItems(Filters{})
	if err != nil {
		return nil, err
	}
	if len(someday) > 0 {
		o := someday[0]
		for _, it := range someday {
			if it.CreatedAt.Before(o.CreatedAt) {
				o = it
			}
		}
		add("someday", "someday", o.ID, o.Text, o.CreatedAt)
	}
	return out, nil
}

func oldestAction(acts []*Action, byDelegation bool) *Action {
	var o *Action
	when := func(act *Action) time.Time {
		if byDelegation && act.BecameNextAt != nil {
			return *act.BecameNextAt
		}
		return act.CreatedAt
	}
	for _, act := range acts {
		if o == nil || when(act).Before(when(o)) {
			o = act
		}
	}
	return o
}

// --- how much of Next is workable ----------------------------------------

// Every open action — the pool the "Next actions" view is the workable part of.
// Five states, and the view shows one of them; if the other four come to
// outweigh it, "what can I do now" has quietly stopped being a list you can act
// from.
//
// "Further down a plan" is the one of the five that is nobody's fault and still
// worth counting: an action a project holds behind its next one is invisible to
// every view that is worked from, by design, and this is the figure that says
// how much is in that state. The order of the cases is what makes the first row
// exactly the view's own count — a delegated or snoozed action is reported as
// that whether or not it is also at the front of its plan.
func (a *App) nextMix() (Dist, error) {
	acts, err := a.loadActions(`a.completed_at IS NULL`)
	if err != nil {
		return Dist{}, err
	}
	next, err := a.nextActionIDs()
	if err != nil {
		return Dist{}, err
	}
	today := a.Today()
	var workable, delegated, snoozed, blocked, behind int
	for _, act := range acts {
		switch {
		case act.AssignedTo != "":
			delegated++
		case act.SnoozeActionID != 0:
			blocked++
		case act.SnoozeUntil != "" && act.SnoozeUntil > today:
			snoozed++
		case act.ProjectID != 0 && next[act.ProjectID] != act.ID:
			behind++
		default:
			workable++
		}
	}
	d := Dist{Total: len(acts)}
	for _, r := range []struct {
		label string
		n     int
	}{
		{"workable now", workable},
		{"waiting on someone", delegated},
		{"snoozed until a date", snoozed},
		{"waiting on a sibling", blocked},
		{"further down a plan", behind},
	} {
		d.Rows = append(d.Rows, Bar{Label: r.label, Count: r.n, Value: fmt.Sprintf("%d", r.n)})
	}
	finish(&d)
	return d, nil
}

// --- the backlog ---------------------------------------------------------

// Open commitments at the end of each of the last twelve months, this month
// first like every other dated panel, counted off the items themselves rather
// than replayed from the log. That makes it "what
// I still have, seen month by month" rather than a reconstruction: an item
// deleted since is missing from the months it was open in, because it is no
// longer there to count. The line is read for its direction, and a deletion
// moves that the same way finishing it would.
func (a *App) backlog() ([]Bar, error) {
	type span struct{ created, completed time.Time }
	var spans []span

	for _, q := range []string{
		`SELECT created_at, completed_at FROM actions`,
		`SELECT created_at, completed_at FROM projects`,
	} {
		rows, err := a.db.Query(q)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var created string
			var completed sql.NullString
			if err := rows.Scan(&created, &completed); err != nil {
				rows.Close()
				return nil, err
			}
			s := span{created: parseTS(created)}
			if completed.Valid && completed.String != "" {
				s.completed = parseTS(completed.String)
			}
			spans = append(spans, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	now := a.now().In(a.loc)
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, a.loc)
	var bars []Bar
	counts := make([]int, 0, 12)
	for i := 0; i <= 11; i++ {
		m := first.AddDate(0, -i, 0)
		end := m.AddDate(0, 1, 0) // the moment the month is over
		if i == 0 {
			end = a.now().UTC() // the month we are in is counted as it stands
		}
		n := 0
		for _, s := range spans {
			if s.created.After(end) {
				continue
			}
			if !s.completed.IsZero() && !s.completed.After(end) {
				continue
			}
			n++
		}
		counts = append(counts, n)
		bars = append(bars, Bar{
			Label: m.Format("Jan 2006"),
			Count: n,
			Value: fmt.Sprintf("%d", n),
			Now:   i == 0,
		})
	}
	scaleTo(bars, func(b Bar) float64 { return float64(b.Count) })
	return bars, nil
}

// --- the three shapes of query the panels need ---------------------------

// auditAts reads a column of timestamps. Every speed panel is a count of
// moments, so the row that carries one is a moment and nothing else — the
// bucketing into days and months happens in Go, against the app's own
// timezone, which is the only zone that may decide what day something was on
// (see Loc).
func (a *App) auditAts(q string, args ...any) ([]time.Time, error) {
	rows, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var at string
		if err := rows.Scan(&at); err != nil {
			return nil, err
		}
		out = append(out, parseTS(at))
	}
	return out, rows.Err()
}

// durationsQuery reads "the moment it ended, the moment it began" pairs and
// returns the spans between them. A row whose beginning is missing is dropped
// rather than counted as zero: an item created before the log was keeping that
// event has no measurable life, and guessing one would quietly pull every
// median towards nothing.
func (a *App) durationsQuery(q string, args ...any) ([]time.Duration, error) {
	rows, err := a.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Duration
	for rows.Next() {
		var end string
		var start sql.NullString
		if err := rows.Scan(&end, &start); err != nil {
			return nil, err
		}
		if !start.Valid || start.String == "" {
			continue
		}
		if d := parseTS(end).Sub(parseTS(start.String)); d >= 0 {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

// --- small helpers -------------------------------------------------------

func scaleTo(bars []Bar, of func(Bar) float64) {
	max := 0.0
	for _, b := range bars {
		if v := of(b); v > max {
			max = v
		}
	}
	for i := range bars {
		if max > 0 {
			bars[i].Share = of(bars[i]) / max
		}
	}
}

// finish gives a distribution its bar lengths and its percentages: the bar is
// drawn against the biggest row so the shape is readable, and the percentage
// is of the whole so the number is true.
func finish(d *Dist) {
	scaleTo(d.Rows, func(b Bar) float64 { return float64(b.Count) })
	for i := range d.Rows {
		if d.Total > 0 {
			d.Rows[i].Pct = int(float64(d.Rows[i].Count)/float64(d.Total)*100 + 0.5)
		}
	}
}

func median(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), ds...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// HumanDuration writes a span the way the ages are written: words, and one
// unit. It is the duration counterpart of the age vocabulary in
// internal/web (see research/item-line-study.html), and it lives here because
// what it words is a number this package computed.
func HumanDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "—"
	case d < time.Minute:
		// not "0 min": a span that rounds to nothing is still a span, and a
		// zero beside a row that plainly has something in it reads as a bug
		return "under a minute"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute")
	case d < 36*time.Hour:
		return plural(int(d.Hours()+0.5), "hour")
	case d < 14*24*time.Hour:
		return plural(int(d.Hours()/24+0.5), "day")
	case d < 60*24*time.Hour:
		return plural(int(d.Hours()/24/7+0.5), "week")
	case d < 365*24*time.Hour:
		return plural(int(d.Hours()/24/30+0.5), "month")
	default:
		return plural(int(d.Hours()/24/365+0.5), "year")
	}
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
