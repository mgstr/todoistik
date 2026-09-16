package web

import (
	"encoding/json"
	"net/url"
	"testing"
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
