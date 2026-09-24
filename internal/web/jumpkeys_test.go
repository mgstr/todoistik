package web

import (
	"strings"
	"testing"
)

// Four of the fourteen view-jump letters are also button letters — `d` is Done
// and the Dashboard, and `t`, `r` and `w` are spent twice the same way (keys.md,
// "The map"). What makes that safe is the `g` prefix: while a jump is pending,
// the next key is the jump and nothing else. That rule is not a line of code
// anywhere, it is an ordering — the pending-`g` branch has to be read before the
// row commands — and when the order was the other way round every one of the
// four silently did the button instead, on any screen with a row under the
// cursor. Nothing about the page or the rendered HTML was wrong, which is why
// no other test here could see it.
//
// The keyboard layer is JavaScript and is not otherwise tested from Go (see
// anim_test.go). This is the exception because it is the one invariant that
// made a documented rule quietly false, and because it costs a substring search
// to hold.

func TestAPendingJumpIsReadBeforeTheRowCommands(t *testing.T) {
	b, err := staticFS.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	const handler = `document.addEventListener("keydown"`
	start := strings.Index(src, handler)
	if start < 0 {
		t.Fatalf("no document keydown handler in app.js: the search below means nothing")
	}
	body := src[start:]

	pending := strings.Index(body, "if (gPending) {")
	if pending < 0 {
		t.Fatalf("no pending-jump branch in the keydown handler")
	}
	// Both of them: the handler calls rowCommand twice, once above the ctrl
	// guard for modifier mode and once below it for the bare letters, and a
	// pending jump has to come before each. Checking only the first would have
	// passed while `^d` still fired Done with the overlay up.
	first := strings.Index(body, "rowCommand(e)")
	last := strings.LastIndex(body, "rowCommand(e)")
	if first < 0 {
		t.Fatalf("no row-command call in the keydown handler")
	}
	if pending > first || pending > last {
		t.Errorf("a row command is read before the pending `g`, so `g d` presses the button "+
			"instead of opening the Dashboard (gPending at %d, rowCommand at %d and %d)",
			pending, first, last)
	}
}

// The other half of the same rule: a jump is bare in every mode, so the pending
// branch must let a chord past rather than answer it. Without this the branch
// would swallow `^v` and ctrl-enter whenever an overlay happened to be up.
func TestAPendingJumpAnswersOnlyBareKeys(t *testing.T) {
	b, err := staticFS.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	start := strings.Index(src, `document.addEventListener("keydown"`)
	if start < 0 {
		t.Fatal("no document keydown handler in app.js")
	}
	from := strings.Index(src[start:], "if (gPending) {")
	branch := src[start+from:]
	if end := strings.Index(branch, "\n    }"); end > 0 {
		branch = branch[:end]
	}
	if !strings.Contains(branch, "e.ctrlKey") {
		t.Error("the pending-jump branch does not check for a modifier, so a chord pressed " +
			"while the overlay is up would be taken for a jump")
	}
}
