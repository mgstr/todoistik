// The way in: a Reminders list is emptied into the inbox, one capture per
// reminder, and a reminder is deleted only once the app has said it has the
// text.
package main

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"todoistik/internal/apiclient"
)

func moveMain(args []string) {
	// anything still on the list is a failure worth an exit code: this runs
	// unattended, where the summary line is not read but the status is
	left, err := moveRun(args, flag.ExitOnError)
	finish("move", left, err)
}

// moveRun is one import, flags and all, and it comes back rather than exiting
// so that a loop can run it beside the others.
func moveRun(args []string, eh flag.ErrorHandling) (int, error) {
	fs := flag.NewFlagSet("move", eh)
	f := common(fs, "the Reminders list to empty (required)")
	list, err := f.parse(fs, args)
	if err != nil {
		return 0, err
	}
	return move(list, apiclient.Base(*f.base), *f.token, *f.dry)
}

// move carries one list across and returns how many reminders it had to leave
// behind.
//
// Every capture happens first, and the deletions follow in one pass, because
// each round trip to Reminders costs seconds: deleting one at a time would
// make a twenty-item list take minutes. The order still never loses anything —
// a reminder is only ever deleted after the app answered for it, and a run cut
// short between the two leaves the reminder in place, where the next run
// re-captures it, the app calls it a duplicate, and it is deleted then.
func move(list, base, token string, dry bool) (int, error) {
	all, err := readReminders(list)
	if err != nil {
		return 0, err
	}
	// completed reminders are left where they are: the list is being emptied
	// of what is still outstanding, and a finished reminder is a record, not
	// an inbox item
	var rems []reminder
	for _, r := range all {
		if !r.Completed {
			rems = append(rems, r)
		}
	}
	if len(rems) == 0 {
		return 0, nil // nothing open is the quiet outcome, and says so by saying nothing
	}

	c := apiclient.New(base, token)

	type delivered struct {
		id, text  string
		duplicate bool
	}
	var (
		done  []delivered
		left  int
		fatal error
	)

	for _, r := range rems {
		text := captureText(r)
		if text == "" {
			fmt.Fprintf(errOut, "kept: a reminder with no text (id %s)\n", r.ID)
			left++
			continue
		}
		if dry {
			fmt.Fprintf(out, "would capture: %s\n", text)
			continue
		}

		status, err := c.Capture(text)
		if err != nil {
			// a wrong token or an unreachable app fails identically for every
			// remaining reminder, so stop asking — but still delete what the
			// app already took, or the run would leave those to come back
			if errors.Is(err, apiclient.ErrFatal) {
				fatal = err
				left += len(rems) - len(done) - left
				break
			}
			fmt.Fprintf(errOut, "kept: %s (%v)\n", text, err)
			left++
			continue
		}
		done = append(done, delivered{id: r.ID, text: text, duplicate: status == "duplicate"})
	}

	if dry {
		fmt.Fprintf(out, "\n%d open on %q; nothing was captured or deleted\n", len(rems), list)
		return 0, nil
	}

	// what the app took, the list gives up
	stuck := map[string]string{}
	if len(done) > 0 {
		ids := make([]string, len(done))
		for i, d := range done {
			ids[i] = d.id
		}
		failures, err := deleteReminders(list, ids)
		if err != nil {
			return len(done) + left, err
		}
		for _, f := range failures {
			stuck[f.ID] = f.Why
		}
	}

	var captured, duplicate int
	for _, d := range done {
		switch {
		case stuck[d.id] != "":
			// captured but not deleted: say it plainly, because the next run
			// will capture it again and the app will call that a duplicate
			fmt.Fprintf(errOut, "captured but still on the list: %s (%s)\n", d.text, stuck[d.id])
			left++
		case d.duplicate:
			// the identical line was already sitting in the inbox, so the
			// reminder had nothing left to carry — but it is not a silent
			// drop, or a reminder that vanished would look like one that moved
			fmt.Fprintf(out, "duplicate (already in the inbox): %s\n", d.text)
			duplicate++
		default:
			fmt.Fprintf(out, "captured: %s\n", d.text)
			captured++
		}
	}

	// the tally follows the lines it counts, and only them. What was left
	// behind is counted in it but never the reason for it: a run that moved
	// nothing and failed has already said so on stderr, and a "0 captured, 0
	// duplicate" beside that is the zero tally this rule exists to prevent
	if captured+duplicate > 0 {
		fmt.Fprintf(out, "%d captured, %d duplicate, %d left on the list\n", captured, duplicate, left)
	}
	return left, fatal
}

// captureText writes a reminder as the capture todoistik takes. Everything the
// reminder holds goes in, because the reminder is deleted straight after: a due
// date or a note left out here is lost, and the inbox is raw text by design
// (design.md, "Capture") with no field to put them in instead.
//
// The title and the due date are the first line and the note is the rest. The
// note used to be flattened onto that one line with them, because the inbox was
// a list of lines and a note's own breaks had nowhere to go; they have one now
// (design.md, "Inbox item"), and a note is exactly what it is for — it survives
// the trip intact, and processing puts it in a description, which is where sync
// reads a note back out of.
//
// Nothing is written as a token — no `#tag`, no `@context`. A captured line is
// prose until someone processes it, and inventing notation here would be this
// utility deciding what an item means, which is the one thing capture must
// never require.
func captureText(r reminder) string {
	line := collapse(r.Name)
	if due := r.dueText(); due != "" {
		line += " (due " + due + ")"
	}
	line = strings.TrimSpace(line)
	body := strings.TrimSpace(strings.ReplaceAll(r.Body, "\r\n", "\n"))
	switch {
	case body == "":
		return line
	case line == "":
		// a reminder that is all note still carries something worth deciding
		// about, and an empty first line would be an inbox row with nothing on
		// it
		return body
	}
	return line + "\n" + body
}
