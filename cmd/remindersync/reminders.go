// The Reminders side: one JavaScript-for-Automation script per operation,
// each run in one visit, because a round trip costs about five seconds
// whatever it carries.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// --- the reminder, flattened ---------------------------------------------

type reminder struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	Due       string `json:"due"`    // "2026-09-15 14:30", local
	AllDay    string `json:"allDay"` // "2026-09-15", for a day carrying no time
	Completed bool   `json:"completed"`
	DoneAt    string `json:"doneAt"` // "2026-09-08T14:30", when it was ticked off
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

// dueDay is the day the reminder is due, whatever time it carries. It is what
// a todoistik due date can be compared against: an action's due date is a day
// and nothing finer, so a reminder due at 14:30 on the right day is already
// saying what the action says (implementation.md, "Reminders, both ways").
func (r reminder) dueDay() string {
	if r.AllDay != "" {
		return r.AllDay
	}
	if day, _, ok := strings.Cut(r.Due, " "); ok {
		return day
	}
	return r.Due
}

// --- the scripts ----------------------------------------------------------

// Every property is read for the whole list in one go and the reminders sorted
// out here, rather than asked for with a `whose` filter or item by item. Each
// round trip to Reminders costs about five seconds whatever it carries, and a
// filtered collection re-runs its filter on every property read — which is the
// difference between a read that takes half a minute and one that never ends.
//
// Completed reminders come back too, with a flag and the moment they were
// ticked, because the two directions want opposite things from them: the
// import skips them, and the export reads one as a completion request to file
// (see "The completion channel" in implementation.md).
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
  var dones = rs.completionDate();
  var out = [];
  for (var i = 0; i < ids.length; i++) {
    var d = dues[i], due = '', allDay = '';
    if (d) {
      if (d.getHours() === 0 && d.getMinutes() === 0 && d.getSeconds() === 0) {
        allDay = day(d);
      } else {
        due = day(d) + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes());
      }
    }
    var c = dones[i];
    out.push({
      id: ids[i] || '',
      name: names[i] || '',
      body: bodies[i] || '',
      due: due,
      allDay: allDay,
      completed: !!done[i],
      doneAt: c ? (day(c) + 'T' + pad(c.getHours()) + ':' + pad(c.getMinutes())) : ''
    });
  }
  return JSON.stringify(out);
}
`

// createScript adds every reminder named, in one visit. A due date is written
// as local midnight, which is the same thing the read reads back as a day
// without a time — the app's own way of storing a reminder set for a date.
const createScript = `
function run() {
  var wanted = %s, listName = %s;
  var app = Application('Reminders');
  var lists = app.lists.whose({ name: listName })();
  if (lists.length === 0) { throw new Error('the list is gone: ' + listName) }
  var target = lists[0];
  var failed = [];
  for (var i = 0; i < wanted.length; i++) {
    var w = wanted[i];
    try {
      var props = { name: w.title };
      if (w.body) { props.body = w.body }
      if (w.due) {
        var p = w.due.split('-');
        props.dueDate = new Date(+p[0], +p[1] - 1, +p[2]);
      }
      target.reminders.push(app.Reminder(props));
    } catch (e) { failed.push({ id: w.title, why: e.message }) }
  }
  return JSON.stringify(failed);
}
`

// updateScript writes the fields that moved onto reminders already on the
// list, found by the ids read earlier. The title is one of them: what
// identifies a reminder is the marker inside it, not the words, so renaming an
// item in todoistik renames its reminder instead of stranding it.
const updateScript = `
function run() {
  var wanted = %s, listName = %s;
  var app = Application('Reminders');
  var lists = app.lists.whose({ name: listName })();
  if (lists.length === 0) { throw new Error('the list is gone: ' + listName) }
  var rs = lists[0].reminders;
  var ids = rs.id();
  var at = {};
  for (var i = 0; i < ids.length; i++) { at[ids[i]] = i }
  var failed = [];
  for (var j = 0; j < wanted.length; j++) {
    var w = wanted[j];
    if (at[w.id] === undefined) {
      failed.push({ id: w.id, why: 'it is no longer on the list' });
      continue;
    }
    try {
      var r = rs[at[w.id]];
      if (w.title) { r.name = w.title }
      if (w.body) { r.body = w.body }
      if (w.due) {
        var p = w.due.split('-');
        r.dueDate = new Date(+p[0], +p[1] - 1, +p[2]);
      }
    } catch (e) { failed.push({ id: w.id, why: e.message }) }
  }
  return JSON.stringify(failed);
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

// --- running them ---------------------------------------------------------

// readReminders answers with everything on the list, completed included.
func readReminders(list string) ([]reminder, error) {
	out, err := osascript(fmt.Sprintf(readScript, jsString(list)))
	if err != nil {
		return nil, err
	}
	var rems []reminder
	if err := jsonUnmarshal(out, &rems); err != nil {
		return nil, err
	}
	return rems, nil
}

// writeFailure names the reminder a write could not touch — by id where the reminder
// was already on the list, and by title where it was being created and has no
// id yet — and why, so the summary can say which item was left behind rather
// than only how many.
type writeFailure struct {
	ID  string `json:"id"`
	Why string `json:"why"`
}

func createReminders(list string, want []desired) ([]writeFailure, error) {
	if len(want) == 0 {
		return nil, nil
	}
	return scriptOn(createScript, list, want)
}

func updateReminders(list string, want []update) ([]writeFailure, error) {
	if len(want) == 0 {
		return nil, nil
	}
	return scriptOn(updateScript, list, want)
}

func deleteReminders(list string, ids []string) ([]writeFailure, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return scriptOn(deleteScript, list, ids)
}

// scriptOn runs one write script over one payload and reads back what it could
// not do. All three writes have that shape, and a failure that is not read
// back is an item silently dropped. An empty payload makes no visit: each one
// costs about five seconds, and a run with nothing to change should cost none
// of them.
func scriptOn(script, list string, payload any) ([]writeFailure, error) {
	wanted, err := jsValue(payload)
	if err != nil {
		return nil, err
	}
	out, err := osascript(fmt.Sprintf(script, wanted, jsString(list)))
	if err != nil {
		return nil, err
	}
	var failures []writeFailure
	if err := jsonUnmarshal(out, &failures); err != nil {
		return nil, err
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
