package app

import (
	"database/sql"
	"fmt"
	"time"
)

// An item is outstanding for the weekly review when its lastReviewedAt is
// older than its review period. There is no global "last review" record;
// this is also how the app shows a review is due (design.md, "Weekly
// review").
//
// The period is a week for everything except someday/maybe items, whose
// period is the settings file's review.someday_days (a month by default):
// a parked idea does not change week to week, and a review that walks the
// whole parking lot every time is a review that gets skipped. Snoozing
// exempts nothing here — a snooze date is itself one of the claims the
// review checks.
const reviewWeek = 7 * 24 * time.Hour

func (a *App) reviewCutoff() time.Time { return a.now().UTC().Add(-reviewWeek) }

func (a *App) somedayReviewCutoff() time.Time {
	return a.now().UTC().AddDate(0, 0, -a.somedayReviewDays)
}

// SetSomedayReviewDays wires in the settings file's review.someday_days,
// once, at startup. Open seeds the same default the file has, so this is
// only needed where a settings file is actually read.
func (a *App) SetSomedayReviewDays(days int) { a.somedayReviewDays = days }

// ReviewCounts is what the review screen (and the nav badge) shows: how
// many items each step still has outstanding.
type ReviewCounts struct {
	Inbox      int `json:"inbox"`
	WaitingFor int `json:"waitingFor"`
	Projects   int `json:"projects"`
	Next       int `json:"nextActions"`
	Someday    int `json:"someday"`
	Schedules  int `json:"schedules"`
}

func (c ReviewCounts) Total() int {
	return c.Inbox + c.WaitingFor + c.Projects + c.Next + c.Someday + c.Schedules
}

// OutstandingTotal excludes the inbox: an inbox item has no lastReviewedAt
// and is governed by a different rule entirely (Inbox Zero must run until
// it is empty, not until a week has passed) - see design.md, "Inbox" and
// "Weekly review". This is what the nav's Review badge counts; the raw
// inbox size is its own signal, carried by the Inbox nav entry instead.
func (c ReviewCounts) OutstandingTotal() int {
	return c.WaitingFor + c.Projects + c.Next + c.Someday + c.Schedules
}

func (a *App) ReviewCounts() (*ReviewCounts, error) {
	c := &ReviewCounts{}
	cutoff, somedayCutoff := ts(a.reviewCutoff()), ts(a.somedayReviewCutoff())
	rows := []struct {
		dest  *int
		query string
		args  []any
	}{
		{&c.Inbox, `SELECT COUNT(*) FROM inbox_items`, nil},
		{&c.WaitingFor, `SELECT COUNT(*) FROM actions WHERE became_next_at IS NOT NULL AND completed_at IS NULL
			AND assigned_to != '' AND last_reviewed_at < ?`, []any{cutoff}},
		{&c.Projects, `SELECT COUNT(*) FROM projects WHERE completed_at IS NULL AND last_reviewed_at < ?`, []any{cutoff}},
		{&c.Next, `SELECT COUNT(*) FROM actions WHERE became_next_at IS NOT NULL AND completed_at IS NULL
			AND assigned_to = '' AND last_reviewed_at < ?`, []any{cutoff}},
		{&c.Someday, `SELECT COUNT(*) FROM someday_items WHERE last_reviewed_at < ?`, []any{somedayCutoff}},
		{&c.Schedules, `SELECT COUNT(*) FROM schedules WHERE last_reviewed_at < ?`, []any{cutoff}},
	}
	for _, r := range rows {
		if err := a.db.QueryRow(r.query, r.args...).Scan(r.dest); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// MarkReviewed stamps lastReviewedAt — the deliberate act of walking an
// item during review. Edits never stamp it; only this does.
func (a *App) MarkReviewed(itemType string, id int64) error {
	var table string
	switch itemType {
	case "action":
		table = "actions"
	case "project":
		table = "projects"
	case "someday":
		table = "someday_items"
	case "schedule":
		table = "schedules"
	default:
		return fmt.Errorf("nothing to review on %q", itemType)
	}
	return a.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE `+table+` SET last_reviewed_at=? WHERE id=?`, ts(a.now()), id); err != nil {
			return err
		}
		return a.audit(tx, EvReviewed, itemType, id, nil)
	})
}

// Outstanding reports whether one loaded item is still outstanding, used by
// review screens to show progress within a step. Someday/maybe items are
// judged by SomedayOutstanding instead — see the comment at the top.
func (a *App) Outstanding(lastReviewedAt time.Time) bool {
	return lastReviewedAt.Before(a.reviewCutoff())
}

// SomedayOutstanding is Outstanding on the someday/maybe cadence.
func (a *App) SomedayOutstanding(lastReviewedAt time.Time) bool {
	return lastReviewedAt.Before(a.somedayReviewCutoff())
}
