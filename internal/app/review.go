package app

import (
	"database/sql"
	"fmt"
	"time"
)

// An item is outstanding for the weekly review when it is not snoozed and
// its lastReviewedAt is older than a week. There is no global "last review"
// record; this is also how the app shows a review is due (design.md,
// "Weekly review").
const reviewWeek = 7 * 24 * time.Hour

func (a *App) reviewCutoff() time.Time { return a.now().UTC().Add(-reviewWeek) }

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

func (a *App) ReviewCounts() (*ReviewCounts, error) {
	c := &ReviewCounts{}
	cutoff := ts(a.reviewCutoff())
	today := a.Today()
	rows := []struct {
		dest  *int
		query string
	}{
		{&c.Inbox, `SELECT COUNT(*) FROM inbox_items`},
		{&c.WaitingFor, `SELECT COUNT(*) FROM actions WHERE became_next_at IS NOT NULL AND completed_at IS NULL
			AND assigned_to != '' AND last_reviewed_at < ? AND (snooze_until = '' OR snooze_until <= ?)`},
		{&c.Projects, `SELECT COUNT(*) FROM projects WHERE completed_at IS NULL
			AND last_reviewed_at < ? AND (snooze_until = '' OR snooze_until <= ?)`},
		{&c.Next, `SELECT COUNT(*) FROM actions WHERE became_next_at IS NOT NULL AND completed_at IS NULL
			AND assigned_to = '' AND last_reviewed_at < ? AND (snooze_until = '' OR snooze_until <= ?)`},
		{&c.Someday, `SELECT COUNT(*) FROM someday_items WHERE last_reviewed_at < ? AND (snooze_until = '' OR snooze_until <= ?)`},
		{&c.Schedules, `SELECT COUNT(*) FROM schedules WHERE last_reviewed_at < ?`},
	}
	for i, r := range rows {
		var err error
		switch i {
		case 0:
			err = a.db.QueryRow(r.query).Scan(r.dest)
		case 5:
			err = a.db.QueryRow(r.query, cutoff).Scan(r.dest)
		default:
			err = a.db.QueryRow(r.query, cutoff, today).Scan(r.dest)
		}
		if err != nil {
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
// review screens to show progress within a step.
func (a *App) Outstanding(lastReviewedAt time.Time, snoozeUntil string) bool {
	if snoozeUntil != "" && snoozeUntil > a.Today() {
		return false
	}
	return lastReviewedAt.Before(a.reviewCutoff())
}
