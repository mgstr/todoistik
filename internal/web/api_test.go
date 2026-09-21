package web

import (
	"encoding/json"
	"net/url"
	"testing"

	"todoistik/internal/app"
)

// A read whose line names something the app cannot read still answers, with
// what it could — but says what it left out. A caller has no mark on a screen
// to see, so without this a mistyped tag would read the whole view and look
// like a read of the filtered one.
func TestTheReadAPISaysWhatItLeftOutOfTheLine(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}

	read := func(q string) map[string]json.RawMessage {
		t.Helper()
		var answer map[string]json.RawMessage
		body := getPage(t, s, "/api/view/next?"+url.Values{"q": {q}}.Encode())
		if err := json.Unmarshal([]byte(body), &answer); err != nil {
			t.Fatalf("%v\n%s", err, body)
		}
		return answer
	}

	if _, ok := read("#car milk")["problems"]; ok {
		t.Errorf("a line the app read whole carries problems")
	}

	var problems []apiProblem
	if err := json.Unmarshal(read("#cra due:sometime milk")["problems"], &problems); err != nil {
		t.Fatal(err)
	}
	want := []apiProblem{{Token: "due:sometime", Kind: "window"}, {Token: "#cra", Kind: "tag"}}
	if len(problems) != len(want) {
		t.Fatalf("problems = %+v, want %+v", problems, want)
	}
	for i := range want {
		if problems[i] != want[i] {
			t.Errorf("problem %d = %+v, want %+v", i, problems[i], want[i])
		}
	}
}

// Every view offers its own subset of the filters and the read API may not
// offer more than the screen does (design.md, "The read API"), so a filter a
// view does not have is named and left out — whether it was written as a line
// or spelled out one parameter at a time.
func TestTheReadAPIOffersOnlyWhatEachViewFiltersBy(t *testing.T) {
	s, a := newTestServer(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if err := a.AddContext("home", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Change the tyres", Tags: []string{"car"}}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Wash up", Context: "home"}, false); err != nil {
		t.Fatal(err)
	}

	read := func(path string) (int, []apiProblem) {
		t.Helper()
		var answer struct {
			Items    []json.RawMessage `json:"items"`
			Problems []apiProblem      `json:"problems"`
		}
		body := getPage(t, s, path)
		if err := json.Unmarshal([]byte(body), &answer); err != nil {
			t.Fatalf("%v\n%s", err, body)
		}
		return len(answer.Items), answer.Problems
	}

	// Tasks asks about tags and names only: a context answers "what can I do
	// now", which is Next actions' question
	items, problems := read("/api/view/tasks?q=%40home")
	if len(problems) != 1 || problems[0] != (apiProblem{Token: "@home", Kind: "not-in-view"}) {
		t.Errorf("problems = %+v, want @home named", problems)
	}
	if items != 2 {
		t.Errorf("the tasks view held %d items, want the context to have narrowed nothing", items)
	}
	// the same filter spelled as a parameter is the same answer
	if _, problems = read("/api/view/tasks?context=home"); len(problems) != 1 {
		t.Errorf("problems = %+v, want @home named", problems)
	}
	// and on the view that does offer it, it filters and says nothing
	if items, problems = read("/api/view/next?q=%40home"); items != 1 || len(problems) != 0 {
		t.Errorf("next?@home = %d items, problems %+v", items, problems)
	}
	// the Inbox takes no filters at all (design.md, "Inbox")
	if _, problems = read("/api/view/inbox?tag=car"); len(problems) != 1 || problems[0].Token != "#car" {
		t.Errorf("problems = %+v, want #car named", problems)
	}
}
