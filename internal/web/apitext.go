package web

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"todoistik/internal/app"
)

// The text spelling of a read (design.md, "The read API").
//
// It renders the answer the read API already gives — the same views, the same
// filters, the same items, the same refusals — and never a different one. The
// format says how the answer is written down; what is in it is the view's
// business and was settled before this file existed.
//
// It renders the answer rather than the screen the answer is normally seen on.
// A screen leaves things out because they are one keystroke away: a project's
// actions are on the project's own page, an action's description is one field
// down, and a row that carried them would be a page per item (see the
// "projectrow" and "actionrow" templates, which say so). A reader of text has
// no keystroke and no second page, so what is left out of the text is gone
// rather than deferred — which is why the text carries what the JSON carries.
//
// Every date is written out in full, and today's date is in the header. The
// screens say "3 weeks ago" because a person glancing down a list subtracts
// nothing; a text answer is read once and then quoted back an hour or a day
// later, by which time a relative date has quietly stopped being true.
//
// An item is named the way internal/request already names one — `::41` — so
// that whatever reads the answer can point at an item rather than describe it,
// and be understood when it says so back through the capture API.

// textIndent is what a body is written under its own line by. Four spaces
// rather than a tab, because the answer is pasted into places that render a
// tab as anything from two columns to eight.
const textIndent = "    "

// writeText is writeJSON's opposite number: the same answer, spelled for a
// person or for something reading over their shoulder.
func (s *Server) writeText(w http.ResponseWriter, view string, f app.Filters, problems []apiProblem, data any) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	io.WriteString(w, s.textAnswer(view, f, problems, data))
}

// textAnswer is the whole rendering, kept out of the handler so that a test
// can read it as a string instead of through a recorder.
func (s *Server) textAnswer(view string, f app.Filters, problems []apiProblem, data any) string {
	today, loc := s.app.Today(), s.app.Loc()
	var body strings.Builder
	count := -1 // -1 is "this view is not a list of items" — only "review" is

	switch v := data.(type) {
	case []*app.InboxItem:
		count = len(v)
		for _, it := range v {
			textRow(&body, it.ID, fields(it.Line(), "captured:"+day(it.CreatedAt, loc)), it.Body())
			endItem(&body)
		}

	case []*app.SomedayItem:
		count = len(v)
		for _, it := range v {
			line, rest := app.SplitCapture(it.Text)
			textRow(&body, it.ID, fields(line, tags(it.Tags),
				"captured:"+day(it.CreatedAt, loc), "reviewed:"+day(it.LastReviewedAt, loc)), rest)
			endItem(&body)
		}

	case []*app.Project:
		count = len(v)
		for _, p := range v {
			textProject(&body, p, today, loc, "")
			endItem(&body)
		}

	case []*app.Action:
		count = len(v)
		for _, a := range v {
			textAction(&body, a, today, loc, "")
			endItem(&body)
		}

	case *app.TodayView:
		count = len(v.OutOfTime) + len(v.Picked)
		textSection(&body, "Out of time", len(v.OutOfTime))
		for _, a := range v.OutOfTime {
			textAction(&body, a, today, loc, "")
			endItem(&body)
		}
		textSection(&body, "Picked", len(v.Picked))
		for _, a := range v.Picked {
			textAction(&body, a, today, loc, "")
			endItem(&body)
		}

	case []*app.ArchiveEntry:
		count = len(v)
		for _, e := range v {
			switch {
			case e.Project != nil:
				textProject(&body, e.Project, today, loc, "done:"+day(e.CompletedAt, loc))
			case e.Action != nil:
				textAction(&body, e.Action, today, loc, "done:"+day(e.CompletedAt, loc))
			}
			endItem(&body)
		}

	case []*app.Schedule:
		count = len(v)
		for _, sc := range v {
			next, last := "never again", "never fired"
			if sc.NextFire != "" {
				next = "next:" + sc.NextFire
			}
			if sc.LastFiredAt != nil {
				last = "fired:" + day(*sc.LastFiredAt, loc)
			}
			textRow(&body, sc.ID, fields(sc.Text+sc.Suffix, sc.RuleReadable, next, last), "")
			endItem(&body)
		}

	case *app.ReviewCounts:
		// Not a list of items but a list of numbers, so it is written as one.
		// A step with nothing outstanding says 0 rather than being left out:
		// the point of the review is that every step was looked at.
		fmt.Fprintf(&body, "inbox: %d\nwaiting for: %d\nprojects: %d\nnext actions: %d\nsomeday: %d\nschedules: %d\noutstanding: %d\n",
			v.Inbox, v.WaitingFor, v.Projects, v.Next, v.Someday, v.Schedules, v.OutstandingTotal())
	}

	var head strings.Builder
	if count < 0 {
		head.WriteString(view + "\n")
	} else {
		head.WriteString(view + " — " + items(count) + "\n")
	}
	head.WriteString("today: " + today + "\n")
	if f.Active() {
		head.WriteString("filter: " + f.Query() + "\n")
	}
	// What the read could not make of the request, said here for the reason
	// the JSON answer carries "problems": without it, a read of the whole view
	// and a read of the filtered one look exactly alike (design.md, "The read
	// API").
	if len(problems) > 0 {
		var said []string
		for _, p := range problems {
			said = append(said, app.QueryProblem{Token: p.Token, Kind: p.Kind}.String())
		}
		head.WriteString("not read: " + strings.Join(said, ", ") + "\n")
	}
	if body.Len() == 0 {
		return head.String()
	}
	return head.String() + "\n" + body.String()
}

// textAction writes one action: its line, then its description under it. extra
// is what the view adds and the action does not carry — the day it was
// completed, in the Archive.
func textAction(b *strings.Builder, a *app.Action, today string, loc *time.Location, extra string) {
	f := []string{a.Title}
	if a.ProjectTitle != "" {
		f = append(f, "["+a.ProjectTitle+" ::"+strconv.FormatInt(a.ProjectID, 10)+"]")
	}
	f = append(f, a.Meta())
	if a.BecameNextAt != nil {
		f = append(f, "next:"+day(*a.BecameNextAt, loc))
	}
	if a.IsOverdue(today) {
		f = append(f, "overdue")
	}
	f = append(f, extra)
	f = append(f, a.Errors()...)
	textRow(b, a.ID, fields(f...), a.Description)
}

// textProject writes one project, its definition of done and its actions. The
// Projects screen shows none of the three, because they are a keystroke away
// on the project's own page; here there is no keystroke.
func textProject(b *strings.Builder, p *app.Project, today string, loc *time.Location, extra string) {
	f := []string{p.Title}
	if p.Stalled {
		f = append(f, "stalled")
	}
	if p.IsSnoozed(today) {
		f = append(f, "snooze:"+p.SnoozeUntil)
	}
	f = append(f, tags(p.Tags), "opened:"+day(p.CreatedAt, loc), extra)
	f = append(f, p.Errors()...)
	textRow(b, p.ID, fields(f...), "")
	if p.DOD != "" {
		writeIndented(b, "done when: "+p.DOD)
	}
	for _, a := range p.Actions {
		var sub strings.Builder
		// the action's own project title is the one it is written under, and
		// saying it again on every line under it would be noise
		under := *a
		under.ProjectTitle = ""
		textAction(&sub, &under, today, loc, "")
		writeIndented(b, strings.TrimRight(sub.String(), "\n"))
	}
}

// textRow is one item: its id and the line that names it, then whatever body
// it carries indented under that line. It does not end itself — the blank line
// between items belongs to the list writing them, and a row nested under a
// project must not open a gap inside it.
func textRow(b *strings.Builder, id int64, line, body string) {
	fmt.Fprintf(b, "::%d  %s\n", id, line)
	if body != "" {
		writeIndented(b, body)
	}
}

// endItem separates one top-level item from the next, so that an item with a
// body and one without read the same way down the page.
func endItem(b *strings.Builder) { b.WriteString("\n") }

func writeIndented(b *strings.Builder, text string) {
	for _, l := range strings.Split(text, "\n") {
		if l == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(textIndent + l + "\n")
	}
}

func textSection(b *strings.Builder, title string, n int) {
	fmt.Fprintf(b, "%s — %s\n\n", title, items(n))
}

// fields joins what is there and drops what is not, so that an action with no
// context and no tags is a title and not a title followed by four spaces. Two
// spaces between, because one is what a title has inside it.
func fields(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "  ")
}

func tags(t []string) string {
	var out []string
	for _, one := range t {
		out = append(out, "#"+one)
	}
	return strings.Join(out, " ")
}

func day(t time.Time, loc *time.Location) string { return t.In(loc).Format(app.DateFormat) }

func items(n int) string {
	if n == 1 {
		return "1 item"
	}
	return strconv.Itoa(n) + " items"
}
