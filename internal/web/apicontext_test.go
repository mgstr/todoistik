package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"todoistik/internal/app"
)

// The bundle is a stapler, not a query. What is worth pinning is that it
// staples every view and no more: a view added to the app is in it without
// anyone remembering to add it, the Archive is the one view it narrows, and
// each section is byte for byte what asking for that view alone would give.

func getBundle(t *testing.T, s *Server, path string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d\n%s", path, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// Not "the eleven views I listed in the test" — whatever apiViews holds, which
// is also the list a bad view name is answered with. One list, so a twelfth
// view cannot be added to the app and quietly left out of the bundle.
func TestTheBundleAnswersEveryViewTheReadAPIHas(t *testing.T) {
	s, _ := newTestServer(t)

	var bundle struct {
		Today string `json:"today"`
		Views []struct {
			View string `json:"view"`
		} `json:"views"`
	}
	if err := json.Unmarshal([]byte(getBundle(t, s, "/api/context")), &bundle); err != nil {
		t.Fatal(err)
	}
	if bundle.Today != s.app.Today() {
		t.Errorf("today is %q, the app says %q", bundle.Today, s.app.Today())
	}
	var got []string
	for _, v := range bundle.Views {
		got = append(got, v.View)
	}
	if strings.Join(got, " ") != strings.Join(apiViews, " ") {
		t.Fatalf("bundle answers %v, the read API has %v", got, apiViews)
	}
}

// Each section is what asking for that view alone gives, header and all, so a
// section cut out of the paste is still a complete answer.
func TestEachTextSectionIsThatViewsOwnAnswer(t *testing.T) {
	s, a := newTestServer(t)
	if _, _, err := a.Capture("Renew the parking permit", app.SourceApp); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Buy new winter tyres"}); err != nil {
		t.Fatal(err)
	}

	bundle := getBundle(t, s, "/api/context?format=text")
	for _, v := range []string{"inbox", "next", "review"} {
		alone := strings.TrimRight(readText(t, s, "/api/view/"+v+"?format=text"), "\n")
		if !strings.Contains(bundle, alone) {
			t.Fatalf("the %s section is not the %s view's own answer.\nalone:\n%s\nbundle:\n%s", v, v, alone, bundle)
		}
	}
}

// The Archive is the one view with no bottom, so it is the one view the bundle
// narrows — and it is narrowed with the vocabulary `completed:` already has,
// not a second one.
func TestTheBundleNarrowsOnlyTheArchive(t *testing.T) {
	s, _ := newTestServer(t)

	var bundle struct {
		Views []struct {
			View    string      `json:"view"`
			Filters app.Filters `json:"filters"`
		} `json:"views"`
	}
	if err := json.Unmarshal([]byte(getBundle(t, s, "/api/context?archive=week")), &bundle); err != nil {
		t.Fatal(err)
	}
	for _, v := range bundle.Views {
		switch {
		case v.View == "archive" && v.Filters.Completed != "week":
			t.Errorf("the archive was read with %q, not the window asked for", v.Filters.Completed)
		case v.View != "archive" && v.Filters.Active():
			t.Errorf("%s was narrowed by %v; every view but the archive is answered whole", v.View, v.Filters)
		}
	}
}

// A window the app does not have is refused rather than silently replaced by
// the default, for the reason an unread filter token is named: a bundle whose
// archive covers something other than what was asked for cannot be told apart
// from one that covers what was.
func TestABadArchiveWindowIsRefused(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/context?archive=sometime", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "is not a window") {
		t.Fatalf("archive=sometime: %d\n%s", rec.Code, rec.Body.String())
	}
}

// "none" leaves the Archive out entirely — the one thing the window vocabulary
// cannot say, and the reason the bundle needs a word of its own for it.
func TestArchiveNoneLeavesTheArchiveOut(t *testing.T) {
	s, _ := newTestServer(t)
	if strings.Contains(getBundle(t, s, "/api/context?archive=none&format=text"), "archive —") {
		t.Fatal("archive=none still answered the archive")
	}
	if !strings.Contains(getBundle(t, s, "/api/context?format=text"), "archive —") {
		t.Fatal("the archive is missing from a bundle that did not ask for none")
	}
}

// An action finished outside the window is out of the bundle's archive, and
// one inside it is in — the window is applied, not merely recorded.
func TestTheArchiveWindowIsApplied(t *testing.T) {
	s, a := newTestServer(t)
	act, err := a.CreateAction(0, app.ActionFields{Title: "Book the tyre change"})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	id := "::" + strconv.FormatInt(act.ID, 10)

	if !strings.Contains(getBundle(t, s, "/api/context?archive=today&format=text"), id) {
		t.Fatal("an action completed today is not in an archive window of today")
	}
	if strings.Contains(getBundle(t, s, "/api/context?archive=yesterday&format=text"), id) {
		t.Fatal("an action completed today is in an archive window of yesterday")
	}
}

// The bundle takes the same format parameter, with the same default and the
// same refusal, because there is one rule about formats for the whole API.
func TestTheBundleTakesTheSameFormats(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/context", nil))
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("no format: content type %q, JSON is the default", ct)
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/context?format=yaml", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "one of: json text") {
		t.Fatalf("format=yaml: %d\n%s", rec.Code, rec.Body.String())
	}
}
