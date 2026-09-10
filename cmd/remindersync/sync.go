// The way out, and the one thing that comes back: a Reminders list is made to
// say what a todoistik view says, and a reminder ticked off there files a
// completion request for a person to answer at the desk. Nothing is completed
// in the app from here, and running it twice changes nothing the second time.
package main

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"todoistik/internal/request"

	"todoistik/internal/apiclient"
)

func syncMain(args []string) {
	left, err := syncRun(args, flag.ExitOnError)
	finish("sync", left, err)
}

// syncRun is one pass of the export, flags and all, and it comes back rather
// than exiting so that a loop can run it beside the others.
func syncRun(args []string, eh flag.ErrorHandling) (int, error) {
	fs := flag.NewFlagSet("sync", eh)
	f := common(fs, "the Reminders list to make match the view (required)")
	view := fs.String("view", "next", "which view to mirror")
	query := fs.String("q", "", "the view's filter line, as it is typed on the screen: \"#gc #sync\"")
	list, err := f.parse(fs, args)
	if err != nil {
		return 0, err
	}
	return sync(list, apiclient.Base(*f.base), *f.token, *view, *query, *f.dry)
}

// source is what a request says it came from: this program's name for itself,
// and the word a future second kind of sync would not share.
const source = "reminders"

// desired is one item as the list should hold it: the title it is written
// under, which carries the marker that identifies it, and the two fields
// Reminders has somewhere to put.
type desired struct {
	ItemID int64  `json:"-"`     // the action's own id, which the marker carries
	Name   string `json:"-"`     // the item's name alone, for the printed line
	Title  string `json:"title"` // what the reminder is called: the name and the marker
	Body   string `json:"body"`  // "" = todoistik has nothing to say about the note
	Due    string `json:"due"`   // "YYYY-MM-DD", "" = no due date in todoistik
}

// update is one reminder already on the list, and the fields that moved. An
// empty field means "leave it alone", never "clear it" — see plan.
type update struct {
	ID    string `json:"id"`
	Name  string `json:"-"`     // for the printed line
	Title string `json:"title"` // set only when the item was renamed in todoistik
	Body  string `json:"body"`
	Due   string `json:"due"`
}

// pending is a ticked reminder: a message rather than an item, to be filed as
// a completion request and then taken off the list.
type pending struct {
	reminder reminder
	line     string // the captured line it files
}

// sync reads the view, reads the list, and returns how many items it could not
// place.
//
// The read of the app comes first and the whole of it: a half-read view would
// look like a filter that had lost items, and the deletions would then act on
// that. Everything after it is one visit to Reminders per kind of change,
// because a round trip costs about five seconds whatever it carries.
func sync(list, base, token, view, query string, dry bool) (int, error) {
	c := apiclient.New(base, token)
	items, err := c.View(view, query)
	if err != nil {
		return 0, err
	}

	var (
		want    []desired
		skipped int
	)
	for _, it := range items {
		d := reminderFor(it)
		if d.Name == "" || d.ItemID == 0 {
			// nothing to write it under, or nothing to recognise it by on the
			// next run: either way it cannot be mirrored
			skipped++
			continue
		}
		want = append(want, d)
	}

	have, err := readReminders(list)
	if err != nil {
		return 0, err
	}
	p := plan(want, have, time.Now())

	if skipped > 0 {
		fmt.Fprintf(errOut, "skipped: %d item(s) with no name or no id\n", skipped)
	}
	// an item already saying what the view says is not mentioned at all.
	// Nothing happened to it, and this runs every few minutes: the lines worth
	// reading are the ones where something moved

	if dry {
		for _, q := range p.Requests {
			fmt.Fprintf(out, "would file: %s\n", q.line)
		}
		for _, d := range p.Create {
			fmt.Fprintf(out, "would create: %s%s\n", d.Title, dueNote(d.Due))
		}
		for _, u := range p.Update {
			fmt.Fprintf(out, "would update: %s (%s)\n", u.Name, movedFields(u))
		}
		for _, id := range p.Delete {
			fmt.Fprintf(out, "would delete: %s\n", titleOf(have, id))
		}
		// a dry run always says something, even when the answer is "nothing":
		// it was asked what it would do, and "nothing" is an answer
		fmt.Fprintf(out, "\n%d to file, %d to create, %d to update, %d to delete, %d already there on %q; nothing was changed\n",
			len(p.Requests), len(p.Create), len(p.Update), len(p.Delete), len(p.Unchanged), list)
		return 0, nil
	}

	left := skipped
	var filed, created, updated, deleted int

	// the requests go first, and a reminder is taken off the list only once the
	// app has answered for its line — the same order the import keeps, and for
	// the same reason: a tick deleted before it is filed is a tick nobody can
	// get back
	spent := make([]string, 0, len(p.Requests))
	for _, q := range p.Requests {
		status, err := c.Capture(q.line)
		if err != nil {
			fmt.Fprintf(errOut, "kept: %s (%v)\n", q.line, err)
			left++
			continue
		}
		spent = append(spent, q.reminder.ID)
		filed++
		if status == "duplicate" {
			// the identical request is already in the inbox, unanswered
			fmt.Fprintf(out, "filed (already in the inbox): %s\n", q.line)
		} else {
			fmt.Fprintf(out, "filed: %s\n", q.line)
		}
	}

	// then the additions, with the deletions last: a run cut off in the middle
	// leaves the list holding too much rather than too little, and too much is
	// what the next run is able to fix

	stuck, err := createReminders(list, p.Create)
	if err != nil {
		return left + len(p.Create), err
	}
	// a create has no id to be named by, so its failure names the title
	why := whyBy(stuck)
	for _, d := range p.Create {
		if w := why[d.Title]; w != "" {
			fmt.Fprintf(errOut, "could not create: %s (%s)\n", d.Name, w)
			left++
			continue
		}
		fmt.Fprintf(out, "created: %s%s\n", d.Title, dueNote(d.Due))
		created++
	}

	stuck, err = updateReminders(list, p.Update)
	if err != nil {
		return left + len(p.Update), err
	}
	why = whyBy(stuck)
	for _, u := range p.Update {
		if w := why[u.ID]; w != "" {
			fmt.Fprintf(errOut, "could not update: %s (%s)\n", u.Name, w)
			left++
			continue
		}
		fmt.Fprintf(out, "updated: %s (%s)\n", u.Name, movedFields(u))
		updated++
	}

	// what the app took, the list gives up: the spent ticks go with the
	// strangers, in the one visit
	going := append(p.Delete, spent...)
	stuck, err = deleteReminders(list, going)
	if err != nil {
		return left + len(going), err
	}
	why = whyBy(stuck)
	for _, id := range going {
		if w := why[id]; w != "" {
			fmt.Fprintf(errOut, "could not delete: %s (%s)\n", titleOf(have, id), w)
			left++
			continue
		}
		fmt.Fprintf(out, "deleted: %s\n", titleOf(have, id))
		deleted++
	}

	// the tally follows the lines it counts, and only them. What was left
	// behind is counted in it but never the reason for it: a pass that moved
	// nothing has printed nothing, whether because the list already said what
	// the view says or because the run failed and stderr carries why
	if filed+created+updated+deleted > 0 {
		fmt.Fprintf(out, "%d filed, %d created, %d updated, %d deleted, %d already there on %q\n",
			filed, created, updated, deleted, len(p.Unchanged), list)
	}
	return left, nil
}

// reminderFor writes one item as the list should hold it.
//
// The title is the item's name and the marker, and nothing else. None of the
// notation the filter was written in goes across: a `#gc` in a reminder's name
// would be todoistik's vocabulary leaking into a program that has no idea what
// a tag is. The marker is not vocabulary — it is an identity, meaningless to
// Reminders and meaningless as prose — which is why it can share the field.
//
// The description becomes the note and the due date becomes the reminder's own
// due date, because those two are the only fields Reminders has that mean the
// same thing. A tag, a context, a size, a project: todoistik keeps those, and
// the reminder is a copy of what to do, not a copy of the item.
func reminderFor(it apiclient.Item) desired {
	name := collapse(it.Title)
	if name == "" {
		// an inbox or someday item is text, not a title
		name = collapse(it.Text)
	}
	d := desired{
		ItemID: it.ID,
		Name:   name,
		Body:   collapse(it.Description),
		Due:    strings.TrimSpace(it.DueDate),
	}
	if name != "" && it.ID != 0 {
		d.Title = name + " " + request.Marker(it.ID)
	}
	return d
}

// requestLine writes one ticked reminder as the line the app captures. The
// name goes in without the marker: the marker is bookkeeping, and the line
// already carries the id where a machine can read it.
//
// A reminder ticked but carrying no completion date is stamped with the moment
// of the run. Reminders records one normally, and the run's own clock is the
// closest honest answer left when it does not — nearer the truth than a
// midnight, and the only guess anywhere in this channel.
func requestLine(r reminder, id int64, now time.Time) string {
	at := now
	if r.DoneAt != "" {
		if t, err := time.ParseInLocation(request.TimeFormat, r.DoneAt, time.Local); err == nil {
			at = t
		}
	}
	return request.Write(request.Line{
		Source: source,
		ItemID: id,
		At:     at,
		Name:   request.WithoutMarker(collapse(r.Name)),
	})
}

type syncPlan struct {
	Requests  []pending
	Create    []desired
	Update    []update
	Delete    []string // reminder ids
	Unchanged []string // names already saying what the view says
}

// plan works out the whole difference before anything is written, so that a
// dry run and a real run are the same decision made twice.
//
// **A ticked reminder is a message, not an item.** If it carries a marker it
// files a completion request and is then taken off the list, whether or not its
// action is still in the view: the marker is an identity and the app resolves
// one without a view, so an action snoozed or parked between the tick and the
// run does not lose the tick. Deleting it is what makes ignoring a request
// stick — nothing anywhere remembers that one was made, so a tick left in
// place would be filed again on every run.
//
// **The list is the view.** A reminder carrying no marker, or a marker no item
// in the view holds, is deleted, whoever typed it. That is what makes the list
// readable as an answer to "what is on this filter" rather than a pile that
// only grows.
//
// **A reminder already there is not rewritten into a copy of the item.** Only
// what todoistik actually knows is written: a note or a due date the item does
// not carry leaves the reminder's own alone, because an empty field in
// todoistik is not an assertion that Reminders should be empty too, and
// clearing it would erase whatever the phone had set, on every single run.
func plan(want []desired, have []reminder, now time.Time) syncPlan {
	var p syncPlan

	byID := map[int64][]reminder{}
	for _, r := range have {
		id, ok := request.ItemFromMarker(r.Name)
		if !ok {
			continue
		}
		byID[id] = append(byID[id], r)
		if r.Completed {
			p.Requests = append(p.Requests, pending{reminder: r, line: requestLine(r, id, now)})
		}
	}

	wanted := map[int64]bool{}
	for _, d := range want {
		wanted[d.ItemID] = true
		rs := byID[d.ItemID]
		open, changed := 0, false
		for _, r := range rs {
			if r.Completed {
				continue // it is a request now, and on its way off the list
			}
			open++
			if u, ok := changes(d, r); ok {
				p.Update = append(p.Update, u)
				changed = true
			}
		}
		switch {
		case open > 0:
			if !changed {
				p.Unchanged = append(p.Unchanged, d.Name)
			}
		case len(rs) > 0:
			// its only reminders were ticked: the request is in flight, and
			// whether the item comes back to the phone is the answer to it
		default:
			p.Create = append(p.Create, d)
		}
	}

	for _, r := range have {
		id, marked := request.ItemFromMarker(r.Name)
		if marked && r.Completed {
			continue // already leaving, as a request that was filed
		}
		if !marked || !wanted[id] {
			p.Delete = append(p.Delete, r.ID)
		}
	}
	return p
}

// changes is what the reminder would have to be told, and nothing more.
//
// The title is written when the item was renamed in todoistik, which is the
// whole point of an identity that is not the title: the reminder is the same
// reminder, under the name the item is called now.
//
// A due date already on the right day is left exactly as it is, time and all:
// an action's due date is a day and nothing finer, so a reminder due at 14:30
// on that day is already saying everything the action says, and rewriting it
// to midnight would throw away a time only the phone knew.
func changes(d desired, r reminder) (update, bool) {
	u := update{ID: r.ID, Name: d.Name}
	if collapse(r.Name) != d.Title {
		u.Title = d.Title
	}
	if d.Body != "" && collapse(r.Body) != d.Body {
		u.Body = d.Body
	}
	if d.Due != "" && r.dueDay() != d.Due {
		u.Due = d.Due
	}
	if u.Title == "" && u.Body == "" && u.Due == "" {
		return update{}, false
	}
	return u, true
}

// --- saying what happened -------------------------------------------------

// whyBy indexes what a write could not do, by whatever the script named it —
// an id, or a title where there was no id yet.
func whyBy(stuck []writeFailure) map[string]string {
	why := map[string]string{}
	for _, f := range stuck {
		why[f.ID] = said(f.Why, "Reminders would not say why")
	}
	return why
}

func dueNote(due string) string {
	if due == "" {
		return ""
	}
	return " (due " + due + ")"
}

// movedFields names what an update carries, so the printed line says what
// changed rather than only that something did.
func movedFields(u update) string {
	var parts []string
	if u.Title != "" {
		parts = append(parts, "renamed")
	}
	if u.Due != "" {
		parts = append(parts, "due "+u.Due)
	}
	if u.Body != "" {
		parts = append(parts, "note")
	}
	return strings.Join(parts, ", ")
}

// titleOf names a reminder about to be deleted by what it said, because an id
// is not something a person can go and look for on the list.
func titleOf(have []reminder, id string) string {
	for _, r := range have {
		if r.ID != id {
			continue
		}
		if t := request.WithoutMarker(collapse(r.Name)); t != "" {
			return t
		}
		return "a reminder with no title (id " + id + ")"
	}
	return "id " + id
}
