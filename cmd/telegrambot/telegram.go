package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// telegramBase is the Bot API's address, and the one seam a test replaces.
var telegramBase = "https://api.telegram.org"

// errTelegramFatal marks what will fail identically on every retry — a token
// Telegram does not know, another process already polling the same bot — so
// the run stops instead of retrying forever into a log nobody reads.
var errTelegramFatal = errors.New("fatal")

type telegram struct {
	token string
	http  *http.Client
}

// pollSeconds is how long one getUpdates call is held open waiting for a
// message. Long enough that an idle bot costs one request a minute; the HTTP
// timeout sits above it so a held call is never mistaken for a dead one.
const pollSeconds = 50

func newTelegram(token string) *telegram {
	return &telegram{token: token, http: &http.Client{Timeout: (pollSeconds + 20) * time.Second}}
}

type update struct {
	ID      int64    `json:"update_id"`
	Message *message `json:"message"`
}

type message struct {
	Chat struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	} `json:"chat"`
	Text string `json:"text"`
}

// updates waits for what arrived after offset. Handing the next offset back is
// also how Telegram is told the earlier ones were received: until then it
// keeps them, and a bot that was down finds them waiting.
func (t *telegram) updates(ctx context.Context, offset int64, wait int) ([]update, error) {
	var out []update
	err := t.call(ctx, "getUpdates", map[string]any{
		"offset":  offset,
		"timeout": wait,
		// an edit is not a new capture, and a capture that changed after it
		// landed would be a second item with no link to the first
		"allowed_updates": []string{"message"},
	}, &out)
	return out, err
}

// send writes one message as plain text. Plain, because an item's title is
// whatever was typed and Markdown would read `_` and `*` in it as formatting
// and refuse the message over an unbalanced one.
func (t *telegram) send(ctx context.Context, chat int64, text string) error {
	body := map[string]any{
		"chat_id": chat,
		"text":    text,
		// a list with a link in one item would otherwise end in a preview
		// card bigger than the list
		"link_preview_options": map[string]bool{"is_disabled": true},
	}
	for attempt := 0; ; attempt++ {
		err := t.call(ctx, "sendMessage", body, nil)
		var limited *rateLimited
		if !errors.As(err, &limited) || attempt == 3 {
			return err
		}
		// a long view split over several messages can outrun the per-chat
		// limit; Telegram says how long to wait, and waiting loses nothing
		select {
		case <-time.After(limited.wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// setCommands fills the menu the app's `/` opens, so the views are picked from
// a list rather than spelled on a phone keyboard.
func (t *telegram) setCommands(ctx context.Context, views []view) error {
	var cmds []map[string]string
	for _, v := range views {
		cmds = append(cmds, map[string]string{"command": v.command, "description": v.title})
	}
	return t.call(ctx, "setMyCommands", map[string]any{"commands": cmds}, nil)
}

type rateLimited struct{ wait time.Duration }

func (r *rateLimited) Error() string { return fmt.Sprintf("rate limited for %v", r.wait) }

func (t *telegram) call(ctx context.Context, method string, params any, result any) error {
	payload, err := json.Marshal(params)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, telegramBase+"/bot"+t.token+"/"+method, bytes.NewReader(payload))
	if err != nil {
		return t.redact(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.http.Do(req)
	if err != nil {
		// net/http puts the URL in its errors, and the URL holds the token
		return t.redact(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	var answer struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return fmt.Errorf("telegram %s: %s", method, resp.Status)
	}
	switch {
	case answer.OK:
		if result != nil {
			return json.Unmarshal(answer.Result, result)
		}
		return nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: telegram does not know this bot token (%s)", errTelegramFatal, answer.Description)
	case resp.StatusCode == http.StatusConflict:
		// another process polling the same bot, or a webhook set on it: the
		// two would take turns stealing each other's messages
		return fmt.Errorf("%w: telegram %s: %s", errTelegramFatal, method, answer.Description)
	case resp.StatusCode == http.StatusTooManyRequests:
		return &rateLimited{wait: time.Duration(max(answer.Parameters.RetryAfter, 1)) * time.Second}
	}
	return fmt.Errorf("telegram %s: %s", method, t.redactText(answer.Description))
}

func (t *telegram) redact(err error) error { return errors.New(t.redactText(err.Error())) }

func (t *telegram) redactText(s string) string {
	if t.token == "" {
		return s
	}
	return strings.ReplaceAll(s, t.token, "<token>")
}
