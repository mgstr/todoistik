package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

// The loop is driven at milliseconds rather than the minutes the flag is in:
// what is under test is that a pass follows a pass at all, and how long the
// wait is has no bearing on that.
const testPeriod = 10 * time.Millisecond

// captures stands the capture API up and hands back a channel of what arrived,
// which is also how a test waits for a pass to happen. Buffered, so a pass is
// never held open waiting to be read — the loop has to be free to go round
// again while the test is still looking at the last thing it did.
//
// answer is asked per capture, so a test can have the first one fail and the
// second one succeed.
func captures(t *testing.T, answer func(n int) (int, string)) (*httptest.Server, chan string) {
	t.Helper()
	got := make(chan string, 16)
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Text string `json:"text"`
		}
		payload, _ := io.ReadAll(r.Body)
		json.Unmarshal(payload, &body)
		code, status := answer(int(atomic.AddInt32(&n, 1)))
		if code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		got <- body.Text
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"` + status + `"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

// running starts the loop and hands back the stop, the two logs, and a wait
// that does not come back until the loop has actually stopped. The logs are
// only read after that wait, which is what makes reading them safe.
func running(t *testing.T, c config) (stop func(), logs func() (string, string)) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	out, errOut = &stdout, &stderr
	t.Cleanup(func() { out, errOut = os.Stdout, os.Stderr })

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		loop(ctx, c, testPeriod)
	}()
	return cancel, func() (string, string) {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("the loop did not stop when it was interrupted")
		}
		return stdout.String(), stderr.String()
	}
}

func waitFor(t *testing.T, got chan string, want string) {
	t.Helper()
	for {
		select {
		case text := <-got:
			if strings.HasPrefix(text, want) {
				return
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("no pass captured %q", want)
		}
	}
}

// What -period is for: a mail that arrives after a pass is taken by the next
// one, with nothing outside the program doing the repeating.
func TestTheNextPassTakesWhatArrivedAfterTheLast(t *testing.T) {
	user, addr := imapFor(t, mail("the first one", "marju@example.com", "<one@x>"))
	app, got := captures(t, func(int) (int, string) { return http.StatusOK, "accepted" })

	_, logs := running(t, cfg(app, addr))
	waitFor(t, got, "the first one")

	raw := mail("the second one", "marju@example.com", "<two@x>")
	if _, err := user.Append(testLabel, strings.NewReader(raw), &imap.AppendOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, got, "the second one")

	stdout, _ := logs()
	if n := count(t, user, testAllMail); n != 2 {
		t.Errorf("All Mail holds %d: every pass must leave the mail in the account", n)
	}
	// each pass is stamped, and stamped once however many mails it moved
	if !strings.Contains(stdout, "=== ") {
		t.Errorf("the log says no time: a pass in a log file nobody can date is a pass nobody can trace\n%s", stdout)
	}
}

// A failure belongs to the pass and not to the loop: the app being unreachable
// or refusing a token now is usually a thing that works in five minutes, and a
// process that exited on the first one would need something else watching it.
func TestAPassThatFailsDoesNotStopTheLoop(t *testing.T) {
	user, addr := imapFor(t, mail("the only one", "marju@example.com", "<one@x>"))
	app, got := captures(t, func(n int) (int, string) {
		if n == 1 {
			return http.StatusInternalServerError, ""
		}
		return http.StatusOK, "accepted"
	})

	_, logs := running(t, cfg(app, addr))
	waitFor(t, got, "the only one")

	stdout, stderr := logs()
	// the failed pass kept the label on, so the mail came back — and said so
	if !strings.Contains(stderr, "mailsync: ") {
		t.Errorf("a failed pass said nothing on stderr:\n%s", stderr)
	}
	if n := count(t, user, testLabel); n != 0 {
		t.Errorf("the label still holds %d after the pass that worked", n)
	}
	if n := count(t, user, testAllMail); n != 1 {
		t.Errorf("All Mail holds %d", n)
	}
	if !strings.Contains(stdout, "captured: the only one") {
		t.Errorf("the pass that worked said nothing:\n%s", stdout)
	}
}

// Silence is the good outcome. This runs every few minutes for months, and a
// log that says "nothing wearing the label" nine hundred times is a log nobody
// reads — which makes it a log that hides the line that mattered.
func TestAPassThatMovedNothingSaysNothing(t *testing.T) {
	user, addr := imapFor(t) // an empty label
	app, _ := captures(t, func(int) (int, string) { return http.StatusOK, "accepted" })

	_, logs := running(t, cfg(app, addr))
	time.Sleep(5 * testPeriod) // several passes, all of them empty

	stdout, stderr := logs()
	if stdout != "" || stderr != "" {
		t.Errorf("a quiet pass printed something:\nout: %q\nerr: %q", stdout, stderr)
	}
	if n := count(t, user, testLabel); n != 0 {
		t.Errorf("the label holds %d", n)
	}
}
