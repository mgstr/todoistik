package app

import (
	"errors"
	"testing"
	"time"

	"todoistik/internal/request"
)

// one open action to be reported finished, and the request line naming it
func anAction(t *testing.T, a *App) *Action {
	t.Helper()
	act, err := a.CreateAction(0, ActionFields{Title: "залогировать кеш 8 сентября"}, false)
	if err != nil {
		t.Fatal(err)
	}
	return act
}

func aRequest(id int64, at time.Time) string {
	return request.Write(request.Line{Source: "reminders", ItemID: id, At: at, Name: "залогировать кеш 8 сентября"})
}

// A completion request is resolved against what the app actually holds, and
// the verdict is what decides whether there is anything to confirm.
func TestReadCompletionRequest(t *testing.T) {
	a, _ := newTestApp(t)
	act := anAction(t, a)
	ticked := time.Date(2026, 9, 8, 14, 30, 0, 0, time.UTC)

	t.Run("an ordinary capture is not a request", func(t *testing.T) {
		_, ok, err := a.ReadCompletionRequest("Buy new winter tyres")
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})

	t.Run("an open action can be completed from it", func(t *testing.T) {
		req, ok, err := a.ReadCompletionRequest(aRequest(act.ID, ticked))
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if req.Verdict != RequestOpen {
			t.Fatalf("Verdict = %q, want open", req.Verdict)
		}
		if req.Action == nil || req.Action.ID != act.ID {
			t.Fatalf("Action = %+v, want the one it names", req.Action)
		}
		if !req.At.Equal(ticked) {
			t.Errorf("At = %v, want %v", req.At, ticked)
		}
	})

	t.Run("an action nobody has is gone", func(t *testing.T) {
		req, ok, err := a.ReadCompletionRequest(aRequest(9999, ticked))
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if req.Verdict != RequestGone {
			t.Errorf("Verdict = %q, want gone", req.Verdict)
		}
	})
}

// Completing at the desk first is the case both sides did the same work: there
// is nothing left to confirm, and the screen has to be able to say so.
func TestReadCompletionRequestAlreadyDone(t *testing.T) {
	a, _ := newTestApp(t)
	act := anAction(t, a)
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	req, ok, err := a.ReadCompletionRequest(aRequest(act.ID, time.Now()))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if req.Verdict != RequestDone {
		t.Fatalf("Verdict = %q, want done", req.Verdict)
	}
	if req.Action == nil || req.Action.CompletedAt == nil {
		t.Errorf("the screen has to be able to say when: %+v", req.Action)
	}
}

// An action promoted into a project is gone, and it is the one disappearance
// that is not a loss - so the request says which of the two it was.
func TestReadCompletionRequestPromoted(t *testing.T) {
	a, now := newTestApp(t)
	act := anAction(t, a)
	ticked := now.Add(-time.Hour) // ticked on the phone, then promoted at the desk
	if _, err := a.Promote(act.ID, ProjectFields{Title: "365 дней", DOD: "a year of caches"},
		[]ActionFields{{Title: "залогировать кеш 9 сентября"}}); err != nil {
		t.Fatal(err)
	}
	// the promotion handed this very id to the project's first action, because
	// an id is a rowid and nothing reserves the ones given back
	if reborn, err := a.Action(act.ID); err == nil && reborn.Title == act.Title {
		t.Fatal("this test no longer exercises a reused id")
	}
	req, ok, err := a.ReadCompletionRequest(aRequest(act.ID, ticked))
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if req.Verdict != RequestPromoted {
		t.Errorf("Verdict = %q, want promoted", req.Verdict)
	}
}

// Confirming one is the ordinary completion, at the moment the work was
// actually finished - which is the whole reason the request carries a time.
func TestProcessCompletion(t *testing.T) {
	a, now := newTestApp(t)
	act := anAction(t, a) // created Friday the 4th, on the test clock
	ticked := time.Date(2026, 9, 5, 14, 30, 0, 0, time.UTC)
	*now = time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC) // and answered two days later

	item, _, err := a.Capture(aRequest(act.ID, ticked))
	if err != nil {
		t.Fatal(err)
	}
	done, err := a.ProcessCompletion(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.CompletedAt == nil {
		t.Fatal("the action was not completed")
	}
	if !done.CompletedAt.Equal(ticked) {
		t.Errorf("completedAt = %v, want the moment it was ticked (%v), not the moment of the answer (%v)",
			done.CompletedAt, ticked, *now)
	}

	// the request leaves the inbox, and the log holds both halves
	items, _ := a.Inbox()
	if len(items) != 0 {
		t.Errorf("inbox still holds %d items", len(items))
	}
	log, _ := a.AuditLog(10)
	var completed, confirmed bool
	for _, e := range log {
		if e.Event == EvCompleted && e.ItemID == act.ID {
			completed = true
		}
		if e.Event == EvConfirmed && e.ItemID == item.ID {
			confirmed = true
		}
	}
	if !completed || !confirmed {
		t.Errorf("audit log: completed=%v confirmed=%v", completed, confirmed)
	}
}

// Ignoring is the trash branch, and it has to leave the action exactly as it
// was: the answer given was that it is not done.
func TestIgnoringACompletionRequestLeavesTheAction(t *testing.T) {
	a, _ := newTestApp(t)
	act := anAction(t, a)
	item, _, err := a.Capture(aRequest(act.ID, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ProcessTrash(item.ID); err != nil {
		t.Fatal(err)
	}
	still, err := a.Action(act.ID)
	if err != nil {
		t.Fatal(err)
	}
	if still.CompletedAt != nil {
		t.Error("ignoring a request completed the action")
	}
	if !still.IsNext() {
		t.Error("ignoring a request took the action off the next list")
	}
}

// The screen never offers the button for an action that cannot be completed,
// so this is the answer that arrives anyway: a second tab, a back button, or a
// form posted after the action was completed at the desk.
func TestProcessCompletionRefusesWhatItCannotComplete(t *testing.T) {
	a, _ := newTestApp(t)
	act := anAction(t, a)

	item, _, err := a.Capture(aRequest(act.ID, time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.CompleteAction(act.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProcessCompletion(item.ID); !errors.Is(err, ErrRequestNotOpen) {
		t.Fatalf("err = %v, want ErrRequestNotOpen", err)
	}
	// and the request is still in the inbox, to be thrown away deliberately
	items, _ := a.Inbox()
	if len(items) != 1 {
		t.Errorf("inbox holds %d items, want the request still there", len(items))
	}

	plain, _, err := a.Capture("Buy new winter tyres")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProcessCompletion(plain.ID); !errors.Is(err, ErrRequestNotOpen) {
		t.Errorf("an ordinary capture is not confirmable: err = %v", err)
	}
}
