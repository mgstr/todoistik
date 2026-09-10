// Every direction, over and over: the two programs stay in step without
// anything being run by hand.
//
// It is a loop rather than a cron entry because Reminders costs about five
// seconds a visit and answers one caller at a time: a schedule that fires two
// runs at once has them queueing behind each other with no idea the other is
// there, and a laptop that was asleep has cron firing all the missed ones at
// once. One process, one pass at a time, is the whole of the concurrency
// control — and it is also the only way `launchd` or `cron` is not needed to
// try this out.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func loopMain(args []string) {
	fs := flag.NewFlagSet("loop", flag.ExitOnError)
	url := fs.String("url", env("TODOISTIK_URL", "http://127.0.0.1:8390"), "the running app's base URL, for every run that does not name its own")
	token := fs.String("token", os.Getenv("TODOISTIK_TOKEN"), "bearer token, for every run that does not name its own")
	period := fs.Int("period", 5, "minutes between passes, counted from the end of one to the start of the next")
	dry := fs.Bool("dry-run", false, "print what every run would do; change nothing on either side")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: remindersync loop [flags] <config file>

The config file holds one run per line, each line being the arguments of a
direction — the same words that would follow "remindersync" at the prompt:

  # the phone's inbox, into todoistik
  move -list Inbox
  # what is tagged both, out to the watch
  sync -list geocaching -q "#gc #sync"
  # a second list, from a different filter
  sync -list "к покупке" -view next -q "@grocery"

Blank lines and # comments are ignored. -url and -token below are the default
for every line; a line naming its own wins.

`)
		fs.PrintDefaults()
	}
	fs.Parse(args)

	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "remindersync loop: one config file is required")
		fs.Usage()
		os.Exit(2)
	}
	if *period < 1 {
		fmt.Fprintln(os.Stderr, "remindersync loop: -period is in minutes and cannot be less than one")
		os.Exit(2)
	}

	runs, err := readRuns(fs.Arg(0))
	if err != nil {
		// a config file that is wrong is fatal before the first pass, not
		// discovered on the fortieth: the same rule the app's own settings file
		// keeps (README, "Settings")
		fmt.Fprintf(os.Stderr, "remindersync loop: %v\n", err)
		os.Exit(2)
	}
	for i := range runs {
		runs[i].defaults(*url, *token, *dry)
	}

	// what it will do, said once, at the start. Everything after this is
	// silence unless something moves, so this block is the only place a log
	// says which runs these lines are coming from — and it is the parsed runs
	// rather than the file's own text, because what was understood is the
	// thing worth checking: a comment, a dropped program name and a quoted
	// filter line all read back here as what will actually run
	fmt.Fprintf(out, "%d run(s) every %d minute(s), from %s:\n", len(runs), *period, fs.Arg(0))
	for _, r := range runs {
		fmt.Fprintf(out, "  %s\n", r)
	}
	fmt.Fprintln(out)

	// interrupted mid-pass, it stops after the run it is in rather than
	// halfway through one: a pass abandoned between a capture and a deletion is
	// exactly the state the run's own ordering works to avoid
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	loop(ctx, runs, time.Duration(*period)*time.Minute)
}

// a run is one line of the config file: which direction, its arguments, and
// the words to call it by in the output.
type run struct {
	direction string
	args      []string
	shown     string
}

// String is what the loop prints, and it is the line as the file wrote it —
// never the arguments actually passed. Those carry the token, and this output
// is what gets redirected to a log file and read weeks later: a secret that
// reaches a log is a secret to change, and nothing about a printed pass is
// worth that.
func (r run) String() string { return r.shown }

// defaults puts the loop's own -url, -token and -dry-run in front of the
// line's arguments. In front, because the flag package lets the last of a
// repeated flag win: a line naming its own url or token overrides the default
// instead of fighting it.
func (r *run) defaults(url, token string, dry bool) {
	head := []string{"-url", url}
	if token != "" {
		head = append(head, "-token", token)
	}
	if dry {
		head = append(head, "-dry-run")
	}
	r.args = append(head, r.args...)
}

// do performs one run. A failure is the run's own and never the loop's: the
// next pass tries again, and a direction that cannot reach the app now is
// usually one that can in five minutes.
func (r run) do() (int, error) {
	switch r.direction {
	case "move":
		return moveRun(r.args, flag.ContinueOnError)
	case "sync":
		return syncRun(r.args, flag.ContinueOnError)
	}
	return 0, fmt.Errorf("no such direction: %q", r.direction)
}

func loop(ctx context.Context, runs []run, period time.Duration) {
	stdout, stderr := out, errOut
	defer func() { out, errOut = stdout, stderr }()
	for {
		for _, r := range runs {
			if ctx.Err() != nil {
				return
			}
			// Each stream says for itself which run it is about, because the
			// two are often redirected to different files and a line in one
			// cannot lean on a line in the other.
			//
			// What was done gets a heading, printed the first time the run
			// writes anything — so a pass where nothing moved prints nothing
			// at all, while everything that is printed still arrives under a
			// time and a run. What went wrong is one line each, carrying the
			// same two facts inline: a failure is read on its own, often out
			// of a much longer file, and one that has to be traced upwards to
			// find out which list it was about is one nobody traces.
			at := time.Now().Format("2006-01-02 15:04:05")
			out = &headed{stdout, once(stdout, "=== "+at+"  "+r.String())}
			errOut = &prefixed{stderr, at + " " + r.String() + ": "}

			left, err := r.do()
			switch {
			case err != nil:
				fmt.Fprintf(errOut, "%v\n", err)
			case left > 0:
				fmt.Fprintf(errOut, "%d left behind\n", left)
			}
		}
		// the wait starts when the pass ends, so two passes can never overlap
		// however long one takes
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
// never again, so a run that moved three things is headed once.
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

// --- the config file ------------------------------------------------------

// readRuns reads the file into the runs it names, and refuses anything it
// cannot run: an unknown direction, a line with only a direction on it, a
// quote that is never closed. Every one of those is a typo that would
// otherwise be printed once a pass, forever.
func readRuns(path string) ([]run, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var runs []run
	for n, line := range strings.Split(string(body), "\n") {
		fields, err := words(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", n+1, err)
		}
		if len(fields) == 0 {
			continue
		}
		// a line may name the program before the direction, because the words
		// a person would type start with `./remindersync` — and a file whose
		// lines are "what you would type" has to mean that literally, or the
		// one thing it saves is remembering an exception to it
		if isProgram(fields[0]) {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			return nil, fmt.Errorf("line %d: no direction on it (move or sync)", n+1)
		}
		r := run{direction: fields[0], args: fields[1:], shown: shown(fields)}
		switch r.direction {
		case "move", "sync":
		default:
			return nil, fmt.Errorf("line %d: no such direction: %q (move or sync)", n+1, r.direction)
		}
		if len(r.args) == 0 {
			return nil, fmt.Errorf("line %d: %s needs -list", n+1, r.direction)
		}
		runs = append(runs, r)
	}
	if len(runs) == 0 {
		return nil, errors.New("that file names no runs")
	}
	return runs, nil
}

// isProgram reports a word that names this program rather than a direction:
// `remindersync`, `./remindersync`, or whatever path it was started from. It is
// matched by name and not by resolving anything, because the point is to read a
// line a person wrote, not to work out which file they meant.
func isProgram(word string) bool {
	name := strings.TrimSuffix(filepath.Base(word), ".exe")
	return name == "remindersync" || name == strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
}

// shown writes a config line back for the output, with any token in it hidden.
// A line may name its own, and printing it would put it in the log just as
// surely as the loop's own would.
func shown(fields []string) string {
	out := make([]string, 0, len(fields))
	hide := false
	for _, f := range fields {
		switch {
		case hide:
			out = append(out, "…")
			hide = false
		case f == "-token" || f == "--token":
			out = append(out, f)
			hide = true
		case strings.HasPrefix(f, "-token=") || strings.HasPrefix(f, "--token="):
			out = append(out, f[:strings.Index(f, "=")]+"=…")
		default:
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

// words splits one line the way a shell would: on whitespace, with quotes
// holding a value together, because `-q "#gc #sync"` is how the filter line is
// written everywhere else and a config file that could not hold it would be a
// second notation to remember.
//
// A `#` outside quotes starts a comment, and inside them it is a tag — which
// is the whole reason the quoting has to be understood rather than the line
// being cut at the first hash.
func words(line string) ([]string, error) {
	var (
		out   []string
		token strings.Builder
		open  bool // a token is being built, even if it is empty: `-q ""`
		quote rune
	)
	push := func() {
		if open {
			out = append(out, token.String())
			token.Reset()
			open = false
		}
	}
	for _, c := range line {
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
				continue
			}
			token.WriteRune(c)
		case c == '"' || c == '\'':
			quote = c
			open = true
		case c == '#':
			push()
			return out, nil
		case c == ' ' || c == '\t' || c == '\r':
			push()
		default:
			token.WriteRune(c)
			open = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("a %c that is never closed", quote)
	}
	push()
	return out, nil
}
