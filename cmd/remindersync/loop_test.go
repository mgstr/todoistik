package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"todoistik/internal/apiclient"
)

// A config file line is written the way the same words are typed at the
// prompt, quotes and all, so the splitting is the contract: `-q "#gc #sync"`
// has to survive, and the `#` in it must not start a comment.
func TestWords(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"a plain line", `move -list Inbox`, []string{"move", "-list", "Inbox"}},
		{"a quoted filter line keeps its hashes", `sync -list geocaching -q "#gc #sync"`,
			[]string{"sync", "-list", "geocaching", "-q", "#gc #sync"}},
		{"single quotes hold too", `sync -q '@home #car'`, []string{"sync", "-q", "@home #car"}},
		{"a quoted list name with a space", `move -list "к покупке"`, []string{"move", "-list", "к покупке"}},
		{"an unquoted hash is a comment", `move -list Inbox # the phone's inbox`,
			[]string{"move", "-list", "Inbox"}},
		{"a whole-line comment", `  # nothing here`, nil},
		{"a blank line", "   \t ", nil},
		{"an empty value stays a value", `sync -q ""`, []string{"sync", "-q", ""}},
		{"a hash inside a word is not a comment", `sync -q #gc`, []string{"sync", "-q"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := words(c.in)
			if err != nil {
				t.Fatalf("words(%q): %v", c.in, err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("words(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}

	if _, err := words(`sync -q "#gc`); err == nil {
		t.Error("an unclosed quote should not be read as a value")
	}
}

// A config file that is wrong is fatal before the first pass, not discovered on
// the fortieth — so every way of being wrong has to be caught here.
func TestReadRuns(t *testing.T) {
	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "remindersync.conf")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("the runs it names, in the order it names them", func(t *testing.T) {
		runs, err := readRuns(write(t, `
# the phone's inbox
move -list Inbox

sync -list geocaching -q "#gc #sync"
`))
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 2 {
			t.Fatalf("read %d runs, want 2", len(runs))
		}
		if runs[0].String() != "move -list Inbox" {
			t.Errorf("first run = %q", runs[0])
		}
		if runs[1].String() != `sync -list geocaching -q #gc #sync` {
			t.Errorf("second run = %q", runs[1])
		}
	})

	t.Run("a line may name the program before the direction", func(t *testing.T) {
		runs, err := readRuns(write(t, "./remindersync move -list Inbox\nremindersync sync -list geocaching\n/usr/local/bin/remindersync move -list Inbox\n"))
		if err != nil {
			t.Fatal(err)
		}
		if len(runs) != 3 {
			t.Fatalf("read %d runs, want 3", len(runs))
		}
		for _, r := range runs {
			if r.direction != "move" && r.direction != "sync" {
				t.Errorf("direction = %q", r.direction)
			}
			if strings.Contains(r.String(), "remindersync") {
				t.Errorf("%q: the program's own name should not be in what runs", r)
			}
		}
	})

	t.Run("a line naming only the program is refused", func(t *testing.T) {
		if _, err := readRuns(write(t, "./remindersync\n")); err == nil {
			t.Error("a line with no direction on it was accepted")
		}
	})

	t.Run("a token in the file is not printed back", func(t *testing.T) {
		runs, err := readRuns(write(t, "sync -list x -token s3cret\nmove -list y --token=s3cret\n"))
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range runs {
			if strings.Contains(r.String(), "s3cret") {
				t.Errorf("%q would put the token in the log", r)
			}
		}
	})

	for _, c := range []struct{ name, body string }{
		{"an unknown direction", "syncs -list geocaching"},
		{"a direction with nothing to work on", "sync"},
		{"an unclosed quote", `sync -list "geocaching`},
		{"a file naming nothing", "# only comments\n\n"},
	} {
		t.Run(c.name+" is refused", func(t *testing.T) {
			if _, err := readRuns(write(t, c.body)); err == nil {
				t.Errorf("readRuns accepted %q", c.body)
			}
		})
	}

	t.Run("a file that is not there is refused", func(t *testing.T) {
		if _, err := readRuns(filepath.Join(t.TempDir(), "nope.conf")); err == nil {
			t.Error("a missing config file is the one argument this command has")
		}
	})
}

// The loop's own -url and -token stand in for every line that does not name
// its own, and a line that does has to win — which is why they go in front.
func TestRunDefaults(t *testing.T) {
	r := run{direction: "sync", args: []string{"-list", "geocaching"}}
	r.defaults("http://host:8390", "tok", false)
	want := []string{"-url", "http://host:8390", "-token", "tok", "-list", "geocaching"}
	if !reflect.DeepEqual(r.args, want) {
		t.Fatalf("args = %q, want %q", r.args, want)
	}

	// the line's own url is later on the command line, so the flag package
	// gives it the last word
	own := run{direction: "sync", args: []string{"-list", "x", "-url", "http://other:8390"}}
	own.defaults("http://host:8390", "tok", true)
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	f := common(fs, "the list")
	fs.String("view", "next", "")
	if _, err := f.parse(fs, own.args); err != nil {
		t.Fatal(err)
	}
	if got := apiclient.Base(*f.base); got != "http://other:8390" {
		t.Errorf("url = %q, want the line's own", got)
	}
	if !*f.dry {
		t.Error("-dry-run from the loop did not reach the run")
	}
}
