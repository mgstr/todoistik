package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"todoistik/internal/app"
)

func postCapture(t *testing.T, s *Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

// A caller from outside names the channel it is, in the query string or in the
// payload, and one that names nothing is `api` — it is a way in like any other
// and an empty source would be the one row in the aggregation that says
// nothing (design.md, "External capture").
func TestTheCaptureAPITakesTheCallersOwnName(t *testing.T) {
	for _, c := range []struct{ name, path, body, want string }{
		{"a query parameter", "/api/capture?source=shortcuts", `{"text":"Ring the vet"}`, "shortcuts"},
		{"a field in the payload", "/api/capture", `{"text":"Ring the vet","source":"watch"}`, "watch"},
		{"nothing at all", "/api/capture", `{"text":"Ring the vet"}`, app.SourceAPI},
		{"a name that is all punctuation", "/api/capture?source=%21%21", `{"text":"Ring the vet"}`, app.SourceAPI},
		{"raw text and no JSON", "/api/capture?source=shortcuts", "Ring the vet", "shortcuts"},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, a := newTestServer(t)
			rec := postCapture(t, s, c.path, c.body)
			if rec.Code != http.StatusCreated {
				t.Fatalf("POST: %d\n%s", rec.Code, rec.Body.String())
			}
			items, err := a.Inbox()
			if err != nil || len(items) != 1 {
				t.Fatalf("inbox: %d items (%v)", len(items), err)
			}
			if items[0].Source != c.want {
				t.Fatalf("captured under %q, want %q", items[0].Source, c.want)
			}
			// the answer says what was recorded, so a script can see that it
			// named itself the way it meant to
			var answer struct {
				Item app.InboxItem `json:"item"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
				t.Fatal(err)
			}
			if answer.Item.Source != c.want {
				t.Fatalf("the reply says %q", answer.Item.Source)
			}
		})
	}
}

// The dialog is a way in too, and the one that is a person typing.
func TestTheCaptureDialogIsItsOwnChannel(t *testing.T) {
	s, a := newTestServer(t)
	postForm(t, s, "/capture", url.Values{"text": {"Ask about the fence"}})
	items, _ := a.Inbox()
	if len(items) != 1 || items[0].Source != app.SourceApp {
		t.Fatalf("%d items, captured under %q", len(items), items[0].Source)
	}
}

// Both spellings of a read are the same answer (implementation.md, "A read is
// spelled as data or as text"), so the inbox carries the channel and the hour
// in each — and an item captured before the field existed says nothing rather
// than claiming a channel it never had.
func TestBothSpellingsOfTheInboxCarryTheSourceAndTheHour(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Ring the vet", "telegram"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Book the tyre change", ""); err != nil {
		t.Fatal(err)
	}

	var answer struct {
		Items []app.InboxItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(getPage(t, s, "/api/view/inbox")), &answer); err != nil {
		t.Fatal(err)
	}
	items := answer.Items
	if len(items) != 2 || items[0].Source != "telegram" || items[1].Source != "" {
		t.Fatalf("json: %+v", items)
	}

	text := getPage(t, s, "/api/view/inbox?format=text")
	if !strings.Contains(text, "via:telegram") {
		t.Errorf("the text spelling drops the channel:\n%s", text)
	}
	if strings.Contains(text, "via:\n") || strings.Contains(text, "via: ") {
		t.Errorf("an item with no channel should carry no via: at all:\n%s", text)
	}
	if !strings.Contains(text, "captured:"+items[0].CreatedAt.UTC().Format("2006-01-02T15:04")) {
		t.Errorf("the inbox's stamp is a moment, not a day:\n%s", text)
	}
}

// The processing screen is the one place the channel and the hour are read: it
// is where a thin capture has to be recognised, and knowing either before you
// look at an item buys nothing (design.md, "Inbox item").
func TestTheProcessingScreenSaysTheChannelAndTheHour(t *testing.T) {
	s, a := newTestServer(t)
	it, _, err := a.Capture("Milk, bread, the good coffee", "reminders")
	if err != nil {
		t.Fatal(err)
	}
	page := getPage(t, s, "/process")
	if !strings.Contains(page, "via reminders") {
		t.Errorf("the processing screen does not say where it came from:\n%s", page)
	}
	if !strings.Contains(page, "at "+it.CreatedAt.UTC().Format("15:04")) {
		t.Errorf("the processing screen does not say when it arrived:\n%s", page)
	}
	if inbox := getPage(t, s, "/inbox"); strings.Contains(inbox, "reminders") {
		t.Errorf("the inbox list marks the row, which it must never do:\n%s", inbox)
	}
}
