package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// Undo from the outside: a press at the UI is a step, the question reads it
// out, and the answer takes it back (design.md, "Undo"). The rules about what
// a step is are pinned in internal/app; these are about the wiring — that a
// post is a gesture, that the API's is not, and what the page offers.

func undoKeyOn(body string) string {
	i := strings.Index(body, "data-undo-open")
	if i < 0 {
		return ""
	}
	return body[i : i+strings.Index(body[i:], ">")]
}

func askUndo(t *testing.T, s *Server) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/undo", nil))
	return rec.Code, rec.Body.String()
}

func TestAPressIsAStepAndUndoTakesItBack(t *testing.T) {
	s, a := newTestServer(t)
	// nothing done yet: the key is on the page and cannot be pressed, which
	// is what keeps it out of the bar
	if key := undoKeyOn(getPage(t, s, "/inbox")); !strings.Contains(key, "disabled") {
		t.Fatalf("undo is offered with nothing to take back: %s", key)
	}
	if code, _ := askUndo(t, s); code != http.StatusNoContent {
		t.Fatalf("asking with nothing to undo answered %d", code)
	}

	postForm(t, s, "/capture", url.Values{"text": {"Call the garage"}, "back": {"/inbox"}})
	postForm(t, s, "/process/1/trash", url.Values{})
	if key := undoKeyOn(getPage(t, s, "/inbox")); key == "" || strings.Contains(key, "disabled") {
		t.Fatalf("undo is not offered after a press: %q", key)
	}

	code, question := askUndo(t, s)
	if code != http.StatusOK || !strings.Contains(question, "trashed") || !strings.Contains(question, "Call the garage") {
		t.Fatalf("the question (%d) does not read out the trashing:\n%s", code, question)
	}
	i := strings.Index(question, `data-step="`) + len(`data-step="`)
	step := question[i : i+strings.Index(question[i:], `"`)]

	rec := postForm(t, s, "/undo", url.Values{"step": {step}, "back": {"/inbox"}})
	if got := rec.Header().Get("Location"); got != "/inbox" {
		t.Errorf("undo landed on %q, want the screen it was asked on", got)
	}
	items, _ := a.Inbox()
	if len(items) != 1 || items[0].Text != "Call the garage" {
		t.Fatalf("the capture did not come back: %+v", items)
	}
	// more than one level: the capture itself is the step before
	if _, question = askUndo(t, s); !strings.Contains(question, "created") {
		t.Fatalf("the next question is not about the capture:\n%s", question)
	}
}

// A capture through the API is nobody's gesture, so it is not what `u` takes
// back — losing a mail to a stray key is the one thing this must not do.
func TestAnAPICaptureIsNotAStep(t *testing.T) {
	s, _ := newTestServer(t)
	rec := postCapture(t, s, "/api/capture?source=mail", "Re: the quote")
	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated && rec.Code != http.StatusAccepted {
		t.Fatalf("capture: %d %s", rec.Code, rec.Body.String())
	}
	if code, body := askUndo(t, s); code != http.StatusNoContent {
		t.Fatalf("an API capture is offered to be undone (%d):\n%s", code, body)
	}
}

// Undoing the step that made the item whose page this is leaves no page to
// come back to, so it lands on the pile the item would have been in.
func TestUndoingFromThePageOfTheItemItRemoves(t *testing.T) {
	s, _ := newTestServer(t)
	postForm(t, s, "/capture", url.Values{"text": {"Garden"}, "back": {"/inbox"}})
	postForm(t, s, "/process/1/someday", url.Values{"text": {"Garden"}})
	_, question := askUndo(t, s)
	i := strings.Index(question, `data-step="`) + len(`data-step="`)
	step := question[i : i+strings.Index(question[i:], `"`)]
	rec := postForm(t, s, "/undo", url.Values{"step": {step}, "back": {"/somedayitem/1"}})
	if got := rec.Header().Get("Location"); got != "/someday" {
		t.Errorf("undo landed on %q, want the view the vanished item belonged to", got)
	}
}

// Undone is `d` now: the letter Done has on the same page, said from the
// other side (keys.md, "The map"). And it asks for the deaf window that keeps
// a double press from completing the item it has just brought back.
func TestUndoneIsOnD(t *testing.T) {
	s, a := newTestServer(t)
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Buy milk"}); err != nil {
		t.Fatal(err)
	}
	postForm(t, s, "/action/1/complete", url.Values{"back": {"/tasks"}})
	form := formOn(t, getPage(t, s, "/action/1"), "/action/1/uncomplete")
	if !strings.Contains(form, `data-key="d"`) || !strings.Contains(form, "data-deaf-after") {
		t.Errorf("the Undone form is not `d` with a deaf window: %s", form)
	}
}
