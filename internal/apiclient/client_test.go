package apiclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// View is read the same way whatever the view, so the shape a view answers in
// is this package's to absorb, not every caller's.
func TestView(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		want   []Item
		fails  bool
	}{
		{
			"a list view is its list",
			`{"view":"next","items":[{"id":15,"title":"залогировать кеш"},{"id":7,"title":"Replace the log"}]}`,
			[]Item{{ID: 15, Title: "залогировать кеш"}, {ID: 7, Title: "Replace the log"}},
			false,
		},
		{
			"an empty list view is no items, not a refusal",
			`{"view":"next","items":null}`,
			nil,
			false,
		},
		{
			"today is out of time first, then picked, and an action in both once",
			`{"view":"today","items":{"outOfTime":[{"id":3,"title":"Pay rent","dueDate":"2026-09-15"},{"id":7,"title":"Replace the log","dueDate":"2026-09-16"}],"picked":[{"id":7,"title":"Replace the log","dueDate":"2026-09-16"},{"id":15,"title":"залогировать кеш"}]}}`,
			[]Item{{ID: 3, Title: "Pay rent", DueDate: "2026-09-15"}, {ID: 7, Title: "Replace the log", DueDate: "2026-09-16"}, {ID: 15, Title: "залогировать кеш"}},
			false,
		},
		{
			"today with an empty group is the other group",
			`{"view":"today","items":{"outOfTime":null,"picked":[{"id":15,"title":"залогировать кеш"}]}}`,
			[]Item{{ID: 15, Title: "залогировать кеш"}},
			false,
		},
		{
			"today with nothing in it is no items",
			`{"view":"today","items":{"outOfTime":null,"picked":null}}`,
			nil,
			false,
		},
		{
			"counts are not items",
			`{"view":"review","items":{"inbox":3,"someday":12}}`,
			nil,
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(c.answer))
			}))
			defer srv.Close()
			got, err := New(srv.URL, "").View("any", "")
			if c.fails {
				if err == nil {
					t.Fatalf("View() = %+v, want a refusal", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("View() error: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("View() = %+v, want %+v", got, c.want)
			}
		})
	}
}

// A view read through a line the app could only partly read is refused, not
// mirrored: `#cra` for `#car` would otherwise put the whole view on a phone,
// looking exactly like the filtered one.
func TestAViewReadThroughAnUnreadableLineIsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"view":"next","items":[{"id":1,"title":"Everything"}],` +
			`"problems":[{"token":"#cra","kind":"tag"},{"token":"due:sometime","kind":"window"}]}`))
	}))
	defer srv.Close()

	items, err := New(srv.URL, "").View("next", "#cra due:sometime")
	if items != nil {
		t.Errorf("View() = %+v, want nothing", items)
	}
	if !errors.Is(err, ErrFatal) {
		t.Fatalf("View() error = %v, want a fatal one: the same line fails the same way next run", err)
	}
	if !strings.Contains(err.Error(), "#cra is no tag") || !strings.Contains(err.Error(), "due:sometime is not a window") {
		t.Errorf("View() error = %v, want it to name every token", err)
	}

	// the same answer is still readable by a caller that wants the problems
	answer, err := New(srv.URL, "").Read("next", "#cra")
	if err != nil || len(answer.Problems) != 2 {
		t.Errorf("Read() = %+v, %v", answer, err)
	}
}
