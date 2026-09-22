// Package apiclient is the app as seen from outside it: the two endpoints
// anything out there is given — capture on the way in, a view on the way out —
// under one bearer token.
//
// It is a package of its own because more than one program needs it. Both
// utilities in cmd/ put items into the inbox, and the part of that worth
// getting right is not the POST — it is telling apart what will fail the same
// way for everything left in the run from what is about this one item. A
// second copy of that judgement would drift, and the first thing to go would
// be a run that keeps trying after the token turned out to be wrong.
package apiclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrFatal marks the failures that will happen identically for every remaining
// item — a wrong token, an app that is not there — so a run can stop asking
// instead of failing item by item.
var ErrFatal = errors.New("fatal")

type Client struct {
	base  string
	token string
	http  *http.Client
}

func New(base, token string) *Client {
	return &Client{base: base, token: token, http: &http.Client{Timeout: 20 * time.Second}}
}

// Base trims the trailing slash, so that "http://host:8390/" and
// "http://host:8390" build the same URL.
func Base(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }

// Capture posts one line and reports what the app did with it: "accepted" or
// "duplicate". Both mean the app has the text; anything else is an error, and
// the reminder stays where it is.
func (c *Client) Capture(text string) (string, error) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.base+"/api/capture", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	payload, resp, err := c.do(req)
	if err != nil {
		return "", err
	}

	var answer struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	json.Unmarshal(payload, &answer)

	if err := refused(resp, answer.Error, payload); err != nil {
		return "", err
	}
	if answer.Status == "" {
		answer.Status = "accepted"
	}
	return answer.Status, nil
}

// Item is as much of an item as a reminder can hold. Every view is read
// through this one struct: an action carries a title, an inbox or someday item
// carries text, and the rest of what an item has — its tags, its context, its
// project — has nowhere to go in Reminders and is left where it belongs.
//
// The id is the one field that is not about what the item says: it is what the
// marker carries, and what a completion request names on the way back (see
// "The completion channel" in implementation.md).
type Item struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Text        string `json:"text"`
	Description string `json:"description"`
	DueDate     string `json:"dueDate"`
}

// View reads one view with the caller's own filter line, exactly as the read
// API offers it (design.md, "The read API"). The line is the same one typed on
// the screen — `#gc #sync` — because that is where the filter being mirrored
// was worked out.
//
// A line the app could not read whole is refused here rather than answered:
// what this returns is mirrored somewhere — a list on a phone, a reply in a
// chat — and a mistyped `#cra` would put the *whole* view there, looking
// exactly like the filtered one. A caller that wants the partial answer and
// the problems with it reads them itself, through Read.
func (c *Client) View(name, query string) ([]Item, error) {
	answer, err := c.Read(name, query)
	if err != nil {
		return nil, err
	}
	if len(answer.Problems) > 0 {
		var parts []string
		for _, p := range answer.Problems {
			parts = append(parts, p.String())
		}
		// fatal: a filter line comes from a flag or a config file, so the next
		// item, and the next run, would be refused for the same reason
		return nil, fmt.Errorf("%w: the filter line was not read whole: %s", ErrFatal, strings.Join(parts, ", "))
	}
	items, ok := itemsOf(answer.Items)
	if !ok {
		// a view that answers with counts rather than a list of items — the
		// weekly review does — is a mistake worth naming, not an empty run
		return nil, fmt.Errorf("%w: the %s view did not answer with a list of items", ErrFatal, name)
	}
	return items, nil
}

// Answer is a view as the read API sends it, before anything is made of it.
// View flattens it into what a Reminders list can hold; a caller that shows
// more than a list — Today's two groups, a project marked stalled — reads the
// items itself.
type Answer struct {
	Items json.RawMessage `json:"items"`
	// Problems is what the app left out of the filter line, because it could
	// not read it: a name on no remembered list, a window that is not one.
	Problems []Problem `json:"problems"`
	Error    string    `json:"error"`
}

type Problem struct {
	Token string `json:"token"` // as written, "#cra"
	Kind  string `json:"kind"`  // "tag", "context", "second-context", "not-a-filter", "window", "not-in-view"
}

// String says what is wrong with the token in the words a person reads it in,
// which is one wording for every caller: the same sentence reaches a terminal
// and a chat.
func (p Problem) String() string {
	switch p.Kind {
	case "tag":
		return p.Token + " is no tag"
	case "context":
		return p.Token + " is no context"
	case "second-context":
		return p.Token + " is a second context, and an action has one"
	case "not-a-filter":
		return p.Token + " is not a filter"
	case "window":
		return p.Token + " is not a window"
	case "not-in-view":
		return p.Token + " is not a filter this view has"
	}
	return p.Token + " was not read"
}

// Read reads one view with a filter line and hands the answer back whole.
func (c *Client) Read(name, query string) (*Answer, error) {
	u := c.base + "/api/view/" + url.PathEscape(name)
	if strings.TrimSpace(query) != "" {
		u += "?" + url.Values{"q": {query}}.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	payload, resp, err := c.do(req)
	if err != nil {
		return nil, err
	}

	var answer Answer
	json.Unmarshal(payload, &answer)

	if err := refused(resp, answer.Error, payload); err != nil {
		return nil, err
	}
	return &answer, nil
}

// ReadText reads one view as plain text, exactly as the read API spells it
// (design.md, "The read API"): the header saying the view, the day and the
// filter, then one item per line with whatever body it carries under it.
//
// Unlike View, a filter line the app could only partly read is not refused
// here. The text answer says what it left out in its own header — `not read:
// #cra is no tag` — so a caller handing the text on is handing the warning on
// with it, which is the thing View has to refuse for: a Reminders list has
// nowhere to put that sentence, and a block of text is nothing but somewhere
// to put it.
func (c *Client) ReadText(name, query string) (string, error) {
	q := url.Values{"format": {"text"}}
	if strings.TrimSpace(query) != "" {
		q.Set("q", query)
	}
	return c.text(c.base + "/api/view/" + url.PathEscape(name) + "?" + q.Encode())
}

// ContextText reads every view at once, as text. archive is how far back the
// Archive section reaches — one of the windows `completed:` takes, or "none" —
// and empty leaves the app to its own default.
func (c *Client) ContextText(archive string) (string, error) {
	q := url.Values{"format": {"text"}}
	if strings.TrimSpace(archive) != "" {
		q.Set("archive", strings.TrimSpace(archive))
	}
	return c.text(c.base + "/api/context?" + q.Encode())
}

// text is the read itself. A refusal arrives as JSON even when text was asked
// for — the app cannot know how to spell an error in a format it has not
// agreed to answer in — so the error is read the way every other one here is.
func (c *Client) text(u string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	payload, resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	var answer struct {
		Error string `json:"error"`
	}
	json.Unmarshal(payload, &answer)
	if err := refused(resp, answer.Error, payload); err != nil {
		return "", err
	}
	return string(payload), nil
}

// itemsOf reads a view's items as one list, whichever shape they came in.
//
// Every view answers with a list but "Today", which answers with its two
// groups — what has run out of time and what was picked — because the screen
// shows them apart (design.md, "Today"). A list has nowhere to show a group,
// so here they are one list again: out of time first, the way the screen
// orders them, and an action that is both due and picked once. It appears
// twice on the screen because each group is answering its own question; on a
// list the second copy would be a second reminder for one action, and ticking
// one would leave the other saying it is still to do.
func itemsOf(raw json.RawMessage) ([]Item, bool) {
	var list []Item
	if json.Unmarshal(raw, &list) == nil && (list != nil || string(raw) == "null") {
		return list, true
	}
	// the groups are named rather than guessed at: an empty one arrives as
	// null, and an answer that merely is an object — the review's counts — has
	// to stay a refusal
	var groups map[string]json.RawMessage
	if json.Unmarshal(raw, &groups) != nil {
		return nil, false
	}
	var all []Item
	for _, g := range []string{"outOfTime", "picked"} {
		part, named := groups[g]
		var its []Item
		if !named || json.Unmarshal(part, &its) != nil {
			return nil, false
		}
		all = append(all, its...)
	}
	seen := map[int64]bool{}
	for _, it := range all {
		if seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		list = append(list, it)
	}
	return list, true
}

func (c *Client) do(req *http.Request) ([]byte, *http.Response, error) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: cannot reach %s: %v", ErrFatal, c.base, err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return payload, resp, nil
}

// refused turns the app's answer into the one distinction that matters here:
// what will fail the same way for everything else in the run, and what is
// about this request alone.
func refused(resp *http.Response, msg string, payload []byte) error {
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("%w: %s (set -token or TODOISTIK_TOKEN)", ErrFatal, said(msg, "unauthorized"))
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrFatal, said(msg, resp.Status))
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: the app answered %s: %s", ErrFatal, resp.Status, said(msg, strings.TrimSpace(string(payload))))
	case resp.StatusCode >= 400:
		return fmt.Errorf("the app refused it: %s", said(msg, resp.Status))
	}
	return nil
}

func said(msg, fallback string) string {
	if strings.TrimSpace(msg) != "" {
		return msg
	}
	return fallback
}
