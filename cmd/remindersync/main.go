// Command remindersync carries items between a macOS Reminders list and
// todoistik. It goes in both directions, and which one is named on the command
// line, because they are not opposites and confusing them would cost items:
//
//	move  a Reminders list into the inbox — one capture per reminder, and the
//	      reminder deleted once the app has said it has the text. The list is
//	      emptied; todoistik gains items.
//	sync  a view of todoistik onto a Reminders list — the list is made to
//	      match what the view holds, and a reminder ticked off there files a
//	      completion request for a person to answer. It can be run again and
//	      again, and the second run changes nothing.
//	loop  every direction named in a file, over and over, so that the two
//	      programs stay in step without anything being run by hand.
//
// Both talk to the app the way anything outside it does — the capture API on
// the way in, the read API on the way out, bearer token on both — rather than
// opening the database, because those two are the only ways in and out by
// design (design.md, "External capture" and "The read API"), and going around
// them would skip the duplicate collapse and the audit entry.
//
// Both talk to Reminders through osascript, because Reminders has no other
// interface: EventKit would mean a second language and its own signed bundle
// for the privacy prompt, and the scripting interface holds the same data.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "move":
		moveMain(os.Args[2:])
	case "sync":
		syncMain(os.Args[2:])
	case "loop":
		loopMain(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		// the two directions do opposite things to the same list, so an
		// unnamed one is never guessed at
		fmt.Fprintf(os.Stderr, "remindersync: no such direction: %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

// Where a run says what it did, and where it says what went wrong.
//
// **A run that changed nothing says nothing.** Every line printed is an item
// that actually moved: a reminder written, retitled, deleted, or a request
// filed. Nothing else — no heading, no "nothing to do", no tally of zeroes.
// This runs every few minutes for months, and a log that says "0 created, 0
// deleted" nine hundred times is a log nobody reads, which means it is a log
// that hides the tenth line that mattered. Silence is the good outcome, and it
// is worth being able to recognise at a glance.
//
// **What went wrong goes to stderr**, always, whether or not anything moved. A
// failure is not a change, but it is never the quiet outcome either.
//
// They are variables because the loop lends each run writers of its own (see
// loop.go): one that prints the run's heading the first time anything is
// written to it, and one that puts the time and the run in front of every
// failure. A heading nothing follows is exactly the noise this avoids.
var (
	out    io.Writer = os.Stdout
	errOut io.Writer = os.Stderr
)

func usage() {
	fmt.Fprint(os.Stderr, `remindersync moves items between macOS Reminders and todoistik.

  remindersync move -list "Inbox"                        empty a Reminders list into the inbox
  remindersync sync -list "geocaching" -q "#gc #sync"    make a Reminders list match a view
  remindersync loop -period 5 ~/remindersync.conf        both of the above, every few minutes

Run any of them with -h for its own flags.
`)
}

// flags are what both directions need: which list, which app, and the promise
// to change nothing.
type flags struct {
	list  *string
	base  *string
	token *string
	dry   *bool
}

func common(fs *flag.FlagSet, listUsage string) *flags {
	return &flags{
		list:  fs.String("list", "", listUsage),
		base:  fs.String("url", env("TODOISTIK_URL", "http://127.0.0.1:8390"), "the running app's base URL"),
		token: fs.String("token", os.Getenv("TODOISTIK_TOKEN"), "bearer token; empty for a server started without one"),
		dry:   fs.Bool("dry-run", false, "print what would happen; change nothing on either side"),
	}
}

// parse reads the flags and answers with the list name. A run without a list
// has nothing to work on, and defaulting one would be a guess about which list
// to empty or overwrite.
//
// Whether that is fatal is the flag set's own business, which is what lets one
// line of a loop's config file be wrong without taking the loop down: a
// direction typed at the prompt exits, and the same direction read from a file
// comes back as an error for the caller to print.
func (f *flags) parse(fs *flag.FlagSet, args []string) (string, error) {
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() > 0 {
		return "", fmt.Errorf("%q is not a flag; every argument here is one", fs.Arg(0))
	}
	list := strings.TrimSpace(*f.list)
	if list == "" {
		err := errors.New("-list is required")
		if fs.ErrorHandling() == flag.ExitOnError {
			fmt.Fprintf(os.Stderr, "remindersync %s: %v\n", fs.Name(), err)
			fs.Usage()
			os.Exit(2)
		}
		return "", err
	}
	return list, nil
}

// finish reports one run the way an unattended run is read: the summary line
// is for a person, the exit status is for whatever ran it, so anything left
// undone has to reach the status too.
func finish(direction string, left int, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "remindersync %s: %v\n", direction, err)
		os.Exit(1)
	}
	if left > 0 {
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// jsString writes a Go string as a literal the script can hold. A JSON string
// is a JavaScript string, so encoding/json is the whole escaping rule.
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil { // unreachable for a string
		return `""`
	}
	return string(b)
}

// jsValue writes anything the scripts are handed — a list of ids, a list of
// reminders to create — as a JavaScript literal, by the same rule.
func jsValue(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// jsonUnmarshal reads what a script answered, and says whose answer it could
// not read: a malformed line here is Reminders having changed under us, not a
// bad item.
func jsonUnmarshal(out string, v any) error {
	if err := json.Unmarshal([]byte(out), v); err != nil {
		return fmt.Errorf("cannot read what Reminders answered: %v", err)
	}
	return nil
}

// said answers with the message when there is one, and with the fallback when
// whatever produced it said nothing. Reminders and the app both have ways of
// failing without a reason, and a line reading "kept: ()" says less than one
// naming the guess.
func said(msg, fallback string) string {
	if strings.TrimSpace(msg) != "" {
		return msg
	}
	return fallback
}

// collapse puts a multi-line string on one line. It is what makes two titles
// comparable at all, and what keeps a title that was typed with a break in it
// from becoming an item whose second line is part of its name.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
