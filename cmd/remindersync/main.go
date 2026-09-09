// Command remindersync empties a macOS Reminders list into todoistik's inbox:
// one capture per reminder, and a reminder is deleted only once the app has
// said it has the text.
//
// It talks to the app the way anything outside it does — POST /api/capture,
// bearer token — rather than opening the database, because the capture API is
// the only way in by design (design.md, "Capture") and going around it would
// skip the duplicate collapse and the audit entry.
//
// It talks to Reminders through osascript, because Reminders has no other
// interface: EventKit would mean a second language and its own signed bundle
// for the privacy prompt, and the scripting interface holds the same data.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	list := flag.String("list", "", "the Reminders list to empty (required)")
	base := flag.String("url", env("TODOISTIK_URL", "http://127.0.0.1:8390"), "the running app's base URL")
	token := flag.String("token", os.Getenv("TODOISTIK_TOKEN"), "bearer token; empty for a server started without one")
	dry := flag.Bool("dry-run", false, "print the lines that would be captured; capture nothing, delete nothing")
	flag.Parse()

	if strings.TrimSpace(*list) == "" {
		fmt.Fprintln(os.Stderr, "remindersync: -list is required")
		flag.Usage()
		os.Exit(2)
	}

	left, err := run(*list, strings.TrimRight(*base, "/"), *token, *dry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "remindersync: %v\n", err)
		os.Exit(1)
	}
	// anything still on the list is a failure worth an exit code: this runs
	// unattended, where the summary line is not read but the status is
	if left > 0 {
		os.Exit(1)
	}
}

// run carries one list across and returns how many reminders it had to leave
// behind.
//
// Every capture happens first, and the deletions follow in one pass, because
// each round trip to Reminders costs seconds: deleting one at a time would
// make a twenty-item list take minutes. The order still never loses anything —
// a reminder is only ever deleted after the app answered for it, and a run cut
// short between the two leaves the reminder in place, where the next run
// re-captures it, the app calls it a duplicate, and it is deleted then.
func run(list, base, token string, dry bool) (int, error) {
	rems, err := readReminders(list)
	if err != nil {
		return 0, err
	}
	if len(rems) == 0 {
		fmt.Printf("nothing open on %q\n", list)
		return 0, nil
	}

	c := &client{base: base, token: token, http: &http.Client{Timeout: 20 * time.Second}}

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
			fmt.Printf("kept: a reminder with no text (id %s)\n", r.ID)
			left++
			continue
		}
		if dry {
			fmt.Printf("would capture: %s\n", text)
			continue
		}

		status, err := c.capture(text)
		if err != nil {
			// a wrong token or an unreachable app fails identically for every
			// remaining reminder, so stop asking — but still delete what the
			// app already took, or the run would leave those to come back
			if errors.Is(err, errFatal) {
				fatal = err
				left += len(rems) - len(done) - left
				break
			}
			fmt.Printf("kept: %s (%v)\n", text, err)
			left++
			continue
		}
		done = append(done, delivered{id: r.ID, text: text, duplicate: status == "duplicate"})
	}

	if dry {
		fmt.Printf("\n%d open on %q; nothing was captured or deleted\n", len(rems), list)
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
			fmt.Printf("captured but still on the list: %s (%s)\n", d.text, stuck[d.id])
			left++
		case d.duplicate:
			// the identical line was already sitting in the inbox, so the
			// reminder had nothing left to carry — but it is not a silent
			// drop, or a reminder that vanished would look like one that moved
			fmt.Printf("duplicate (already in the inbox): %s\n", d.text)
			duplicate++
		default:
			fmt.Printf("captured: %s\n", d.text)
			captured++
		}
	}

	fmt.Printf("\n%d captured, %d duplicate, %d left on the list\n", captured, duplicate, left)
	return left, fatal
}

// --- the reminder, flattened ---------------------------------------------

type reminder struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Body   string `json:"body"`
	Due    string `json:"due"`    // "2026-09-15 14:30", local
	AllDay string `json:"allDay"` // "2026-09-15", for a day carrying no time
}

// captureText writes a reminder as the one line todoistik captures. Everything
// the reminder holds goes in, because the reminder is deleted straight after:
// a due date or a note left out here is lost, and the inbox is raw text by
// design (design.md, "Capture") with no field to put them in instead.
//
// Nothing is written as a token — no `#tag`, no `@context`. A captured line is
// prose until someone processes it, and inventing notation here would be this
// utility deciding what an item means, which is the one thing capture must
// never require.
func captureText(r reminder) string {
	text := collapse(r.Name)
	if due := r.dueText(); due != "" {
		text += " (due " + due + ")"
	}
	if body := collapse(r.Body); body != "" {
		text += " — " + body
	}
	return strings.TrimSpace(text)
}

// dueText prefers the day without a time, which is what a reminder set for a
// date rather than a moment carries. Writing "00:00" for those would be
// inventing a deadline the reminder never had.
func (r reminder) dueText() string {
	if r.AllDay != "" {
		return r.AllDay
	}
	return r.Due
}

// collapse puts a multi-line note on one line: the inbox is a list of lines,
// and a note's own line breaks are not information worth breaking that.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// --- the app --------------------------------------------------------------

var errFatal = errors.New("fatal")

type client struct {
	base  string
	token string
	http  *http.Client
}

// capture posts one line and reports what the app did with it: "accepted" or
// "duplicate". Both mean the app has the text; anything else is an error, and
// the reminder stays where it is.
func (c *client) capture(text string) (string, error) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.base+"/api/capture", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: cannot reach %s: %v", errFatal, c.base, err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var answer struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	json.Unmarshal(payload, &answer)

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", fmt.Errorf("%w: %s (set -token or TODOISTIK_TOKEN)", errFatal, said(answer.Error, "unauthorized"))
	case resp.StatusCode >= 500:
		return "", fmt.Errorf("%w: the app answered %s: %s", errFatal, resp.Status, said(answer.Error, strings.TrimSpace(string(payload))))
	case resp.StatusCode >= 400:
		return "", fmt.Errorf("the app refused it: %s", said(answer.Error, resp.Status))
	}
	if answer.Status == "" {
		answer.Status = "accepted"
	}
	return answer.Status, nil
}

func said(msg, fallback string) string {
	if strings.TrimSpace(msg) != "" {
		return msg
	}
	return fallback
}

// --- Reminders ------------------------------------------------------------

// Every property is read for the whole list in one go and the open ones picked
// out here, rather than asked for with a `whose` filter or item by item. Each
// round trip to Reminders costs about five seconds whatever it carries, and a
// filtered collection re-runs its filter on every property read — which is the
// difference between a read that takes half a minute and one that never ends.
//
// A due date is written as a day alone when it falls exactly on local
// midnight, which is how a reminder set for a date rather than a moment is
// stored. Its `allDayDueDate` would say so directly, but that property cannot
// be read over this interface at all — in bulk or one at a time.
const readScript = `
function pad(n) { return (n < 10 ? '0' : '') + n }
function day(d) { return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) }
function run() {
  var listName = %s;
  var app = Application('Reminders');
  var lists = app.lists.whose({ name: listName })();
  if (lists.length === 0) {
    throw new Error('no Reminders list named "' + listName + '"; the lists are: ' + app.lists.name().join(', '));
  }
  var rs = lists[0].reminders;
  var ids = rs.id(), names = rs.name(), done = rs.completed(), bodies = rs.body(), dues = rs.dueDate();
  var out = [];
  for (var i = 0; i < ids.length; i++) {
    if (done[i]) { continue }
    var d = dues[i], due = '', allDay = '';
    if (d) {
      if (d.getHours() === 0 && d.getMinutes() === 0 && d.getSeconds() === 0) {
        allDay = day(d);
      } else {
        due = day(d) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes());
      }
    }
    out.push({
      id: ids[i] || '',
      name: names[i] || '',
      body: bodies[i] || '',
      due: due,
      allDay: allDay
    });
  }
  return JSON.stringify(out);
}
`

// deleteScript removes every reminder named, in one visit. It finds them by
// the ids read earlier, and deletes from the back of the list forward, so that
// removing one does not shift the position of another still to go.
const deleteScript = `
function run() {
  var wanted = %s, listName = %s;
  var app = Application('Reminders');
  var lists = app.lists.whose({ name: listName })();
  if (lists.length === 0) { throw new Error('the list is gone: ' + listName) }
  var rs = lists[0].reminders;
  var ids = rs.id();
  var at = {};
  for (var i = 0; i < ids.length; i++) { at[ids[i]] = i }
  var failed = [], targets = [];
  for (var j = 0; j < wanted.length; j++) {
    if (at[wanted[j]] === undefined) {
      failed.push({ id: wanted[j], why: 'it is no longer on the list' });
    } else {
      targets.push(at[wanted[j]]);
    }
  }
  targets.sort(function (a, b) { return b - a });
  for (var k = 0; k < targets.length; k++) {
    try { app.delete(rs[targets[k]]) } catch (e) { failed.push({ id: ids[targets[k]], why: e.message }) }
  }
  return JSON.stringify(failed);
}
`

func readReminders(list string) ([]reminder, error) {
	out, err := osascript(fmt.Sprintf(readScript, jsString(list)))
	if err != nil {
		return nil, err
	}
	var rems []reminder
	if err := json.Unmarshal([]byte(out), &rems); err != nil {
		return nil, fmt.Errorf("cannot read what Reminders answered: %v", err)
	}
	return rems, nil
}

type deleteFailure struct {
	ID  string `json:"id"`
	Why string `json:"why"`
}

func deleteReminders(list string, ids []string) ([]deleteFailure, error) {
	wanted, err := json.Marshal(ids)
	if err != nil {
		return nil, err
	}
	out, err := osascript(fmt.Sprintf(deleteScript, wanted, jsString(list)))
	if err != nil {
		return nil, err
	}
	var failures []deleteFailure
	if err := json.Unmarshal([]byte(out), &failures); err != nil {
		return nil, fmt.Errorf("cannot read what Reminders answered: %v", err)
	}
	return failures, nil
}

// osascript runs one JavaScript-for-Automation script, fed on stdin so there
// is no temporary file to leave behind.
func osascript(script string) (string, error) {
	cmd := exec.Command("osascript", "-l", "JavaScript")
	cmd.Stdin = strings.NewReader(script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", errors.New(scriptError(stderr.String(), err))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// scriptError unwraps what osascript prints — "execution error: Error: Error:
// the message (-2700)" — down to the message, and names the one failure a
// person has to go and fix somewhere else.
func scriptError(stderr string, err error) string {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		return err.Error()
	}
	if i := strings.Index(msg, "execution error: "); i >= 0 {
		msg = msg[i+len("execution error: "):]
	}
	for strings.HasPrefix(msg, "Error: ") {
		msg = strings.TrimPrefix(msg, "Error: ")
	}
	if i := strings.LastIndex(msg, " (-"); i >= 0 && strings.HasSuffix(msg, ")") {
		msg = msg[:i]
	}
	if strings.Contains(msg, "-1743") || strings.Contains(strings.ToLower(msg), "not authorized") {
		msg += "\n  macOS has not granted this program control of Reminders." +
			"\n  System Settings → Privacy & Security → Automation, then tick Reminders" +
			"\n  under the terminal you are running it from."
	}
	return msg
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
