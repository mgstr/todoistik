// Package app owns the domain: the item types from design.md, the SQLite
// store they live in, and every operation the UI and APIs perform on them.
package app

import "time"

// Dates that mean a calendar day (due, snooze, schedule occurrences) are
// "YYYY-MM-DD" strings, "" meaning unset. Timestamps are time.Time in UTC.
const DateFormat = "2006-01-02"

type InboxItem struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
	// Source is the way in this capture arrived by — the app's own dialog, a
	// schedule firing, mail, Telegram, Reminders, an agent, a script. It is
	// asserted by whatever made the capture and never edited afterwards, which
	// is what a field counted over a year has to be (design.md, "Inbox item").
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"createdAt"`
}

// Line is the capture's first line: what every list of inbox items shows, and
// the only line anything reads (design.md, "Inbox item").
func (i *InboxItem) Line() string { l, _ := SplitCapture(i.Text); return l }

// Body is what the capture carried under that line — a reminder's note, a link
// to the mail a loop came in on. Empty for the ordinary one-line capture, and
// seen on the processing screen and nowhere else.
func (i *InboxItem) Body() string { _, b := SplitCapture(i.Text); return b }

type SomedayItem struct {
	ID   int64    `json:"id"`
	Text string   `json:"text"`
	Tags []string `json:"tags,omitempty"`
	// The idea stays raw — no outcome, no next action — but it carries the
	// area of responsibility it belongs to, which is the one thing the
	// monthly walk needs in order to be answerable (design.md, "Tags").
	CreatedAt      time.Time `json:"createdAt"`
	LastReviewedAt time.Time `json:"lastReviewedAt"`
}

type Duration string

// Three buckets, and deliberately no unit in any of them. Naming minutes made
// the field ask how long something takes, which is a question with no honest
// answer and one you have to stop and work out; naming sizes asks how big it
// feels, which you already know. Fewer buckets for the same reason — four
// meant deciding between two that were next to each other.
const (
	DurNone   Duration = ""
	DurShort  Duration = "short"
	DurMedium Duration = "medium"
	DurLong   Duration = "long"
)

var Durations = []Duration{DurShort, DurMedium, DurLong}

func (d Duration) Valid() bool {
	switch d {
	case DurNone, DurShort, DurMedium, DurLong:
		return true
	}
	return false
}

type Action struct {
	ID             int64      `json:"id"`
	ProjectID      int64      `json:"projectId,omitempty"` // 0 = standalone
	ProjectTitle   string     `json:"projectTitle,omitempty"`
	Title          string     `json:"title"`
	Context        string     `json:"context,omitempty"`      // "grocery", no "@"
	ContextParam   string     `json:"contextParam,omitempty"` // "Selver"
	Duration       Duration   `json:"duration,omitempty"`
	NeedsFocus     bool       `json:"needsFocus,omitempty"`
	Description    string     `json:"description,omitempty"`
	AssignedTo     string     `json:"assignedTo,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	DueDate        string     `json:"dueDate,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	LastReviewedAt time.Time  `json:"lastReviewedAt"`
	BecameNextAt   *time.Time `json:"becameNextActionAt,omitempty"`
	SnoozeUntil    string     `json:"snoozeUntil,omitempty"`
	// SnoozeActionID is the sibling this action waits on, 0 when it waits on
	// nothing. It is the other half of the snooze: a date says when this
	// becomes workable and an action says what has to happen first, and an
	// action carries at most one of the two (design.md, "Time fields").
	SnoozeActionID    int64      `json:"snoozeActionId,omitempty"`
	SnoozeActionTitle string     `json:"snoozeActionTitle,omitempty"` // derived, for reading and writing the line
	CompletedAt       *time.Time `json:"completedAt,omitempty"`
}

// IsOpen reports work still in hand: not completed. It is what keeps a project
// out of the stalled check — a project holding anything open has something to
// do, even where that something is snoozed or delegated and is therefore not
// what the project's next action is (design.md, "Stalled projects").
func (a *Action) IsOpen() bool { return a.CompletedAt == nil }

func (a *Action) IsWaiting() bool { return a.AssignedTo != "" && a.CompletedAt == nil }

// IsSnoozed covers both halves. A blocker is cleared the moment it is
// completed or deleted, so a set SnoozeActionID always means still waiting —
// there is no date to compare it against and none is needed.
func (a *Action) IsSnoozed(today string) bool {
	return a.SnoozeActionID != 0 || (a.SnoozeUntil != "" && a.SnoozeUntil > today)
}

// SnoozeLabel is what the snooze is shown as, "" when there is none. One
// badge says both halves, because to every view that shows it they are the
// same fact: this is not workable yet.
func (a *Action) SnoozeLabel(today string) string {
	if a.SnoozeActionID != 0 {
		return "zzz until " + a.SnoozeActionTitle
	}
	if a.SnoozeUntil != "" && a.SnoozeUntil > today {
		return "zzz until " + a.SnoozeUntil
	}
	return ""
}

func (a *Action) IsOverdue(today string) bool {
	return a.DueDate != "" && a.CompletedAt == nil && a.DueDate < today
}

// ContextLabel renders "@grocery(Selver)" or "@grocery", "" when no context.
func (a *Action) ContextLabel() string {
	if a.Context == "" {
		return ""
	}
	if a.ContextParam != "" {
		return "@" + a.Context + "(" + a.ContextParam + ")"
	}
	return "@" + a.Context
}

// Errors returns the action's error states, per design.md "Error state".
func (a *Action) Errors() []string {
	var errs []string
	if a.SnoozeUntil != "" && a.DueDate != "" && a.SnoozeUntil > a.DueDate {
		errs = append(errs, "snoozed past its due date")
	}
	return errs
}

type Project struct {
	ID             int64     `json:"id"`
	Title          string    `json:"title"`
	DOD            string    `json:"dod"`
	Tags           []string  `json:"tags,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	LastReviewedAt time.Time `json:"lastReviewedAt"`
	SnoozeUntil    string    `json:"snoozeUntil,omitempty"`
	// NextActionID is the action this project has been pointed at, 0 when it
	// has been pointed at none. It is a preference and not the answer: what the
	// project's next action *is* is NextAction, which reads this and falls back
	// to the plan (design.md, "Project").
	NextActionID int64      `json:"nextActionId,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
	Actions      []*Action  `json:"actions,omitempty"`
	Stalled      bool       `json:"stalled,omitempty"` // derived, filled by queries
}

func (p *Project) IsSnoozed(today string) bool {
	return p.SnoozeUntil != "" && p.SnoozeUntil > today
}

// ComputeStalled derives the stalled state from the loaded actions: an active,
// unsnoozed project with nothing open at all.
//
// Nothing open, and not "no next action" — the two came apart when a project
// stopped counting every available step as one. A project whose every open
// action is snoozed has no next action and is not stalled: it is waiting on a
// date or on a sibling, and waiting is a plan, where stalled is the absence of
// one (design.md, "Stalled projects"). A delegated action counts the same way
// and always has.
func (p *Project) ComputeStalled(today string) bool {
	if p.CompletedAt != nil || p.IsSnoozed(today) {
		return false
	}
	for _, a := range p.Actions {
		if a.IsOpen() {
			return false
		}
	}
	return true
}

// ActionNode is one line of a project's action list: the action, and how deep
// it sits under whatever it is waiting on.
type ActionNode struct {
	*Action
	Depth int `json:"depth,omitempty"`
}

// ActionTree is the project's action list ordered so that an action waiting on
// a sibling is drawn directly under it, one level in. The plan is read to see
// what comes before what, and a dependency written as a flat badge made that
// order something you had to reconstruct by eye; nesting says it by shape.
//
// Depth is capped at what the cycle check guarantees — the graph is a forest,
// since an action waits on at most one other and never on itself — so this
// walk terminates without needing a visited set. Anything whose blocker is not
// in the list (it has left the project, or the list is a filtered one) is
// drawn as a root, because a line with nothing above it is still a line.
func (p *Project) ActionTree() []ActionNode {
	children := map[int64][]*Action{}
	present := map[int64]bool{}
	for _, a := range p.Actions {
		present[a.ID] = true
	}
	var roots []*Action
	for _, a := range p.Actions {
		if a.SnoozeActionID != 0 && present[a.SnoozeActionID] {
			children[a.SnoozeActionID] = append(children[a.SnoozeActionID], a)
			continue
		}
		roots = append(roots, a)
	}
	out := make([]ActionNode, 0, len(p.Actions))
	var walk func(act *Action, depth int)
	walk = func(act *Action, depth int) {
		out = append(out, ActionNode{Action: act, Depth: depth})
		for _, c := range children[act.ID] {
			walk(c, depth+1)
		}
	}
	for _, r := range roots {
		walk(r, 0)
	}
	return out
}

// NextAction is the project's one next action: the action it has been pointed
// at while that action can still be it, and otherwise the first open, unsnoozed
// action in ActionTree order. Nil when every open action is snoozed, and nil
// when nothing is open at all — which is the stalled project.
//
// Derived on every read, and that is the point of it. The pointing is stored,
// but what it means has to be worked out now: a snooze runs out with the
// calendar and nothing writes when it does, so a project told once which action
// is next would sit with an empty field on the morning that field should have
// filled itself. Falling back to the plan is also what keeps a project nobody
// has pointed at reading exactly as it did before there was anything to point
// with (design.md, "Project").
//
// A delegated action is a candidate like any other: being one is what keeps a
// project whose only move is somebody else's out of the stalled check, and out
// of the "Next actions" view along with it. A snoozed action is not a
// candidate, which is the whole of what a snooze is for.
func (p *Project) NextAction(today string) *Action {
	if p.NextActionID != 0 {
		for _, act := range p.Actions {
			if act.ID == p.NextActionID && canBeNext(act, today) {
				return act
			}
		}
	}
	for _, n := range p.ActionTree() {
		if canBeNext(n.Action, today) {
			return n.Action
		}
	}
	return nil
}

// canBeNext is what a next action has to be: open, and workable now. One
// definition, read by the derivation above and by the app before it writes a
// pointing down (see MakeNext), so that the mark a row offers and the answer
// the page draws can never disagree about which actions are eligible.
func canBeNext(act *Action, today string) bool {
	return act.CompletedAt == nil && !act.IsSnoozed(today)
}

// RestTree is the plan without the line the next action is on: what a
// project's page lists under "More actions", the action shown open above it
// having been lifted out.
//
// The depths are the plan's own, untouched. Anything that was waiting on the
// next action stays one level in, and it still reads correctly, because what
// it is indented under is directly above the list — the open boxes. Re-rooting
// those rows would say they wait on nothing, which is the one thing the shape
// exists to say.
func (p *Project) RestTree(today string) []ActionNode {
	next := p.NextAction(today)
	if next == nil {
		return p.ActionTree()
	}
	full := p.ActionTree()
	out := make([]ActionNode, 0, len(full))
	for _, n := range full {
		if n.Action.ID != next.ID {
			out = append(out, n)
		}
	}
	return out
}

// OpenActions is everything the project still holds, next or not — what the
// stalled check counts and what refuses to let the project be completed.
func (p *Project) OpenActions() []*Action {
	var open []*Action
	for _, a := range p.Actions {
		if a.CompletedAt == nil {
			open = append(open, a)
		}
	}
	return open
}

func (p *Project) Errors() []string {
	if p.DOD == "" && p.CompletedAt == nil {
		return []string{"no definition of done"}
	}
	return nil
}

type Schedule struct {
	ID             int64      `json:"id"`
	Text           string     `json:"text"`
	Rule           string     `json:"rule"` // "YYYY-MM-DD" or a 3-field cron expression
	Suffix         string     `json:"suffix,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	CountedFrom    string     `json:"countedFrom"` // occurrences on/after this day count
	LastFiredAt    *time.Time `json:"lastFiredAt,omitempty"`
	LastReviewedAt time.Time  `json:"lastReviewedAt"`
	NextFire       string     `json:"nextFire,omitempty"`     // derived
	RuleReadable   string     `json:"ruleReadable,omitempty"` // derived
}

// IsOneShot reports whether the rule is a single date.
func (s *Schedule) IsOneShot() bool {
	_, err := time.Parse(DateFormat, s.Rule)
	return err == nil
}

type AuditEntry struct {
	ID       int64     `json:"id"`
	At       time.Time `json:"at"`
	Event    string    `json:"event"`
	ItemType string    `json:"itemType"`
	ItemID   int64     `json:"itemId,omitempty"`
	Snapshot string    `json:"snapshot,omitempty"` // JSON of the item as it was
}

// Audit event names.
const (
	EvCreated         = "created"
	EvEdited          = "edited"
	EvCompleted       = "completed"
	EvUncompleted     = "uncompleted"
	EvTrashed         = "trashed"
	EvDeleted         = "deleted"
	EvDetached        = "detached"
	EvPromoted        = "promoted"
	EvReference       = "sent-to-reference"
	EvReturned        = "returned-to-inbox"
	EvTwoMinute       = "two-minute-rule"
	EvConfirmed       = "completion-confirmed"
	EvFired           = "fired"
	EvScheduleExpired = "one-shot-expired"
	EvReviewed        = "reviewed"
	EvUnreviewed      = "unreviewed"

	// The Inbox Zero branches that create something. They used to write
	// EvDeleted, which said the wrong thing twice: on the Audit screen an item
	// that had become a task was recorded as deleted, and in the log the
	// commonest answers of a normal week were one undifferentiated bucket — so
	// "what does my inbox turn into" had no answer, though every other branch
	// had recorded its own answer since the first capture (see design.md,
	// "Audit entry"). Entries already written stay as they are: the audit log
	// is never rewritten (implementation.md, "Schema changes").
	//
	// Task and Action are two branches and two entries, because they make two
	// different things: a standalone action, which is a task everywhere it is
	// afterwards read, or an action inside a project. That is the same split
	// Match.Kind draws on the screen they are answered from
	// (implementation.md, "The match list") — the item type is `action` either
	// way, and the noun is where the thing is found.
	EvBecameTask    = "became-a-task"
	EvBecameAction  = "became-an-action"
	EvBecameProject = "became-a-project"
	EvBecameSomeday = "became-someday"
)

// TodayTag is the built-in pick-for-the-day tag. It is not on the editable
// tag list, expires daily and is never audited.
const TodayTag = "today"
