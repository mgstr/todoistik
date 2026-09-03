// Package app owns the domain: the item types from design.md, the SQLite
// store they live in, and every operation the UI and APIs perform on them.
package app

import "time"

// Dates that mean a calendar day (due, snooze, schedule occurrences) are
// "YYYY-MM-DD" strings, "" meaning unset. Timestamps are time.Time in UTC.
const DateFormat = "2006-01-02"

type InboxItem struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type SomedayItem struct {
	ID             int64     `json:"id"`
	Text           string    `json:"text"`
	CreatedAt      time.Time `json:"createdAt"`
	LastReviewedAt time.Time `json:"lastReviewedAt"`
	SnoozeUntil    string    `json:"snoozeUntil,omitempty"`
}

type Duration string

const (
	DurNone Duration = ""
	Dur5    Duration = "<5min"
	Dur15   Duration = "<15min"
	Dur1h   Duration = "<1h"
	DurLong Duration = ">1h"
)

var Durations = []Duration{Dur5, Dur15, Dur1h, DurLong}

func (d Duration) Valid() bool {
	switch d {
	case DurNone, Dur5, Dur15, Dur1h, DurLong:
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
	BecameNextAt   *time.Time `json:"becameNextActionAt,omitempty"` // nil = parked
	SnoozeUntil    string     `json:"snoozeUntil,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

// IsNext reports a real next action: becameNextActionAt set, not completed.
func (a *Action) IsNext() bool { return a.BecameNextAt != nil && a.CompletedAt == nil }

func (a *Action) IsWaiting() bool { return a.AssignedTo != "" && a.CompletedAt == nil }

func (a *Action) IsSnoozed(today string) bool {
	return a.SnoozeUntil != "" && a.SnoozeUntil > today
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
	if a.AssignedTo != "" && a.BecameNextAt == nil && a.CompletedAt == nil {
		errs = append(errs, "assigned but not a next action")
	}
	return errs
}

type Project struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	DOD            string     `json:"dod"`
	Description    string     `json:"description,omitempty"`
	Tags           []string   `json:"tags,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	LastReviewedAt time.Time  `json:"lastReviewedAt"`
	SnoozeUntil    string     `json:"snoozeUntil,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	Actions        []*Action  `json:"actions,omitempty"`
	Stalled        bool       `json:"stalled,omitempty"` // derived, filled by queries
}

func (p *Project) IsSnoozed(today string) bool {
	return p.SnoozeUntil != "" && p.SnoozeUntil > today
}

// ComputeStalled derives the stalled state from the loaded actions: an
// active, unsnoozed project with no next action. A waiting-for or snoozed
// action still counts as a next action of its project.
func (p *Project) ComputeStalled(today string) bool {
	if p.CompletedAt != nil || p.IsSnoozed(today) {
		return false
	}
	for _, a := range p.Actions {
		if a.IsNext() {
			return false
		}
	}
	return true
}

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
	EvTwoMinute       = "two-minute-rule"
	EvFired           = "fired"
	EvScheduleExpired = "one-shot-expired"
	EvReviewed        = "reviewed"
)

// TodayTag is the built-in pick-for-the-day tag. It is not on the editable
// tag list, expires daily and is never audited.
const TodayTag = "today"
