package apiclient

import (
	"net/http"
	"net/http/httptest"
	"reflect"
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
