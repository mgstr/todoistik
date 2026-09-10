// Package request holds the grammar of a completion request: the one line a
// program that mirrors items out of the app writes when it finds one of them
// finished over there (design.md, "Completion requests").
//
// It is a package of its own because the line is written by one program and
// read by another. The app has to know what the line means, and the utility
// has to be able to write one without linking the database driver to do it —
// so the grammar lives where both can have it, and neither owns a second copy
// that can drift.
//
// It knows nothing about items, storage or HTTP: what a request means, and
// whether the item it names is still there, is the app's business (see
// internal/app, CompletionRequest).
package request

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Prefix is what makes a captured line a request rather than a thought, and it
// names the source as its last word: `Completion request from reminders`. A
// second kind of sync files its own source instead of being told apart by
// guesswork.
const Prefix = "Completion request from "

// Mark is the sigil the machine-readable parts wear. `@` and `#` are the app's
// vocabulary, read against the remembered lists; `::` is read by nothing else,
// so a request cannot half-parse into an item's fields, and a marker cannot
// become a tag on the way back in.
const Mark = "::"

// TimeFormat writes the moment the work was finished as one word, local. It is
// one word because everything that reads this line splits it on whitespace,
// and it carries a time because a bare day would mean midnight — a fiction the
// Archive would then repeat forever (design.md, "Completion").
const TimeFormat = "2006-01-02T15:04"

// Line is one request, in either direction: parsed out of a captured line, or
// written into one.
type Line struct {
	Source string    // "reminders"
	ItemID int64     // the item the request is about
	At     time.Time // when it was finished over there, in the source's own clock
	Name   string    // what the item was called there
}

// Write formats one request. The name comes last because it is the only part
// with spaces in it, which is what lets the rest be read without quoting.
func Write(l Line) string {
	return fmt.Sprintf("%s%s %s%d %s%s %s",
		Prefix, l.Source,
		Mark, l.ItemID,
		Mark, l.At.Format(TimeFormat),
		strings.Join(strings.Fields(l.Name), " "))
}

// Parse reads a captured line as a request, and reports whether it is one.
//
// A line that starts like a request but does not carry both an item and a time
// is **not** a request: it is an ordinary capture that happens to begin with
// those words, and it is processed as one. Half-reading it would mean a screen
// offering to complete something it could not name.
//
// It is the first line of the capture that is read, and only the first. An
// item's text may run to more than one (design.md, "Inbox item"), so the
// grammar has to name the line it reads or acquire a second meaning the first
// time anything files a request with a body under it.
func Parse(text string) (Line, bool) {
	text, _, _ = strings.Cut(text, "\n")
	rest, ok := cutPrefix(text)
	if !ok {
		return Line{}, false
	}
	fields := strings.Fields(rest)
	if len(fields) < 3 {
		return Line{}, false
	}
	l := Line{Source: fields[0]}

	id, ok := mark(fields[1])
	if !ok {
		return Line{}, false
	}
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil || n <= 0 {
		return Line{}, false
	}
	l.ItemID = n

	stamp, ok := mark(fields[2])
	if !ok {
		return Line{}, false
	}
	// read without a zone: the line is written in the clock the source was
	// looking at, and whose clock that is belongs to the reader, which holds
	// the one configured timezone
	at, err := time.ParseInLocation(TimeFormat, stamp, time.UTC)
	if err != nil {
		return Line{}, false
	}
	l.At = at
	l.Name = strings.Join(fields[3:], " ")
	return l, true
}

// In reads the request's moment in the reader's own timezone. The line carries
// no zone because both sides are one person's machines; which zone it is meant
// in is the app's single configured one (README, "Run").
func (l Line) In(loc *time.Location) time.Time {
	y, m, d := l.At.Date()
	return time.Date(y, m, d, l.At.Hour(), l.At.Minute(), 0, 0, loc)
}

// Marker is the identity a mirrored item carries where the other program can
// keep it — `(::15)` — so that the thing coming back names an item rather than
// a title that may since have been rewritten.
func Marker(itemID int64) string { return "(" + Mark + strconv.FormatInt(itemID, 10) + ")" }

// ItemFromMarker reads the id back out of a name ending in a marker, and
// reports whether there was one.
func ItemFromMarker(name string) (int64, bool) {
	name = strings.TrimSpace(name)
	if !strings.HasSuffix(name, ")") {
		return 0, false
	}
	open := strings.LastIndex(name, "("+Mark)
	if open < 0 {
		return 0, false
	}
	n, err := strconv.ParseInt(name[open+len("("+Mark):len(name)-1], 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// WithoutMarker is the name as a person wrote it, with the identity taken off
// the end. It is what a request reports, because the marker is the app's own
// bookkeeping and reading it back in the inbox twice says nothing.
func WithoutMarker(name string) string {
	if _, ok := ItemFromMarker(name); !ok {
		return strings.TrimSpace(name)
	}
	open := strings.LastIndex(name, "("+Mark)
	return strings.TrimSpace(name[:open])
}

func cutPrefix(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if len(text) < len(Prefix) || !strings.EqualFold(text[:len(Prefix)], Prefix) {
		return "", false
	}
	return text[len(Prefix):], true
}

func mark(field string) (string, bool) {
	if !strings.HasPrefix(field, Mark) {
		return "", false
	}
	return field[len(Mark):], true
}
