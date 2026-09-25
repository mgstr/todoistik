package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"todoistik/internal/app"
)

func getDashboard(t *testing.T, s *Server) string {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /dashboard: %d\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// The bar's length is the one thing on this screen written as a style, and
// html/template sanitises every one of them — a value it will not vouch for is
// replaced with ZgotmplZ and the bar is drawn at nothing. That failure is
// silent and looks exactly like "there is no data", which is the worst thing a
// dashboard can look like by accident, so it is pinned here.
func TestTheBarsAreActuallyDrawn(t *testing.T) {
	s, a := newTestServer(t)
	for _, text := range []string{"Buy tyres", "Ask about the roof", "A newsletter"} {
		if _, _, err := a.Capture(text, app.SourceApp); err != nil {
			t.Fatal(err)
		}
	}
	inbox, _ := a.Inbox()
	if _, err := a.ProcessAction(inbox[0].ID, app.ActionFields{Title: "Book the tyre change"}, 0); err != nil {
		t.Fatal(err)
	}

	body := getDashboard(t, s)
	if strings.Contains(body, "ZgotmplZ") {
		t.Fatal("a bar's width was refused by the template's CSS filter and drawn as nothing")
	}
	if !strings.Contains(body, `style="width:100%"`) {
		t.Error("the panel's largest row is not drawn full width")
	}
	// every panel carries its own number in words, so the screen reads with
	// the bars ignored entirely (design.md, "Dashboard")
	for _, want := range []string{
		"Inbound", "Outbound", "What the inbox became", "Where captures come from",
		"Ages", "The practice", "How much of Next is workable",
		"The oldest thing in each view", "The backlog",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the %q panel is missing", want)
		}
	}
	// and the channel every capture came by, counted off the snapshots
	if !strings.Contains(body, app.SourceApp) {
		t.Errorf("the source panel does not name %q", app.SourceApp)
	}
}

// An empty database is the first thing this screen is ever opened against, and
// a panel with nothing to count must say so rather than dividing by it.
func TestTheDashboardOpensOnAnEmptyDatabase(t *testing.T) {
	s, _ := newTestServer(t)
	body := getDashboard(t, s)
	if strings.Contains(body, "ZgotmplZ") || strings.Contains(body, "NaN") {
		t.Error("an empty dashboard rendered a broken number")
	}
	if !strings.Contains(body, "Nothing has been captured in this window.") {
		t.Error("a panel with nothing to count does not say so")
	}
}

// The rail's order is a decision (implementation.md, "Navigation") and the
// jump letter is derived from the link's own title, so the two cannot drift —
// but only if the title is there and says the right letter.
func TestTheDashboardSitsBetweenAuditAndSettings(t *testing.T) {
	s, _ := newTestServer(t)
	body := getDashboard(t, s)
	audit := strings.Index(body, `href="/audit"`)
	dash := strings.Index(body, `href="/dashboard"`)
	settings := strings.Index(body, `href="/settings"`)
	if audit < 0 || dash < 0 || settings < 0 {
		t.Fatalf("the rail is missing a row: audit=%d dashboard=%d settings=%d", audit, dash, settings)
	}
	if !(audit < dash && dash < settings) {
		t.Error("the Dashboard is not between Audit and Settings")
	}
	if !strings.Contains(body, `href="/dashboard" class="on" title="g d"`) {
		t.Error("the Dashboard's row does not carry its own jump letter while standing on it")
	}
}

// The Dashboard is the one screen with no rows and several windows of height,
// so `j` and `k` move it by section instead of by row (keys.md, "The map").
// The key layer knows nothing about panels — it moves through whatever the
// page marked — so the marking is the whole of the contract between the two,
// and a heading that lost its mark is a screenful `j` can no longer stop at.
func TestEveryHeadingOnTheDashboardIsASection(t *testing.T) {
	s, _ := newTestServer(t)
	body := getDashboard(t, s)
	// `main` is what the key layer looks in, and the page carries headings
	// outside it — the `?` panel has one — that are nobody's section
	main := body[strings.Index(body, "<main>"):strings.Index(body, "</main>")]
	if n := strings.Count(main, "data-kb-section"); n != 12 {
		t.Errorf("the Dashboard marks %d sections, want 12 — one per heading", n)
	}
	for _, tag := range []string{"<h2>", "<h3>"} {
		if strings.Contains(main, tag) {
			t.Errorf("a %s on the Dashboard carries no data-kb-section", tag)
		}
	}
}
