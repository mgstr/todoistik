// Command mailsync empties a Gmail label into the todoistik inbox, one capture
// per message, and takes the label off once the app has said it has the text.
//
// The label is the queue: a mail worth deciding about gets the label wherever
// it is read, and this takes what is wearing it. Taking the label off is a
// move to All Mail, because a Gmail label is an IMAP folder and moving a
// message out of one is exactly "remove this label" — the mail is not deleted,
// not marked read and not moved out of the account.
//
// It talks to the app the way anything outside it does, through the capture
// API under a bearer token, because that is the only way in by design
// (design.md, "External capture") and going around it would skip the duplicate
// collapse and the audit entry.
//
// See implementation.md, "Mail into the inbox".
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emersion/go-imap/v2"

	"todoistik/internal/apiclient"
)

// source is what this program is called in an inbox item's "source" (design.md,
// "Inbox item"). One name and not one per label or per account: the channel is
// mail, and which mailbox it came out of is in the capture's own body, where
// the link to the message already is.
const source = "mail"

// Where a pass writes. **A pass that changed nothing says nothing**: every
// line on stdout is a mail that actually moved, because this runs unattended
// for months and a log saying "nothing wearing the label" nine hundred times
// is a log nobody reads — which is what makes it a log that hides the line
// that mattered. What went wrong goes to stderr whether or not anything moved.
//
// They are variables because the loop lends each pass writers of its own: one
// that stamps the pass the first time anything is written to it, and one that
// puts that time in front of every failure. A heading with nothing under it is
// exactly the noise this avoids, so the heading waits for a line to head.
var (
	out    io.Writer = os.Stdout
	errOut io.Writer = os.Stderr
)

func main() {
	fs := flag.NewFlagSet("mailsync", flag.ExitOnError)
	var (
		label    = fs.String("label", "", "the Gmail label to empty (required)")
		user     = fs.String("user", os.Getenv("MAILSYNC_USER"), "the account, you@gmail.com (required)")
		pwFile   = fs.String("password-file", "", "a file holding the app password; otherwise MAILSYNC_PASSWORD")
		server   = fs.String("server", env("MAILSYNC_SERVER", "imap.gmail.com:993"), "the IMAP server")
		account  = fs.Int("account", 0, "which signed-in Google account the links open in (/mail/u/N/)")
		base     = fs.String("url", env("TODOISTIK_URL", "http://127.0.0.1:8390"), "the running app's base URL")
		token    = fs.String("token", os.Getenv("TODOISTIK_TOKEN"), "bearer token; empty for a server started without one")
		dry      = fs.Bool("dry-run", false, "print what would be captured; change nothing on either side")
		period   = fs.Int("period", 5, "minutes between passes, counted from the end of one to the start of the next; 0 runs one pass and exits")
		password string
	)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `mailsync empties a Gmail label into the todoistik inbox.

  MAILSYNC_PASSWORD=... mailsync -label todoistik -user you@gmail.com

It empties the label every few minutes until it is stopped. -period 0 is one
pass and out, for a launchd or cron entry that does the repeating itself.

The password is an app password and is never a flag: a flag is on the process
list for anything on the machine to read.

`)
		fs.PrintDefaults()
	}
	fs.Parse(os.Args[1:])

	// a run without a label has nothing to work on, and defaulting one would
	// be a guess about which mail to empty out of the account
	if strings.TrimSpace(*label) == "" || strings.TrimSpace(*user) == "" {
		fs.Usage()
		os.Exit(2)
	}
	// a period is in minutes, and minutes do not run backwards
	if *period < 0 {
		fmt.Fprintln(os.Stderr, "mailsync: -period is in minutes and cannot be negative")
		os.Exit(2)
	}
	password, err := readPassword(*pwFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mailsync: %v\n", err)
		os.Exit(2)
	}

	c := config{
		label: *label, user: *user, password: password, server: *server,
		account: *account, base: apiclient.Base(*base), token: *token, dry: *dry,
	}

	// A dry run is one pass however long the period is: it changes nothing, so
	// every pass after the first could only print the same mail over again. It
	// is also read at a prompt, and a command that does not come back is the
	// wrong answer to "show me what this would do".
	if *period == 0 || c.dry {
		left, err := run(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mailsync: %v\n", err)
		}
		// anything still wearing the label is a failure worth an exit code:
		// this runs unattended, where the summary line is not read but the
		// status is
		if left > 0 || err != nil {
			os.Exit(1)
		}
		return
	}

	// interrupted mid-pass, it stops after the pass it is in rather than
	// halfway through one: a pass abandoned between a capture and an unlabel
	// is exactly the state the pass's own ordering works to avoid
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	loop(ctx, c, time.Duration(*period)*time.Minute)
}

// loop empties the label over and over, until it is stopped.
//
// A failure belongs to the pass and never to the loop: a login refused or an
// app unreachable now is usually one that works in five minutes, and a process
// that exited on the first of them would need something else watching it in
// order to come back — which is the whole of what the loop is here to avoid.
//
// The wait starts when a pass ends and not when it began, so two passes can
// never overlap however long one takes. That is the one arrangement that keeps
// the label from being read a second time while the first pass is still
// deciding what to do with what it found.
func loop(ctx context.Context, c config, period time.Duration) {
	stdout, stderr := out, errOut
	defer func() { out, errOut = stdout, stderr }()
	for {
		// Each stream says the time for itself, because the two are often
		// redirected to different files and a line in one cannot lean on a
		// line in the other. The stamp on stdout is a heading printed the
		// first time the pass writes anything, so a pass that moved no mail
		// prints nothing at all; a failure carries its time inline, because it
		// is read on its own, out of a much longer file.
		at := time.Now().Format("2006-01-02 15:04:05")
		out = &headed{stdout, once(stdout, "=== "+at)}
		errOut = &prefixed{stderr, at + " mailsync: "}

		left, err := run(c)
		switch {
		case err != nil:
			fmt.Fprintf(errOut, "%v\n", err)
		case left > 0:
			fmt.Fprintf(errOut, "%d left labelled\n", left)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(period):
		}
	}
}

// headed is a writer that lets something be said before the first thing
// written through it, and nothing at all if nothing is.
type headed struct {
	w    io.Writer
	head func()
}

func (h *headed) Write(p []byte) (int, error) {
	h.head()
	return h.w.Write(p)
}

// once returns a printer that says its line the first time it is called and
// never again, so a pass that moved three mails is stamped once.
func once(w io.Writer, line string) func() {
	done := false
	return func() {
		if done {
			return
		}
		done = true
		fmt.Fprintln(w, line)
	}
}

// prefixed puts the same words in front of every line written through it. One
// Fprintf is one Write and every line here is one Fprintf, which is what makes
// a prefix per write a prefix per line.
type prefixed struct {
	w      io.Writer
	prefix string
}

func (p *prefixed) Write(b []byte) (int, error) {
	if _, err := io.WriteString(p.w, p.prefix); err != nil {
		return 0, err
	}
	return p.w.Write(b)
}

type config struct {
	label, user, password, server string
	account                       int
	base, token                   string
	dry                           bool
}

// run carries one label across and returns how many messages it had to leave
// wearing it.
//
// Every capture happens first and the labels come off in one pass afterwards.
// Nothing is lost by the order: a message's label is only ever removed after
// the app has answered for it, and a run cut short between the two leaves the
// label on, where the next run captures the mail again, is told it is a
// duplicate, and takes the label off then.
func run(c config) (int, error) {
	box, err := dial(c.server, c.user, c.password, c.label)
	if err != nil {
		return 0, err
	}
	defer box.close()

	msgs, err := box.messages()
	if err != nil {
		return 0, err
	}
	if len(msgs) == 0 {
		return 0, nil // a run that moved nothing says nothing
	}

	type delivered struct {
		uid       imap.UID
		text      string
		duplicate bool
	}
	var (
		done  []delivered
		left  int
		fatal error
	)

	app := apiclient.New(c.base, c.token)
	for _, m := range msgs {
		text := captureText(m, c.account)
		if text == "" {
			fmt.Fprintf(errOut, "kept: a message with no subject, no sender and no id (uid %v)\n", m.UID)
			left++
			continue
		}
		if c.dry {
			fmt.Fprintf(out, "would capture: %s\n", oneLine(text))
			continue
		}

		status, err := app.Capture(text, source)
		if err != nil {
			// a wrong token or an unreachable app fails identically for every
			// message left, so stop asking — but still unlabel what the app
			// already took, or the run would leave those to come back
			if errors.Is(err, apiclient.ErrFatal) {
				fatal = err
				left += len(msgs) - len(done) - left
				break
			}
			fmt.Fprintf(errOut, "kept: %s (%v)\n", oneLine(text), err)
			left++
			continue
		}
		done = append(done, delivered{uid: m.UID, text: text, duplicate: status == "duplicate"})
	}

	if c.dry {
		fmt.Fprintf(out, "\n%d wearing %q; nothing was captured and no label was touched\n", len(msgs), c.label)
		return 0, nil
	}

	// what the app took, the label gives up
	uids := make([]imap.UID, len(done))
	for i, d := range done {
		uids[i] = d.uid
	}
	stuck := box.unlabel(uids)

	var captured, duplicate int
	for _, d := range done {
		switch {
		case stuck != nil:
			// captured but still labelled: say it plainly, because the next
			// run will capture it again and the app will call it a duplicate
			fmt.Fprintf(errOut, "captured but still labelled: %s\n", oneLine(d.text))
			left++
		case d.duplicate:
			// the identical capture was already sitting in the inbox, so the
			// mail had nothing left to carry — but it is not a silent drop, or
			// a mail that lost its label would look like one that moved
			fmt.Fprintf(out, "duplicate (already in the inbox): %s\n", oneLine(d.text))
			duplicate++
		default:
			fmt.Fprintf(out, "captured: %s\n", oneLine(d.text))
			captured++
		}
	}
	if stuck != nil {
		fmt.Fprintf(errOut, "mailsync: %v\n", stuck)
	}

	// the tally follows the lines it counts, and only them: a run that moved
	// nothing and failed has already said so on stderr, and "0 captured, 0
	// duplicate" beside that is the zero tally worth not printing
	if captured+duplicate > 0 {
		fmt.Fprintf(out, "%d captured, %d duplicate, %d left labelled\n", captured, duplicate, left)
	}
	return left, fatal
}

// readPassword takes the app password from a file when one is named, and from
// the environment otherwise. Never from a flag: a flag is on the process list
// for anything on the machine to read, and this one has the whole mailbox
// behind it.
func readPassword(file string) (string, error) {
	if strings.TrimSpace(file) != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		if pw := strings.TrimSpace(string(b)); pw != "" {
			return pw, nil
		}
		return "", fmt.Errorf("%s holds no password", file)
	}
	pw := strings.TrimSpace(os.Getenv("MAILSYNC_PASSWORD"))
	if pw == "" {
		return "", errors.New("no password: set MAILSYNC_PASSWORD or -password-file (a Google app password, not the account password)")
	}
	return pw, nil
}

// oneLine writes a capture the way a log line can hold it. The capture itself
// keeps its lines — they are what makes the sender and the link a body rather
// than part of the subject — but a run printing three lines per message would
// bury the one that failed.
func oneLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	if line != text {
		return line + " …"
	}
	return line
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
