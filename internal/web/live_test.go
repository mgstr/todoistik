package web

import "testing"

// The background refresh has one rule that is worth a test rather than a
// careful read: which screens will let it replace what is on them. A list may
// be replaced; a form holds half-written words and may not. Getting that
// wrong is not a stale number, it is typing thrown away, so the guard is
// pinned here — see implementation.md, "Keeping an open page current".

func TestAPlainListIsRefreshable(t *testing.T) {
	for _, view := range []string{"inbox", "today", "next", "projects", "tasks",
		"waiting", "calendar", "someday", "scheduler", "archive", "audit"} {
		if !liveViews[view] {
			t.Errorf("%q is a plain list and does not refresh itself", view)
		}
	}
}

// Review is a stepper and Settings is a form; neither gains a row because
// something arrived, and both would lose what is on them to a swap.
func TestAStepperAndAFormAreNot(t *testing.T) {
	for _, view := range []string{"review", "settings"} {
		if liveViews[view] {
			t.Errorf("%q would be replaced under whoever is working in it", view)
		}
	}
}

// The screens that carry a list view's own name are the dangerous ones: the
// processing screen is "inbox", an action opened from Next is "next". What
// tells them apart is the second crumb, so step() is what has to clear the
// flag — a list of names would have to be remembered every time a screen is
// added, and this cannot be forgotten.
func TestAStepInsideAViewStopsRefreshing(t *testing.T) {
	p := &page{View: "inbox", Live: liveViews["inbox"]}
	if !p.Live {
		t.Fatal("the inbox is a list and should start refreshable")
	}
	if p.step("Processing", "processing").Live {
		t.Error("the processing screen would have its main swapped out from under it")
	}
}
