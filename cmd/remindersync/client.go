// The todoistik side: the two endpoints anything outside the app is given —
// capture on the way in, a view on the way out — under one bearer token.
package main

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

// errFatal marks the failures that will happen identically for every remaining
// item — a wrong token, an app that is not there — so a run can stop asking
// instead of failing item by item.
var errFatal = errors.New("fatal")

type client struct {
	base  string
	token string
	http  *http.Client
}

func newClient(base, token string) *client {
	return &client{base: base, token: token, http: &http.Client{Timeout: 20 * time.Second}}
}

// base trims the trailing slash, so that "http://host:8390/" and
// "http://host:8390" build the same URL.
func base(s string) string { return strings.TrimRight(strings.TrimSpace(s), "/") }

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

// viewItem is as much of an item as a reminder can hold. Every view is read
// through this one struct: an action carries a title, an inbox or someday item
// carries text, and the rest of what an item has — its tags, its context, its
// project — has nowhere to go in Reminders and is left where it belongs.
//
// The id is the one field that is not about what the item says: it is what the
// marker carries, and what a completion request names on the way back (see
// "The completion channel" in implementation.md).
type viewItem struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Text        string `json:"text"`
	Description string `json:"description"`
	DueDate     string `json:"dueDate"`
}

// view reads one view with the caller's own filter line, exactly as the read
// API offers it (design.md, "The read API"). The line is the same one typed on
// the screen — `#gc #sync` — because that is where the filter being mirrored
// was worked out.
func (c *client) view(name, query string) ([]viewItem, error) {
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
		Items []viewItem `json:"items"`
		Error string     `json:"error"`
	}
	decodeErr := json.Unmarshal(payload, &answer)

	if err := refused(resp, answer.Error, payload); err != nil {
		return nil, err
	}
	if decodeErr != nil {
		// a view that answers with counts rather than a list of items — the
		// weekly review does — is a mistake worth naming, not an empty run
		return nil, fmt.Errorf("%w: the %s view did not answer with a list of items", errFatal, name)
	}
	return answer.Items, nil
}

func (c *client) do(req *http.Request) ([]byte, *http.Response, error) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: cannot reach %s: %v", errFatal, c.base, err)
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
		return fmt.Errorf("%w: %s (set -token or TODOISTIK_TOKEN)", errFatal, said(msg, "unauthorized"))
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s", errFatal, said(msg, resp.Status))
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: the app answered %s: %s", errFatal, resp.Status, said(msg, strings.TrimSpace(string(payload))))
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
