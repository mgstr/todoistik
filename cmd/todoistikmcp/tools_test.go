package main

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"todoistik/internal/apiclient"
	"todoistik/internal/app"
	"todoistik/internal/conf"
	"todoistik/internal/web"
)

// The server is tested against the real app over the real protocol: an
// in-memory transport into an mcp.Client, and an httptest server around the
// actual web.Server. What is worth pinning is the part a stub would let drift —
// that the tools reach the app's own answers, that a duplicate capture does not
// read as a failure, and that a view name the app does not have cannot be sent
// at all.

func connect(t *testing.T) (*mcp.ClientSession, *app.App) {
	t.Helper()
	a, err := app.Open(filepath.Join(t.TempDir(), "test.db"), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	ws, err := web.New(a, "", conf.Config{})
	if err != nil {
		t.Fatal(err)
	}
	http := httptest.NewServer(ws.Handler())
	t.Cleanup(http.Close)

	ctx := context.Background()
	clientT, serverT := mcp.NewInMemoryTransports()
	go newServer(apiclient.New(apiclient.Base(http.URL), "")).Run(ctx, serverT)

	session, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session, a
}

func call(t *testing.T, s *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return res
}

func said(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// Three tools, no more: the surface is a decision, not an accident, and a
// fourth appearing is something to notice.
func TestTheServerOffersThreeTools(t *testing.T) {
	s, _ := connect(t)
	res, err := s.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, tool := range res.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got) // the protocol lists them by name, not by the order they were added
	want := "todoistik_capture todoistik_context todoistik_read_view"
	if strings.Join(got, " ") != want {
		t.Fatalf("tools are %v, want %s", got, want)
	}
}

// The tools reach the app's own text, not a rendering of their own.
func TestReadingAViewIsTheAppsOwnAnswer(t *testing.T) {
	s, a := connect(t)
	if err := a.AddTag("car"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateAction(0, app.ActionFields{Title: "Buy new winter tyres", Tags: []string{"car"}}, false); err != nil {
		t.Fatal(err)
	}

	got := said(call(t, s, "todoistik_read_view", map[string]any{"view": "next"}))
	if !strings.Contains(got, "next — 1 item") || !strings.Contains(got, "Buy new winter tyres  #car") {
		t.Fatalf("not the read API's own text:\n%s", got)
	}
	if !strings.Contains(said(call(t, s, "todoistik_context", nil)), "archive — ") {
		t.Fatal("the bundle did not come through whole")
	}
}

// A filter the app could not read whole is answered with the part it could and
// the warning in the text — unlike the Reminders path, which has to refuse,
// because a list has nowhere to put the sentence and a block of text is
// nothing but somewhere to put it.
func TestAnUnreadableFilterComesBackWithItsWarning(t *testing.T) {
	s, _ := connect(t)
	got := said(call(t, s, "todoistik_read_view", map[string]any{"view": "next", "q": "#cra"}))
	if !strings.Contains(got, "not read: #cra is no tag") {
		t.Fatalf("the warning did not come through:\n%s", got)
	}
}

// A view the app does not have cannot be sent: the enum is in the schema, so
// the client refuses it before the app is asked.
func TestAViewTheAppDoesNotHaveIsRefused(t *testing.T) {
	s, _ := connect(t)
	res, err := s.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "todoistik_read_view",
		Arguments: map[string]any{"view": "audit"},
	})
	if err == nil && !res.IsError {
		t.Fatalf("a view the app does not have was answered:\n%s", said(res))
	}
}

// A capture goes in, and a second identical one says so without reading as a
// failure — a model told it failed would try again, and the loop it describes
// is already in the inbox waiting to be decided about.
func TestADuplicateCaptureIsNotAFailure(t *testing.T) {
	s, a := connect(t)

	first := call(t, s, "todoistik_capture", map[string]any{"text": "Ask about the loft insulation"})
	if first.IsError || !strings.Contains(said(first), "Added to the inbox") {
		t.Fatalf("first capture: %v %s", first.IsError, said(first))
	}
	second := call(t, s, "todoistik_capture", map[string]any{"text": "Ask about the loft insulation"})
	if second.IsError {
		t.Fatalf("a duplicate was reported as an error: %s", said(second))
	}
	if !strings.Contains(said(second), "Already in the inbox") {
		t.Fatalf("a duplicate did not say so: %s", said(second))
	}

	inbox, err := a.Inbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 1 {
		t.Fatalf("the inbox holds %d items after two identical captures", len(inbox))
	}
}

// Empty text is refused here rather than sent, so the answer is about what was
// asked rather than about a body the app never received.
func TestAnEmptyCaptureIsRefused(t *testing.T) {
	s, a := connect(t)
	res := call(t, s, "todoistik_capture", map[string]any{"text": "   "})
	if !res.IsError {
		t.Fatalf("an empty capture was accepted: %s", said(res))
	}
	inbox, err := a.Inbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(inbox) != 0 {
		t.Fatalf("an empty capture put %d items in the inbox", len(inbox))
	}
}

// The enum the schema carries is the app's own view list. Two lists would let
// a view be added to the app and stay unreachable here with nothing to say so.
func TestTheViewEnumIsTheAppsViewList(t *testing.T) {
	s, _ := connect(t)
	for _, v := range views {
		res := call(t, s, "todoistik_read_view", map[string]any{"view": v})
		if res.IsError {
			t.Errorf("%s: the app does not answer a view this server offers: %s", v, said(res))
		}
	}
}
