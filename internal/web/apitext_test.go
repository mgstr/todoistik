package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"todoistik/internal/apiclient"
	"todoistik/internal/app"
)

// format=text is a spelling and not a second read API: the rules worth pinning
// are the ones that would let the two drift apart — that it answers the same
// views under the same filters, that it never stays quiet about a token it
// could not read, and that an unknown format is refused rather than guessed at.

func readText(t *testing.T, s *Server, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d\n%s", path, rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("GET %s: content type %q", path, ct)
	}
	return rec.Body.String()
}

// The line is the action written in the notation it is typed in, with its id
// in front of it and its description under it — everything the answer holds,
// because a reader of text has no second page to open.
func TestATextActionCarriesItsNotationAndItsDescription(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddContext("grocery", ""); err != nil {
		t.Fatal(err)
	}
	act, err := a.CreateAction(0, app.ActionFields{
		Title:       "Buy new winter tyres",
		Context:     "grocery",
		Duration:    app.DurShort,
		Tags:        []string{"car"},
		DueDate:     "2000-01-01", // long past, so it is overdue whenever this runs
		Description: "205/55 R16\nquoted 240 eur",
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	got := readText(t, s, "/api/view/tasks?format=text")
	want := "::" + strconv.FormatInt(act.ID, 10) + "  Buy new winter tyres  @grocery #short #car due:2000-01-01"
	if !strings.Contains(got, want) {
		t.Fatalf("no action line %q in:\n%s", want, got)
	}
	if !strings.Contains(got, "overdue") {
		t.Fatalf("an action due in 2000 is not marked overdue:\n%s", got)
	}
	for _, l := range []string{"    205/55 R16", "    quoted 240 eur"} {
		if !strings.Contains(got, l) {
			t.Fatalf("no description line %q in:\n%s", l, got)
		}
	}
	if !strings.Contains(got, "tasks — 1 item") || !strings.Contains(got, "today: "+a.Today()) {
		t.Fatalf("header does not say the view, the count and the day:\n%s", got)
	}
}

// The Projects screen shows neither the definition of done nor the actions,
// because both are one keystroke away on the project's page. Text has no
// keystroke, so it carries them.
func TestATextProjectCarriesItsDODAndItsActions(t *testing.T) {
	s, a := newTestServer(t)
	p, err := a.CreateProject(
		app.ProjectFields{Title: "Winter tyres", DOD: "the car is on winter tyres"},
		[]app.ActionFields{{Title: "Ring the garage"}})
	if err != nil {
		t.Fatal(err)
	}

	got := readText(t, s, "/api/view/projects?format=text")
	if !strings.Contains(got, "::"+strconv.FormatInt(p.ID, 10)+"  Winter tyres") {
		t.Fatalf("no project line:\n%s", got)
	}
	if !strings.Contains(got, "    done when: the car is on winter tyres") {
		t.Fatalf("no definition of done:\n%s", got)
	}
	if !strings.Contains(got, "    ::") || !strings.Contains(got, "Ring the garage") {
		t.Fatalf("the project's action is not under it:\n%s", got)
	}
}

// A filter the app could only partly read is the failure the JSON answer
// spends a field on: without it, a read of the whole view and a read of the
// filtered one are the same text. The text says so in its header.
func TestATextReadSaysWhatItCouldNotRead(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}

	got := readText(t, s, "/api/view/next?format=text&q=%23car+%23cra")
	if !strings.Contains(got, "filter: #car") {
		t.Fatalf("the header does not say what it filtered by:\n%s", got)
	}
	if !strings.Contains(got, "not read: #cra is no tag") {
		t.Fatalf("the header does not say what it left out:\n%s", got)
	}
}

// One wording per problem, said by everything that says it. The server writes
// the sentence into a text read; internal/apiclient writes the same sentence
// for the chat and the terminal, and cannot import the domain to share it.
func TestTheServerAndTheClientSayAProblemTheSameWay(t *testing.T) {
	for _, kind := range []string{
		app.ProblemTag, app.ProblemContext, app.ProblemSecondContext,
		app.ProblemNotAFilter, app.ProblemWindow, app.ProblemNotInView, "something-new",
	} {
		server := app.QueryProblem{Token: "#cra", Kind: kind}.String()
		client := apiclient.Problem{Token: "#cra", Kind: kind}.String()
		if server != client {
			t.Errorf("%s: server says %q, client says %q", kind, server, client)
		}
	}
}

// An unknown format is refused rather than served as JSON: a caller that asked
// for text and was handed JSON would parse it wrong and never know why.
func TestAnUnknownFormatIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/view/next?format=yaml", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "one of: json text") {
		t.Fatalf("format=yaml: %d\n%s", rec.Code, rec.Body.String())
	}
}

// Asking for no format, or for json, is the answer that was there before this
// file existed — the format says how, never what.
func TestJSONIsStillTheDefault(t *testing.T) {
	s, _ := newTestServer(t)
	for _, path := range []string{"/api/view/next", "/api/view/next?format=json"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("GET %s: content type %q", path, ct)
		}
	}
}

// Every view answers in text, including the two that are not a list of items.
func TestEveryViewHasATextSpelling(t *testing.T) {
	s, _ := newTestServer(t)
	for _, v := range []string{
		"inbox", "someday", "projects", "tasks", "next", "today",
		"waiting", "calendar", "archive", "scheduler", "review",
	} {
		got := readText(t, s, "/api/view/"+v+"?format=text")
		if !strings.HasPrefix(got, v) {
			t.Errorf("%s: the answer does not open with the view's name:\n%s", v, got)
		}
	}
}
