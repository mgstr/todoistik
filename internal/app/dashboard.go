package app

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// The Dashboard's numbers: what the app counts about itself (design.md,
// "Dashboard"). Everything here is a query and nothing is stored — the same
// rule every view obeys — and most of it is read off the audit log, which is
// the only place that remembers a capture after the capture is gone.
//
// The screen is three forms and each is one read: Traffic and Queue are the
// same shape, dates down the middle and a bar either side of each, and Inbox
// zero is a year of days. A form is asked for on its own, because only one is
// ever on the screen (implementation.md, "The Dashboard").

// Seg is one segment of a bar: the series it belongs to, the figure written
// out, and how long it is drawn. Weight is in thousandths of the form's
// longest bar and is an integer on purpose — it is written into a style
// attribute, and a whole number is the one thing html/template never refuses.
type Seg struct {
	Name   string
	Slot   int // which of the eight series colours, 1…8
	Text   string
	Weight int
}

// Side is one half of a row. Known is false for a period the record does not
// reach back to: that is a row with nothing written on it, which is a
// different statement from a row that says zero.
type Side struct {
	Segs  []Seg
	Total string
	Rest  int // thousandths of the track the bar leaves empty
	Known bool
}

// Period is one row of a mirrored form: a day, a month or the year.
type Period struct {
	Label string
	Now   bool // today, the month we are in, the year we are in
	Gap   bool // the first row of a band, drawn a little apart from the last
	Left  Side
	Right Side
}

// Series names a colour: what the legend says beside the swatch.
type Series struct {
	Name string
	Slot int
}

// Half is what is written above one side of a mirrored form — its name, what
// it counts, the headline figure and the legend.
type Half struct {
	Title  string
	Window string
	Big    string
	Unit   string
	Versus string
	Series []Series
}

// Mirror is a whole form of that shape: seven days from today, twelve months
// from this one, then the year. Newest first, like every dated list in the
// app (design.md, "Dashboard").
type Mirror struct {
	Left  Half
	Right Half
	Rows  []Period
}

// dayVals is what one day contributes to each side, a number per series.
type dayVals struct{ l, r []float64 }

// span is one row's period before it has been counted.
type span struct {
	label    string
	from, to time.Time // local midnights, both inclusive
	band     byte      // 'd', 'm' or 'y'
	now, gap bool
}

// spans lays out the rows every mirrored form has. Two resolutions and a
// total, because they answer different questions: the week says what is
// happening, the months say whether that is normal, and the year is the
// figure both are read against.
func (a *App) spans() []span {
	today := a.midnight(a.now())
	var out []span
	for i := 0; i <= 6; i++ {
		d := today.AddDate(0, 0, -i)
		out = append(out, span{label: d.Format("Mon 2 Jan"), from: d, to: d, band: 'd', now: i == 0})
	}
	first := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, a.loc)
	for i := 0; i <= 11; i++ {
		m := first.AddDate(0, -i, 0)
		out = append(out, span{label: m.Format("Jan 2006"), from: m, to: m.AddDate(0, 1, -1),
			band: 'm', now: i == 0, gap: i == 0})
	}
	out = append(out, span{label: today.Format("2006"), from: time.Date(today.Year(), 1, 1, 0, 0, 0, 0, a.loc),
		to: today, band: 'y', now: true, gap: true})
	return out
}

func (a *App) midnight(t time.Time) time.Time {
	t = t.In(a.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, a.loc)
}

// recordStart is the day the audit log begins on — the first day anything on
// this screen can be said about. An empty log begins today.
func (a *App) recordStart() (time.Time, error) {
	var first sql.NullString
	if err := a.db.QueryRow(`SELECT MIN(at) FROM audit_log`).Scan(&first); err != nil {
		return time.Time{}, err
	}
	if !first.Valid || first.String == "" {
		return a.midnight(a.now()), nil
	}
	return a.midnight(parseTS(first.String)), nil
}

// mirror counts every row of a form out of one number per series per day. A
// row is the mean of its days — which for a count of events is the rate a day
// and for a level is the average level, so the two forms are one sum. Only
// days the record covers and that have happened are in the mean: a month
// three days old is not dragged down by twenty-seven days of nothing, and the
// month the record began in is not either.
//
// Both sides and every row are drawn against one scale. The rows are all in
// the same unit, so a day can be laid against a month and the left against
// the right without the eye having to convert.
func (a *App) mirror(start time.Time, day func(time.Time) dayVals, ls, rs []Series,
	text func(band byte, v float64) string) ([]Period, []float64, []float64) {
	today := a.midnight(a.now())
	type counted struct {
		l, r  []float64
		known bool
	}
	spans := a.spans()
	rows := make([]counted, len(spans))
	max := 0.0
	for i, sp := range spans {
		from, to := sp.from, sp.to
		if from.Before(start) {
			from = start
		}
		if to.After(today) {
			to = today
		}
		l, r, n := make([]float64, len(ls)), make([]float64, len(rs)), 0
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			v := day(d)
			for j := range v.l {
				l[j] += v.l[j]
			}
			for j := range v.r {
				r[j] += v.r[j]
			}
			n++
		}
		if n == 0 {
			continue
		}
		for j := range l {
			l[j] /= float64(n)
		}
		for j := range r {
			r[j] /= float64(n)
		}
		rows[i] = counted{l, r, true}
		if t := sum(l); t > max {
			max = t
		}
		if t := sum(r); t > max {
			max = t
		}
	}

	side := func(vals []float64, series []Series, band byte, known bool) Side {
		s := Side{Rest: 1000, Known: known}
		if !known {
			return s
		}
		s.Total = text(band, sum(vals))
		for j, v := range vals {
			if v <= 0 || max <= 0 {
				continue
			}
			w := int(v/max*1000 + 0.5)
			s.Segs = append(s.Segs, Seg{Name: series[j].Name, Slot: series[j].Slot, Text: text(band, v), Weight: w})
			s.Rest -= w
		}
		if s.Rest < 0 {
			s.Rest = 0
		}
		return s
	}
	out := make([]Period, len(spans))
	var yearL, yearR []float64
	for i, sp := range spans {
		c := rows[i]
		out[i] = Period{Label: sp.label, Now: sp.now, Gap: sp.gap,
			Left: side(c.l, ls, sp.band, c.known), Right: side(c.r, rs, sp.band, c.known)}
		if sp.band == 'y' && c.known {
			yearL, yearR = c.l, c.r
		}
	}
	return out, yearL, yearR
}

func sum(vs []float64) float64 {
	t := 0.0
	for _, v := range vs {
		t += v
	}
	return t
}

// --- Traffic ---------------------------------------------------------------

// What is drawn on the outgoing side, nearest the dates first. A fixed order
// and fixed colours: the ways of leaving are a closed set, and a legend that
// reshuffled between visits would have to be read again each time rather than
// recognised.
const (
	outSpot = iota
	outDone
	outTrashed
	outDeleted
)

var outSeries = []Series{
	{"done on the spot", 5}, {"done", 6}, {"trashed", 7}, {"deleted", 8},
}

// maxSources is how many channels get a colour of their own. Past it the
// smallest are one segment called "other": a fifth and sixth hue on a bar this
// thin stop being told apart, and the channels that matter are the big ones.
const maxSources = 4

// Traffic is what came in against what left, a day a row.
//
// Incoming is every capture, split by the channel it arrived by — read out of
// the audit log's snapshots, because by the time a capture is anything it has
// stopped being a capture (design.md, "Where it came from").
//
// Outgoing is everything that left the app for good: an action or a project
// finished, a capture answered by the two minute rule, a capture trashed, and
// an action, project, idea or reference item deleted. It counts steps and not
// commitments — a project finished in five actions was five things done.
//
// The two sides are different populations, and the screen puts them side by
// side anyway: one capture can become a project of five actions and another
// becomes nothing at all, so the difference between the bars is not the size
// of anything (design.md, "Dashboard").
func (a *App) Traffic() (*Mirror, error) {
	start, err := a.recordStart()
	if err != nil {
		return nil, err
	}
	// the twelve months reach at least as far back as the first of January,
	// so one window covers the year's row as well
	since := ts(a.monthStart(11))

	type capture struct {
		day, source string
	}
	var captures []capture
	bySource := map[string]int{}
	rows, err := a.db.Query(`SELECT at, COALESCE(json_extract(snapshot,'$.source'),'')
		FROM audit_log WHERE item_type='inbox' AND event=? AND at >= ?`, EvCreated, since)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var at, source string
		if err := rows.Scan(&at, &source); err != nil {
			rows.Close()
			return nil, err
		}
		if source == "" {
			source = "unnamed"
		}
		captures = append(captures, capture{a.dayKey(parseTS(at)), source})
		bySource[source]++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Which channels are drawn: the largest over the window, and the rest as
	// one. The colours then go out in alphabetical order rather than by size,
	// so two channels trading places in the ranking do not trade colours.
	names := make([]string, 0, len(bySource))
	for n := range bySource {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if bySource[names[i]] != bySource[names[j]] {
			return bySource[names[i]] > bySource[names[j]]
		}
		return names[i] < names[j]
	})
	other := false
	if len(names) > maxSources {
		names, other = names[:maxSources-1], true
	}
	sort.Strings(names)
	var in []Series
	index := map[string]int{}
	for i, n := range names {
		in = append(in, Series{n, i + 1})
		index[n] = i
	}
	if other {
		in = append(in, Series{"other", maxSources})
	}
	if len(in) == 0 {
		// nothing has ever been captured; the legend still names the channel
		// the next capture will most likely come by
		in = []Series{{SourceApp, 1}}
	}

	perDay := map[string]*dayVals{}
	at := func(key string) *dayVals {
		v := perDay[key]
		if v == nil {
			v = &dayVals{make([]float64, len(in)), make([]float64, len(outSeries))}
			perDay[key] = v
		}
		return v
	}
	for _, c := range captures {
		i, ok := index[c.source]
		if !ok {
			i = len(in) - 1 // "other"
		}
		at(c.day).l[i]++
	}

	rows, err = a.db.Query(`SELECT at, event, item_type FROM audit_log WHERE at >= ? AND (
			(item_type IN ('action','project') AND event=?)
			OR (item_type='inbox' AND event IN (?,?))
			OR (item_type IN ('action','project','someday','reference') AND event=?))`,
		since, EvCompleted, EvTwoMinute, EvTrashed, EvDeleted)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var when, event, itemType string
		if err := rows.Scan(&when, &event, &itemType); err != nil {
			rows.Close()
			return nil, err
		}
		v := at(a.dayKey(parseTS(when)))
		switch event {
		case EvCompleted:
			v.r[outDone]++
		case EvTwoMinute:
			v.r[outSpot]++
		case EvTrashed:
			v.r[outTrashed]++
		case EvDeleted:
			v.r[outDeleted]++
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	day := func(d time.Time) dayVals {
		if v := perDay[d.Format(DateFormat)]; v != nil {
			return *v
		}
		return dayVals{}
	}
	// a day's row is a count and is written as one; a month and the year are
	// rates, and a rate rounded to a whole number would call 0.4 a day nothing
	text := func(band byte, v float64) string {
		if band == 'd' {
			return fmt.Sprintf("%.0f", v)
		}
		return fmt.Sprintf("%.1f", v)
	}
	m := &Mirror{
		Left:  Half{Title: "Incoming", Window: "captures, by where they came from", Series: in},
		Right: Half{Title: "Outgoing", Window: "left the app, by how", Series: outSeries},
	}
	var yearL, yearR []float64
	m.Rows, yearL, yearR = a.mirror(start, day, in, outSeries, text)

	// the headline is the last seven days, and says so: not the calendar
	// week, which on a Monday would be a figure about one day
	today := a.midnight(a.now())
	weekL, weekR := 0.0, 0.0
	for i := 0; i <= 6; i++ {
		v := day(today.AddDate(0, 0, -i))
		weekL += sum(v.l)
		weekR += sum(v.r)
	}
	figure := func(h *Half, week float64, year []float64) {
		h.Big, h.Unit = fmt.Sprintf("%.0f", week), "last 7 days"
		h.Versus = fmt.Sprintf("%.1f a day", week/7)
		if year != nil {
			h.Versus += fmt.Sprintf(", %.1f over the year", sum(year))
		}
	}
	figure(&m.Left, weekL, yearL)
	figure(&m.Right, weekR, yearR)
	return m, nil
}

func (a *App) dayKey(t time.Time) string { return t.In(a.loc).Format(DateFormat) }

func (a *App) monthStart(back int) time.Time {
	now := a.now().In(a.loc)
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, a.loc).AddDate(0, -back, 0)
}

// --- Queue -----------------------------------------------------------------

var (
	queueSeries = []Series{{"tasks", 1}, {"next actions", 2}}
	keptSeries  = []Series{{"projects", 3}, {"someday", 4}, {"reference", 5}}
)

// Queue is how much was open, a day a row: on the right what is waiting to be
// done — every standalone task, and one next action for each project that has
// one — and on the left what is kept behind it, the projects themselves, the
// someday pile and the reference material.
//
// A day is counted as it stood at its end, and today as it stands now; a month
// and the year are the average of their days.
//
// It is counted off the items that still exist rather than replayed from the
// log, so it is "what I still have, seen day by day": something deleted since
// is missing from the days it was open on, and an action moved into a project
// since is counted where it is now. The rows are read for their direction, and
// neither moves that.
func (a *App) Queue() (*Mirror, error) {
	start, err := a.recordStart()
	if err != nil {
		return nil, err
	}
	type life struct {
		created, completed time.Time // completed is zero while it is open
		id, project        int64
	}
	load := func(q string) ([]life, error) {
		rows, err := a.db.Query(q)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []life
		for rows.Next() {
			var l life
			var created string
			var completed sql.NullString
			if err := rows.Scan(&l.id, &l.project, &created, &completed); err != nil {
				return nil, err
			}
			l.created = parseTS(created)
			if completed.Valid && completed.String != "" {
				l.completed = parseTS(completed.String)
			}
			out = append(out, l)
		}
		return out, rows.Err()
	}
	actions, err := load(`SELECT id, COALESCE(project_id,0), created_at, completed_at FROM actions`)
	if err != nil {
		return nil, err
	}
	projects, err := load(`SELECT id, 0, created_at, completed_at FROM projects`)
	if err != nil {
		return nil, err
	}
	someday, err := load(`SELECT id, 0, created_at, NULL FROM someday_items`)
	if err != nil {
		return nil, err
	}
	reference, err := load(`SELECT id, 0, created_at, NULL FROM reference_items`)
	if err != nil {
		return nil, err
	}

	now := a.now()
	today := a.midnight(now)
	open := func(l life, at time.Time) bool {
		return !l.created.After(at) && (l.completed.IsZero() || l.completed.After(at))
	}
	count := func(ls []life, at time.Time) float64 {
		n := 0
		for _, l := range ls {
			if open(l, at) {
				n++
			}
		}
		return float64(n)
	}
	day := func(d time.Time) dayVals {
		at := d.AddDate(0, 0, 1) // the moment the day is over
		if !d.Before(today) {
			at = now
		}
		live := map[int64]bool{}
		for _, p := range projects {
			if open(p, at) {
				live[p.id] = true
			}
		}
		tasks := 0
		next := map[int64]bool{}
		for _, act := range actions {
			if !open(act, at) {
				continue
			}
			if act.project == 0 {
				tasks++
			} else if live[act.project] {
				// a project has one next action however long its plan is
				// (design.md, "Project"), so it is counted once
				next[act.project] = true
			}
		}
		return dayVals{
			l: []float64{float64(len(live)), count(someday, at), count(reference, at)},
			r: []float64{float64(tasks), float64(len(next))},
		}
	}
	text := func(_ byte, v float64) string { return fmt.Sprintf("%.0f", v) }

	m := &Mirror{
		Left:  Half{Title: "Kept", Window: "open", Series: keptSeries},
		Right: Half{Title: "Queue", Window: "tasks + next actions, open", Series: queueSeries},
	}
	var yearL, yearR []float64
	m.Rows, yearL, yearR = a.mirror(start, day, keptSeries, queueSeries, text)
	v := day(today)
	figure := func(h *Half, now float64, year []float64) {
		h.Big, h.Unit = fmt.Sprintf("%.0f", now), "now"
		if year != nil {
			h.Versus = fmt.Sprintf("%.0f on average over the year", sum(year))
		}
	}
	figure(&m.Left, sum(v.l), yearL)
	figure(&m.Right, sum(v.r), yearR)
	return m, nil
}

// --- Inbox zero ------------------------------------------------------------

// What a cell of the year says.
const (
	ZeroNone  = "none" // a date with nothing to say: still to come, or before the record
	ZeroEmpty = "hit"  // the inbox held nothing at some point that day
	ZeroOpen  = "miss" // it held something all day
)

// ZeroDay is one cell. State is "" where the month has no such date — the
// 30th of February is a hole in the grid and not a day nothing is known about.
type ZeroDay struct {
	State string
	Title string // the day, and when it emptied or the least it held
	Today bool
}

type ZeroMonth struct {
	Label string
	Now   bool
	Days  []ZeroDay // always 31, so the columns are the days of the month
}

// ZeroYear is the calendar year so far: a month a row, a day a column.
type ZeroYear struct {
	Year    int
	Months  []ZeroMonth
	Reached int // days the inbox was empty at some point
	Never   int // days it was not
}

// InboxZeroYear replays the inbox from the log. Every capture puts one in and
// every answer takes one out, so the running count is the inbox as it was at
// each moment — nothing records emptiness directly, and nothing should: it is
// not a thing that happens, it is a thing that is true in between two things
// that happen.
//
// The count is anchored on what the inbox holds now and run backwards from
// there, rather than started at nothing and run forwards. A log that is
// missing an entry somewhere in its past then leaves the recent days right
// and the old ones wrong, which is the right way round for a screen read from
// today backwards.
func (a *App) InboxZeroYear() (*ZeroYear, error) {
	inbox, err := a.Inbox()
	if err != nil {
		return nil, err
	}
	type entry struct {
		at    time.Time
		delta int
	}
	rows, err := a.db.Query(`SELECT at, event FROM audit_log
		WHERE item_type='inbox' AND event<>? ORDER BY at, id`, EvEdited)
	if err != nil {
		return nil, err
	}
	var log []entry
	total := 0
	for rows.Next() {
		var at, event string
		if err := rows.Scan(&at, &event); err != nil {
			rows.Close()
			return nil, err
		}
		e := entry{parseTS(at), -1}
		if event == EvCreated {
			e.delta = 1
		}
		total += e.delta
		log = append(log, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	today := a.midnight(a.now())
	y := &ZeroYear{Year: today.Year()}
	level := len(inbox) - total // what the inbox held before the log's first entry
	held := func() int {
		if level < 0 {
			return 0
		}
		return level
	}
	var first time.Time
	if len(log) > 0 {
		first = a.midnight(log[0].at)
	}
	next := 0
	// everything before the first of January only moves the count
	jan := time.Date(y.Year, 1, 1, 0, 0, 0, 0, a.loc)
	for next < len(log) && log[next].at.Before(jan) {
		level += log[next].delta
		next++
	}
	for m := time.January; m <= time.December; m++ {
		start := time.Date(y.Year, m, 1, 0, 0, 0, 0, a.loc)
		zm := ZeroMonth{Label: start.Format("Jan"), Now: m == today.Month(), Days: make([]ZeroDay, 31)}
		last := start.AddDate(0, 1, -1).Day()
		for d := 1; d <= last; d++ {
			day := time.Date(y.Year, m, d, 0, 0, 0, 0, a.loc)
			cell := &zm.Days[d-1]
			cell.Today = day.Equal(today)
			if day.After(today) || len(log) == 0 || day.Before(first) {
				cell.State = ZeroNone
				continue
			}
			end := day.AddDate(0, 0, 1)
			least, emptied := held(), ""
			if least == 0 {
				emptied = "empty"
			}
			for next < len(log) && log[next].at.Before(end) {
				level += log[next].delta
				if h := held(); h < least {
					least = h
					if h == 0 && emptied == "" {
						emptied = "empty at " + log[next].at.In(a.loc).Format("15:04")
					}
				}
				next++
			}
			label := day.Format("Mon 2 Jan")
			if least == 0 {
				cell.State, cell.Title = ZeroEmpty, label+" · "+emptied
				y.Reached++
			} else {
				cell.State, cell.Title = ZeroOpen, fmt.Sprintf("%s · never below %d", label, least)
				y.Never++
			}
		}
		y.Months = append(y.Months, zm)
	}
	return y, nil
}
