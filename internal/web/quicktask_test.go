package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// `T` on the processing question files the capture as a task without opening
// the form (design.md, "Inbox Zero"). What it must not be is a second reading
// of the capture: these pin it to what the form would have held, so the two
// presses can only ever differ by whether the form was looked at.

// The question offers it as a control of its own, beside the one that opens
// the form, under the capital of the same letter.
func TestTheQuestionOffersTaskAsIs(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Call the garage", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	page := getPage(t, s, "/process?item=1&one=1")
	if !strings.Contains(page, `action="/process/1/quicktask?one=1"`) || !strings.Contains(page, `data-key="T"`) {
		t.Fatalf("the question does not offer the formless task:\n%s", page)
	}
	if !strings.Contains(page, `data-key="t"`) {
		t.Fatalf("the form's own key went missing:\n%s", page)
	}
}

// The first line is read as notation and the body goes to the description,
// exactly as the form is seeded — and `#today` on the line is the pick it
// would have been.
func TestTaskAsIsFilesWhatTheFormWouldHaveOpenedOn(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddContext("phone", ""); err != nil {
		t.Fatal(err)
	}
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Call the garage @phone #car #today\nask about the #tyres, 555 0101", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	inbox, _ := a.Inbox()
	id := itoa(inbox[0].ID)

	// what the form would have held, read off the form itself
	form := getPage(t, s, "/process?item="+id+"&as=task")
	for _, want := range []string{`value="Call the garage"`, "ask about the #tyres, 555 0101"} {
		if !strings.Contains(form, want) {
			t.Fatalf("the form is not seeded with %q:\n%s", want, form)
		}
	}

	rec := postForm(t, s, "/process/"+id+"/quicktask", url.Values{})
	if to := rec.Header().Get("Location"); to != "/process" {
		t.Errorf("the run did not carry on to the next capture: %q", to)
	}
	if left, _ := a.Inbox(); len(left) != 0 {
		t.Fatalf("the capture is still in the inbox: %d left", len(left))
	}
	tasks, err := a.Tasks(app.Filters{})
	if err != nil || len(tasks) != 1 {
		t.Fatalf("want one task, got %d (%v)", len(tasks), err)
	}
	got := tasks[0]
	if got.Title != "Call the garage" {
		t.Errorf("title = %q", got.Title)
	}
	if got.Context != "phone" {
		t.Errorf("context = %q", got.Context)
	}
	if got.Description != "ask about the #tyres, 555 0101" {
		t.Errorf("description = %q", got.Description)
	}
	if got.ProjectID != 0 {
		t.Errorf("a task landed in project %d", got.ProjectID)
	}
	tags := strings.Join(got.Tags, " ")
	if !strings.Contains(tags, "car") || !strings.Contains(tags, app.TodayTag) {
		t.Errorf("tags = %q, want car and the pick", tags)
	}
	// the body is never read as notation, so its # stayed prose
	if strings.Contains(tags, "tyres") {
		t.Errorf("the body reached the tags: %q", tags)
	}
}

// One picked item goes back to the list it was picked from, as every other
// answer does.
func TestTaskAsIsOnOnePickedItemGoesBackToTheInbox(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Call the garage", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	rec := postForm(t, s, "/process/1/quicktask?one=1", url.Values{})
	if to := rec.Header().Get("Location"); to != "/inbox" {
		t.Errorf("Location = %q", to)
	}
}

// A capture the form would have refused is not filed: the form comes back
// filled in and saying why. Here the whole line is notation, so there is no
// title left to file it under.
func TestTaskAsIsHandsBackTheFormItWouldHaveBeenRefusedOn(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("#car", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/process/1/quicktask?one=1", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("POST: %d\n%s", rec.Code, body)
	}
	if !strings.Contains(body, app.ErrTitle.Error()) {
		t.Errorf("the form does not say why it came back:\n%s", body)
	}
	if !strings.Contains(body, `action="/process/1/action?one=1"`) || !strings.Contains(body, `value="#car"`) {
		t.Errorf("what came back is not the task form, seeded:\n%s", body)
	}
	if left, _ := a.Inbox(); len(left) != 1 {
		t.Errorf("a refused capture left the inbox: %d left", len(left))
	}
	if tasks, _ := a.Tasks(app.Filters{}); len(tasks) != 0 {
		t.Errorf("a refused capture was filed anyway: %d tasks", len(tasks))
	}
}
