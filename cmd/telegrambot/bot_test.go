package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"todoistik/internal/apiclient"
)

// stubApp is the app as the two APIs show it: captures are recorded, and a
// view answers with whatever the test put under its name.
type stubApp struct {
	mu       sync.Mutex
	captured []string
	status   string
	views    map[string]string
	queries  []string
}

func (s *stubApp) serve(t *testing.T) *apiclient.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if r.URL.Path == "/api/capture" {
			var body struct{ Text string }
			json.NewDecoder(r.Body).Decode(&body)
			s.captured = append(s.captured, body.Text)
			status := s.status
			if status == "" {
				status = "accepted"
			}
			w.Write([]byte(`{"status":"` + status + `"}`))
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/api/view/")
		s.queries = append(s.queries, name+"?"+r.URL.Query().Get("q"))
		w.Write([]byte(s.views[name]))
	}))
	t.Cleanup(srv.Close)
	return apiclient.New(srv.URL, "")
}

func TestAMessageIsCapturedWhole(t *testing.T) {
	app := &stubApp{}
	b := &bot{app: app.serve(t)}

	text := "Book the tyre change @garage #car\nthey open at 8"
	o := b.answer(text)
	if len(app.captured) != 1 || app.captured[0] != text {
		t.Fatalf("captured %q, want the message as written", app.captured)
	}
	if o.replies[0] != "Added to the inbox" || o.failed {
		t.Errorf("replied %q", o.replies)
	}

	// a path is not a command, and is somebody's thought like any other
	b.answer("/usr/local is full")
	if len(app.captured) != 2 {
		t.Errorf("a line starting with a path was not captured")
	}
}

func TestADuplicateSaysSo(t *testing.T) {
	app := &stubApp{status: "duplicate"}
	o := (&bot{app: app.serve(t)}).answer("Buy milk")
	if o.replies[0] != "Already in the inbox" {
		t.Errorf("replied %q, want the duplicate named", o.replies)
	}
}

func TestACaptureTheAppNeverGotSaysNotSaved(t *testing.T) {
	o := (&bot{app: apiclient.New("http://127.0.0.1:1", "")}).answer("Buy milk")
	if !o.failed || !strings.HasPrefix(o.replies[0], "Not saved: ") {
		t.Errorf("replied %q, failed %v", o.replies, o.failed)
	}
}

func TestAMistypedCommandIsNotCaptured(t *testing.T) {
	app := &stubApp{}
	o := (&bot{app: app.serve(t)}).answer("/nxet @home")
	if len(app.captured) != 0 {
		t.Errorf("captured %q", app.captured)
	}
	if o.replies[0] != "No such command: /nxet" {
		t.Errorf("replied %q", o.replies)
	}
}

func TestAViewIsReadWithTheLineAsWritten(t *testing.T) {
	app := &stubApp{views: map[string]string{
		"next": `{"items":[{"title":"Change the tyres","dueDate":"2026-09-20"},{"title":"Buy milk"}]}`,
	}}
	o := (&bot{app: app.serve(t)}).answer("/next@todoistik_bot  @home\n#car")
	if len(app.queries) != 1 || app.queries[0] != "next?@home #car" {
		t.Errorf("read %q", app.queries)
	}
	want := "Next actions · @home #car · 2\n• Change the tyres · due 2026-09-20\n• Buy milk"
	if len(o.replies) != 1 || o.replies[0] != want {
		t.Errorf("replied %q\nwant %q", o.replies, want)
	}
}

func TestALineTheAppCouldNotReadIsNotAnswered(t *testing.T) {
	app := &stubApp{views: map[string]string{
		"next": `{"items":[{"title":"Everything"}],"problems":[{"token":"#cra","kind":"tag"}]}`,
	}}
	o := (&bot{app: app.serve(t)}).answer("/next #cra")
	if !o.failed || o.replies[0] != "Not read: #cra is no tag" {
		t.Errorf("replied %q — the whole view must not pass for the filtered one", o.replies)
	}
}

func TestAViewWithoutFiltersRefusesALine(t *testing.T) {
	app := &stubApp{}
	b := &bot{app: app.serve(t)}
	for cmd, want := range map[string]string{"/today #car": "Today has no filters", "/inbox milk": "Inbox has no filters"} {
		if o := b.answer(cmd); o.replies[0] != want {
			t.Errorf("%s replied %q, want %q", cmd, o.replies, want)
		}
	}
	if len(app.queries) != 0 {
		t.Errorf("read %q", app.queries)
	}
}

func TestTodayKeepsItsGroups(t *testing.T) {
	app := &stubApp{views: map[string]string{
		"today": `{"items":{"outOfTime":[{"title":"Pay rent","dueDate":"2026-09-15"},{"title":"Replace the log","dueDate":"2026-09-17"}],"picked":[{"title":"Replace the log","dueDate":"2026-09-17"}]}}`,
	}}
	o := (&bot{app: app.serve(t)}).answer("/today")
	want := "Today · 3\n\nOut of time · 2\n• Pay rent · due 2026-09-15\n• Replace the log · due 2026-09-17\n\nPicked · 1\n• Replace the log · due 2026-09-17"
	if len(o.replies) != 1 || o.replies[0] != want {
		t.Errorf("replied %q\nwant %q", o.replies, want)
	}
}

func TestEachViewsLineSaysWhatThatViewIsAbout(t *testing.T) {
	app := &stubApp{views: map[string]string{
		"inbox":     `{"items":[{"text":"Buy milk\nthe oat one"}]}`,
		"projects":  `{"items":[{"title":"Move flat","stalled":true},{"title":"Paint the fence"}]}`,
		"scheduler": `{"items":[{"text":"Pay the rent","nextFire":"2026-10-01"}]}`,
		"archive":   `{"items":[{"action":{"title":"Renew passport","dueDate":"2026-08-01"}},{"project":{"title":"Plan the trip"}}]}`,
	}}
	b := &bot{app: app.serve(t)}
	for cmd, want := range map[string]string{
		"/inbox":     "Inbox · 1\n• Buy milk",
		"/projects":  "Projects · 2\n• Move flat · stalled\n• Paint the fence",
		"/scheduler": "Scheduler · 1\n• Pay the rent · fires 2026-10-01",
		"/archive":   "Archive · 2\n• Renew passport\n• Plan the trip",
	} {
		if o := b.answer(cmd); len(o.replies) != 1 || o.replies[0] != want {
			t.Errorf("%s replied %q\nwant %q", cmd, o.replies, want)
		}
	}
}

func TestALongViewIsSentWholeAcrossMessages(t *testing.T) {
	var lines []string
	for i := 0; i < 50; i++ {
		lines = append(lines, "• задача номер "+strings.Repeat("я", 20))
	}
	parts := split(lines, 400)
	if len(parts) < 2 {
		t.Fatalf("%d part(s)", len(parts))
	}
	var got []string
	for _, p := range parts {
		if units(p) > 400 {
			t.Errorf("a part is %d units long", units(p))
		}
		got = append(got, strings.Split(p, "\n")...)
	}
	if strings.Join(got, "\n") != strings.Join(lines, "\n") {
		t.Errorf("lines were lost or broken across parts")
	}
}

// fakeTelegram serves one batch of updates, then holds every later poll open
// until the test ends, and records what was sent.
type fakeTelegram struct {
	mu      sync.Mutex
	batch   string
	served  bool
	sent    []map[string]any
	paths   []string
	offsets []float64
}

func (f *fakeTelegram) serve(t *testing.T, ctx context.Context) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var params map[string]any
		json.Unmarshal(body, &params)
		f.mu.Lock()
		f.paths = append(f.paths, r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			f.offsets = append(f.offsets, params["offset"].(float64))
			if !f.served {
				f.served = true
				f.mu.Unlock()
				w.Write([]byte(`{"ok":true,"result":` + f.batch + `}`))
				return
			}
			f.mu.Unlock()
			if params["timeout"].(float64) > 0 {
				select {
				case <-r.Context().Done():
				case <-ctx.Done():
				}
			}
			w.Write([]byte(`{"ok":true,"result":[]}`))
			return
		case strings.HasSuffix(r.URL.Path, "/sendMessage"):
			f.sent = append(f.sent, params)
		}
		f.mu.Unlock()
		w.Write([]byte(`{"ok":true,"result":true}`))
	}))
	t.Cleanup(srv.Close)
	telegramBase = srv.URL
}

func TestARunAnswersTheOwnerAndNobodyElse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tg := &fakeTelegram{batch: `[
		{"update_id":10,"message":{"chat":{"id":42},"text":"Buy milk"}},
		{"update_id":11,"message":{"chat":{"id":666,"username":"stranger"},"text":"/next"}}
	]`}
	tg.serve(t, ctx)
	app := &stubApp{}

	var out, errOut bytes.Buffer
	r := &runner{
		tg: newTelegram("123:SECRET"), bot: &bot{app: app.serve(t)},
		chat: 42, out: &out, errOut: &errOut, retry: time.Millisecond,
	}
	done := make(chan error)
	go func() { done <- r.run(ctx) }()

	deadline := time.Now().Add(5 * time.Second)
	for {
		tg.mu.Lock()
		n := len(tg.sent)
		tg.mu.Unlock()
		if n > 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if len(app.captured) != 1 || app.captured[0] != "Buy milk" {
		t.Errorf("captured %q", app.captured)
	}
	if len(tg.sent) != 1 || tg.sent[0]["chat_id"].(float64) != 42 || tg.sent[0]["text"] != "Added to the inbox" {
		t.Errorf("sent %v, want one reply, to the owner", tg.sent)
	}
	if len(app.queries) != 0 {
		t.Errorf("a stranger's command was read: %q", app.queries)
	}
	if !strings.Contains(errOut.String(), "ignored a message from chat 666 (@stranger)") {
		t.Errorf("the stranger's id was not printed:\n%s", errOut.String())
	}
	if !strings.Contains(out.String(), "captured: Buy milk") {
		t.Errorf("the capture was not logged:\n%s", out.String())
	}
	// leaving confirms what was answered, so a restart does not answer it again
	if last := tg.offsets[len(tg.offsets)-1]; last != 12 {
		t.Errorf("the last poll asked from %v, want 12", last)
	}
	if strings.Contains(out.String()+errOut.String(), "SECRET") {
		t.Errorf("the bot token was printed")
	}
}

func TestTheTokenIsNeverInAnError(t *testing.T) {
	telegramBase = "http://127.0.0.1:1"
	tg := newTelegram("123:SECRET")
	_, err := tg.updates(context.Background(), 0, 0)
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("err = %v", err)
	}
}
