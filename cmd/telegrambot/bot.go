package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"

	"todoistik/internal/apiclient"
)

// view is one command: a view of the app, read through the read API.
type view struct {
	command string // what follows the slash, and the read API's name for it
	title   string // what the screen calls it
	// filters says whether the view takes a filter line. The inbox and Today
	// take none (design.md, "Today"), and a line given to one is said to be
	// refused rather than quietly dropped
	filters bool
}

// views are the commands, in the order the app's navigation lists them. The
// weekly review is not one: it answers with counts, and a review is done at the
// desk (design.md, "Design principles").
var views = []view{
	{"inbox", "Inbox", false},
	{"today", "Today", false},
	{"next", "Next actions", true},
	{"tasks", "Tasks", true},
	{"projects", "Projects", true},
	{"waiting", "Waiting for", true},
	{"calendar", "Calendar", true},
	{"someday", "Someday/Maybe", true},
	{"scheduler", "Scheduler", true},
	{"archive", "Archive", true},
}

// messageLimit is Telegram's longest message, in UTF-16 code units — the unit
// Telegram counts in, so a list in Cyrillic or with an emoji in it is measured
// the way it will be measured on arrival.
const messageLimit = 4096

// commandRe is a message that is a command: a slash, a name, and optionally
// the bot's own name after it, which is how a command picked from the menu in
// a group arrives. `/usr/bin is full` is not one, and is captured.
var commandRe = regexp.MustCompile(`^/([A-Za-z0-9_]+)(@[A-Za-z0-9_]+)?$`)

type bot struct {
	app *apiclient.Client
}

// outcome is what one message did: the replies to send, and the line the log
// gets. A failure goes to stderr, everything else to stdout.
type outcome struct {
	replies []string
	log     string
	failed  bool
}

func (b *bot) answer(text string) outcome {
	if strings.TrimSpace(text) == "" {
		// a photo, a sticker, a voice note: there is no text to capture, and
		// a caption without its photo would be a capture that lost its point
		return outcome{replies: []string{"Not saved: only text is captured"}, log: "not captured: a message with no text", failed: true}
	}

	first, rest := strings.TrimSpace(text), ""
	if i := strings.IndexAny(first, " \t\n"); i >= 0 {
		first, rest = first[:i], first[i:]
	}
	m := commandRe.FindStringSubmatch(first)
	if m == nil {
		return b.capture(text)
	}
	name := strings.ToLower(m[1])
	line := strings.Join(strings.Fields(rest), " ")

	if name == "start" || name == "help" {
		// what Telegram sends on the first open of a chat; the menu holds the
		// same list, and this is it written out once
		var lines []string
		for _, v := range views {
			lines = append(lines, "/"+v.command+" "+v.title)
		}
		return outcome{replies: []string{strings.Join(lines, "\n")}, log: "listed the commands"}
	}
	for _, v := range views {
		if v.command == name {
			return b.read(v, line)
		}
	}
	// a mistyped command is not a capture: `/nxet` would otherwise be an
	// inbox item nobody meant, found at the desk
	return outcome{replies: []string{"No such command: /" + m[1]}, log: "no such command: /" + m[1], failed: true}
}

// capture puts the message in the inbox as it was written, every line of it:
// the first line is the item and the rest rides under it (design.md, "Inbox
// item"). The reply says which of the two things happened, because a capture
// that might have vanished is one that has to be checked at the desk.
func (b *bot) capture(text string) outcome {
	status, err := b.app.Capture(text)
	if err != nil {
		return outcome{
			replies: []string{"Not saved: " + err.Error()},
			log:     fmt.Sprintf("not captured: %s (%v)", oneLine(text), err),
			failed:  true,
		}
	}
	if status == "duplicate" {
		return outcome{replies: []string{"Already in the inbox"}, log: "duplicate (already in the inbox): " + oneLine(text)}
	}
	return outcome{replies: []string{"Added to the inbox"}, log: "captured: " + oneLine(text)}
}

func (b *bot) read(v view, line string) outcome {
	label := "/" + v.command
	if line != "" {
		label += " " + line
	}
	fail := func(msg string) outcome {
		return outcome{replies: []string{msg}, log: fmt.Sprintf("not read: %s (%s)", label, msg), failed: true}
	}
	if line != "" && !v.filters {
		return fail(v.title + " has no filters")
	}

	answer, err := b.app.Read(v.command, line)
	if err != nil {
		return fail("Not read: " + err.Error())
	}
	// the app answers a line it could only partly read with the part it
	// could. On the screen the rest is marked; here the list would simply be
	// longer than asked for and look exactly like the right one
	if len(answer.Problems) > 0 {
		var parts []string
		for _, p := range answer.Problems {
			parts = append(parts, p.Token+" "+problemText(p.Kind))
		}
		return fail("Not read: " + strings.Join(parts, ", "))
	}

	lines, count, err := render(v, answer.Items)
	if err != nil {
		return fail("Not read: " + err.Error())
	}
	head := v.title
	if line != "" {
		head += " · " + line
	}
	head += fmt.Sprintf(" · %d", count)
	return outcome{
		replies: split(append([]string{head}, lines...), messageLimit),
		log:     fmt.Sprintf("read %s: %d", label, count),
	}
}

func problemText(kind string) string {
	switch kind {
	case "tag":
		return "is no tag"
	case "context":
		return "is no context"
	case "second-context":
		return "is a second context, and an action has one"
	case "not-a-filter":
		return "is not a filter"
	case "window":
		return "is not a window"
	}
	return "was not read"
}

// row is as much of any item as a line of a list shows. Every view's items
// decode into it: an action or a project has a title, an inbox item, an idea
// and a schedule have text, and an archive entry holds one of the two.
type row struct {
	Title    string `json:"title"`
	Text     string `json:"text"`
	DueDate  string `json:"dueDate"`
	Stalled  bool   `json:"stalled"`
	NextFire string `json:"nextFire"`
	Project  *row   `json:"project"`
	Action   *row   `json:"action"`
}

// render writes a view's items as lines, one per item, and says how many
// items there were. A line is the title and, where there is one, the date the
// view is about — and nothing else: a list on a phone is scanned, and what an
// item holds besides is read at the desk.
func render(v view, raw json.RawMessage) ([]string, int, error) {
	if v.command == "today" {
		var today struct {
			OutOfTime []row `json:"outOfTime"`
			Picked    []row `json:"picked"`
		}
		if err := json.Unmarshal(raw, &today); err != nil {
			return nil, 0, fmt.Errorf("the today view answered in a shape this does not read")
		}
		// the groups stay apart, the way the screen shows them, and an action
		// that is both due and picked is in both (design.md, "Today"). A list
		// here can carry a heading, which a Reminders list could not
		var lines []string
		for _, g := range []struct {
			name string
			rows []row
		}{{"Out of time", today.OutOfTime}, {"Picked", today.Picked}} {
			if len(g.rows) == 0 {
				continue
			}
			lines = append(lines, "", fmt.Sprintf("%s · %d", g.name, len(g.rows)))
			for _, r := range g.rows {
				lines = append(lines, itemLine(v, r))
			}
		}
		return lines, len(today.OutOfTime) + len(today.Picked), nil
	}

	var rows []row
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, 0, fmt.Errorf("the %s view did not answer with a list of items", v.command)
	}
	var lines []string
	for _, r := range rows {
		lines = append(lines, itemLine(v, r))
	}
	return lines, len(rows), nil
}

func itemLine(v view, r row) string {
	switch {
	case r.Action != nil:
		r = *r.Action
	case r.Project != nil:
		r = *r.Project
	}
	name := r.Title
	if name == "" {
		// the first line is the item; the rest is not shown in a list
		// anywhere in the app (design.md, "Inbox item")
		name, _, _ = strings.Cut(r.Text, "\n")
	}
	line := "• " + name
	switch {
	case r.Stalled:
		// stalled is the one mark a list may never drop: a project with no
		// next action looks alive otherwise (design.md, "Stalled projects")
		line += " · stalled"
	case v.command == "scheduler" && r.NextFire != "":
		// a schedule's date is when it next fires; it has no due date
		line += " · fires " + r.NextFire
	case r.DueDate != "" && v.command != "archive":
		// a completed item's deadline is history, not something coming
		line += " · due " + r.DueDate
	}
	return line
}

// split packs lines into as few messages as fit, breaking only between lines.
// A view longer than one message is sent whole across several: a list cut off
// with "and 40 more" is a list that cannot be trusted to be complete
// (design.md, "Goals").
func split(lines []string, limit int) []string {
	var (
		out  []string
		cur  []string
		size int
	)
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.TrimSpace(strings.Join(cur, "\n")))
		}
		cur, size = nil, 0
	}
	for _, l := range lines {
		n := units(l)
		if n > limit {
			// one item longer than a whole message; cut it rather than lose
			// the message it would have been refused in
			l = cut(l, limit)
			n = units(l)
		}
		if size > 0 && size+1+n > limit {
			flush()
		}
		if size > 0 {
			size++
		}
		cur = append(cur, l)
		size += n
	}
	flush()
	return out
}

func units(s string) int { return len(utf16.Encode([]rune(s))) }

func cut(s string, limit int) string {
	r := []rune(s)
	for units(string(r)) > limit-1 {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// oneLine writes a capture the way a log line can hold it.
func oneLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if line != strings.TrimSpace(text) {
		return line + " …"
	}
	return line
}
