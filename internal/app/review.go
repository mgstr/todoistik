package app

import (
	"database/sql"
	"encoding/json"
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

// reviewTable is where an item type keeps its lastReviewedAt. Only the four
// types the review walks have one — an inbox item has none, and is governed
// by Inbox Zero instead (design.md, "Weekly review").
func reviewTable(itemType string) (string, error) {
	switch itemType {
	case "action":
		return "actions", nil
	case "project":
		return "projects", nil
	case "someday":
		return "someday_items", nil
	case "schedule":
		return "schedules", nil
	}
	return "", fmt.Errorf("nothing to review on %q", itemType)
}

// reviewStamp is what the audit keeps of a review: the stamp that review
// replaced. Without it the mark could only ever go on — taking it off again
// would have to invent a date, and an invented one either leaves the item
// outstanding forever or hides it for a week nobody asked for.
type reviewStamp struct {
	LastReviewedAt time.Time `json:"lastReviewedAt"`
}

// MarkReviewed stamps lastReviewedAt — the deliberate act of walking an
// item during review. Edits never stamp it; only this does.
func (a *App) MarkReviewed(itemType string, id int64) error {
	table, err := reviewTable(itemType)
	if err != nil {
		return err
	}
	return a.tx(func(tx *sql.Tx) error {
		var was string
		if err := tx.QueryRow(`SELECT last_reviewed_at FROM `+table+` WHERE id=?`, id).Scan(&was); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE `+table+` SET last_reviewed_at=? WHERE id=?`, ts(a.now()), id); err != nil {
			return err
		}
		return a.audit(tx, EvReviewed, itemType, id, reviewStamp{LastReviewedAt: parseTS(was)})
	})
}

// UnmarkReviewed puts back the stamp the last MarkReviewed replaced, which
// is what lets the mark be a thing you can be wrong about: a row marked by a
// press that landed on the wrong line goes back to being outstanding, rather
// than dropping silently out of the walk for a week.
//
// With nothing recorded — an item last reviewed before the audit carried the
// old stamp — it goes back to its creation date, which is where
// lastReviewedAt started (design.md, "Time fields"). That is outstanding for
// anything old enough to be in a review, and honest about knowing no more
// than that.
func (a *App) UnmarkReviewed(itemType string, id int64) error {
	table, err := reviewTable(itemType)
	if err != nil {
		return err
	}
	return a.tx(func(tx *sql.Tx) error {
		var now string
		if err := tx.QueryRow(`SELECT last_reviewed_at FROM `+table+` WHERE id=?`, id).Scan(&now); err != nil {
			return err
		}
		var snap string
		err := tx.QueryRow(`SELECT snapshot FROM audit_log
			WHERE event=? AND item_type=? AND item_id=? AND snapshot!=''
			ORDER BY id DESC LIMIT 1`, EvReviewed, itemType, id).Scan(&snap)
		var was string
		switch {
		case err == sql.ErrNoRows:
			if err := tx.QueryRow(`SELECT created_at FROM `+table+` WHERE id=?`, id).Scan(&was); err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			var st reviewStamp
			if err := json.Unmarshal([]byte(snap), &st); err != nil {
				return err
			}
			was = ts(st.LastReviewedAt)
		}
		if _, err := tx.Exec(`UPDATE `+table+` SET last_reviewed_at=? WHERE id=?`, was, id); err != nil {
			return err
		}
		return a.audit(tx, EvUnreviewed, itemType, id, reviewStamp{LastReviewedAt: parseTS(now)})
	})
}

// ToggleReviewed flips the mark and reports which side it landed on. One key
// presses both directions, so both are decided here rather than by the page:
// a screen that had been open a while would otherwise unmark an item on the
// strength of what it looked like when it was drawn.
func (a *App) ToggleReviewed(itemType string, id int64) (bool, error) {
	table, err := reviewTable(itemType)
	if err != nil {
		return false, err
	}
	var at string
	if err := a.db.QueryRow(`SELECT last_reviewed_at FROM `+table+` WHERE id=?`, id).Scan(&at); err != nil {
		return false, err
	}
	if a.outstandingFor(itemType)(parseTS(at)) {
		return true, a.MarkReviewed(itemType, id)
	}
	return false, a.UnmarkReviewed(itemType, id)
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

// OutstandingFor is the test one item type answers to. The two cadences are
// picked apart in one place so that a screen walking four types at once
// cannot judge a someday/maybe item by the week everything else gets.
func (a *App) OutstandingFor(itemType string) func(time.Time) bool {
	return a.outstandingFor(itemType)
}

func (a *App) outstandingFor(itemType string) func(time.Time) bool {
	if itemType == "someday" {
		return a.SomedayOutstanding
	}
	return a.Outstanding
}
