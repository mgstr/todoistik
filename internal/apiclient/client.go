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
func (c *Client) View(name, query string) ([]Item, error) {
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

	var answer struct {
		Items []Item `json:"items"`
		Error string `json:"error"`
	}
	decodeErr := json.Unmarshal(payload, &answer)

	if err := refused(resp, answer.Error, payload); err != nil {
		return nil, err
	}
	if decodeErr != nil {
		// a view that answers with counts rather than a list of items — the
		// weekly review does — is a mistake worth naming, not an empty run
		return nil, fmt.Errorf("%w: the %s view did not answer with a list of items", ErrFatal, name)
	}
	return answer.Items, nil
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
