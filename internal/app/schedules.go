package app

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"todoistik/internal/cron"
)

// ValidateRule checks a schedule's When: a single date or a 3-field cron
// expression, validated when the schedule is saved.
func ValidateRule(rule string) error {
	if ValidDate(rule) {
		return nil
	}
	_, err := cron.Parse(rule)
	if err != nil {
		return fmt.Errorf("neither a date nor a cron rule: %w", err)
	}
	return nil
}

// ruleReadable renders the rule for the Scheduler view.
func ruleReadable(rule string) string {
	if ValidDate(rule) {
		return "once, on " + rule
	}
	if e, err := cron.Parse(rule); err == nil {
		return e.Readable()
	}
	return rule
}

// nextFire returns the first day on/after `from` the rule fires, "" if none.
func nextFire(rule, from string) string {
	if ValidDate(rule) {
		if rule >= from {
			return rule
		}
		return "" // a passed one-shot fires on the next day-start, then deletes itself
	}
	e, err := cron.Parse(rule)
	if err != nil {
		return ""
	}
	day, _ := time.Parse(DateFormat, from)
	if e.Matches(day) {
		return from
	}
	if n := e.NextAfter(day); !n.IsZero() {
		return n.Format(DateFormat)
	}
	return ""
}

func (a *App) CreateSchedule(text, rule, suffix string) (*Schedule, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrEmpty
	}
	if err := ValidateRule(rule); err != nil {
		return nil, err
	}
	s := &Schedule{
		Text: text, Rule: rule, Suffix: suffix,
		CreatedAt: a.now().UTC(), CountedFrom: a.Today(), LastReviewedAt: a.now().UTC(),
	}
	err := a.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`INSERT INTO schedules (text, rule, suffix, created_at, counted_from, last_reviewed_at) VALUES (?,?,?,?,?,?)`,
			s.Text, s.Rule, s.Suffix, ts(s.CreatedAt), s.CountedFrom, ts(s.LastReviewedAt))
		if err != nil {
			return err
		}
		s.ID, _ = res.LastInsertId()
		return a.audit(tx, EvCreated, "schedule", s.ID, s)
	})
	if err != nil {
		return nil, err
	}
	return s, nil
}

// EditSchedule updates a schedule. Changing the rule restarts the occurrence
// count from today: past occurrences of a rule that was not yet in place
// were never missed (design.md, "Schedule").
func (a *App) EditSchedule(id int64, text, rule, suffix string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return ErrEmpty
	}
	if err := ValidateRule(rule); err != nil {
		return err
	}
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.scheduleTx(tx, id)
		if err != nil {
			return err
		}
		countedFrom := before.CountedFrom
		if rule != before.Rule {
			countedFrom = a.Today()
		}
		if _, err := tx.Exec(`UPDATE schedules SET text=?, rule=?, suffix=?, counted_from=? WHERE id=?`,
			text, rule, suffix, countedFrom, id); err != nil {
			return err
		}
		return a.audit(tx, EvEdited, "schedule", id, before)
	})
}

func (a *App) DeleteSchedule(id int64) error {
	return a.tx(func(tx *sql.Tx) error {
		before, err := a.scheduleTx(tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM schedules WHERE id=?`, id); err != nil {
			return err
		}
		return a.audit(tx, EvDeleted, "schedule", id, before)
	})
}

func (a *App) scheduleTx(tx *sql.Tx, id int64) (*Schedule, error) {
	row := tx.QueryRow(`SELECT id, text, rule, suffix, created_at, counted_from, last_fired_at, last_reviewed_at FROM schedules WHERE id=?`, id)
	return scanSchedule(row)
}

func (a *App) Schedule(id int64) (*Schedule, error) {
	row := a.db.QueryRow(`SELECT id, text, rule, suffix, created_at, counted_from, last_fired_at, last_reviewed_at FROM schedules WHERE id=?`, id)
	s, err := scanSchedule(row)
	if err != nil {
		return nil, err
	}
	s.NextFire = nextFire(s.Rule, a.Today())
	s.RuleReadable = ruleReadable(s.Rule)
	return s, nil
}

type rowScanner interface{ Scan(dest ...any) error }

func scanSchedule(row rowScanner) (*Schedule, error) {
	s := &Schedule{}
	var created, reviewed string
	var fired sql.NullString
	if err := row.Scan(&s.ID, &s.Text, &s.Rule, &s.Suffix, &created, &s.CountedFrom, &fired, &reviewed); err != nil {
		return nil, err
	}
	s.CreatedAt = parseTS(created)
	s.LastReviewedAt = parseTS(reviewed)
	s.LastFiredAt = parseTSPtr(fired)
	return s, nil
}

// Schedules returns all schedules, ordered by when they next fire.
func (a *App) Schedules(nameFilter string) ([]*Schedule, error) {
	rows, err := a.db.Query(`SELECT id, text, rule, suffix, created_at, counted_from, last_fired_at, last_reviewed_at FROM schedules`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	today := a.Today()
	var out []*Schedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			return nil, err
		}
		if !matchName(nameFilter, s.Text) {
			continue
		}
		s.NextFire = nextFire(s.Rule, today)
		s.RuleReadable = ruleReadable(s.Rule)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sortSchedules(out)
	return out, nil
}

func sortSchedules(ss []*Schedule) {
	// by next fire, empty (impossible/expired) last
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && scheduleLess(ss[j], ss[j-1]); j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

func scheduleLess(a, b *Schedule) bool {
	if (a.NextFire == "") != (b.NextFire == "") {
		return a.NextFire != ""
	}
	if a.NextFire != b.NextFire {
		return a.NextFire < b.NextFire
	}
	return a.ID < b.ID
}

// applySuffix appends the suffix with YYYY/MM/DD replaced by the occurrence
// date; everything else is literal, including any leading space.
func applySuffix(text, suffix, occurrence string) string {
	if suffix == "" {
		return text
	}
	s := strings.NewReplacer("YYYY", occurrence[0:4], "MM", occurrence[5:7], "DD", occurrence[8:10]).Replace(suffix)
	return text + s
}

// occurrencesUpTo lists the days the rule fires in [countedFrom..today],
// excluding days at or before lastFired, oldest first, capped.
func occurrencesUpTo(s *Schedule, today string) ([]string, error) {
	from := s.CountedFrom
	if s.LastFiredAt != nil {
		// day granularity: an occurrence already fired never refires
		if d := s.LastFiredAt.Format(DateFormat); d >= from {
			day, _ := time.Parse(DateFormat, d)
			from = day.AddDate(0, 0, 1).Format(DateFormat)
		}
	}
	if from > today {
		return nil, nil
	}
	if ValidDate(s.Rule) {
		if s.Rule <= today && s.Rule >= s.CountedFrom {
			return []string{s.Rule}, nil
		}
		// a one-shot for a day before countedFrom can never fire
		return nil, nil
	}
	e, err := cron.Parse(s.Rule)
	if err != nil {
		return nil, err
	}
	day, _ := time.Parse(DateFormat, from)
	end, _ := time.Parse(DateFormat, today)
	var occ []string
	for !day.After(end) {
		if e.Matches(day) {
			occ = append(occ, day.Format(DateFormat))
			if len(occ) >= 400 { // hard cap; identical captures collapse anyway
				break
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return occ, nil
}

// dayAfter is the day following a "YYYY-MM-DD".
func dayAfter(day string) string {
	d, err := time.Parse(DateFormat, day)
	if err != nil {
		return day
	}
	return d.AddDate(0, 0, 1).Format(DateFormat)
}

// fireSchedules fires every missed occurrence of every schedule, oldest
// first, as part of day start. A schedule with nothing left to fire then
// deletes itself — the rule a one-shot always followed, said once for every
// kind of rule now that a rule can name its years (design.md, "Schedule").
func (a *App) fireSchedules(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id, text, rule, suffix, created_at, counted_from, last_fired_at, last_reviewed_at FROM schedules ORDER BY id`)
	if err != nil {
		return err
	}
	var schedules []*Schedule
	for rows.Next() {
		s, err := scanSchedule(rows)
		if err != nil {
			rows.Close()
			return err
		}
		schedules = append(schedules, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	today := a.Today()
	for _, s := range schedules {
		occ, err := occurrencesUpTo(s, today)
		if err != nil {
			continue // an unparseable rule (should not happen: validated on save)
		}
		fired := false
		for _, day := range occ {
			text := applySuffix(s.Text, s.Suffix, day)
			if _, _, err := a.captureTx(tx, text); err != nil {
				return err
			}
			fired = true
		}
		if fired {
			if _, err := tx.Exec(`UPDATE schedules SET last_fired_at=? WHERE id=?`, ts(a.now()), s.ID); err != nil {
				return err
			}
			if err := a.audit(tx, EvFired, "schedule", s.ID, s); err != nil {
				return err
			}
		}
		if nextFire(s.Rule, dayAfter(today)) == "" {
			// nothing left for it to do: it deletes itself, audited.
			if _, err := tx.Exec(`DELETE FROM schedules WHERE id=?`, s.ID); err != nil {
				return err
			}
			if err := a.audit(tx, EvScheduleExpired, "schedule", s.ID, s); err != nil {
				return err
			}
		}
	}
	return nil
}

// DayStart runs the lazy day-boundary work on the first use of the app on a
// new day (a read through the API counts as use): clear #today everywhere,
// fire missed schedule occurrences. Cheap no-op the rest of the day.
func (a *App) DayStart() error {
	today := a.Today()
	var last string
	err := a.db.QueryRow(`SELECT value FROM app_state WHERE key='last_day_opened'`).Scan(&last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if last == today {
		return nil
	}
	return a.tx(func(tx *sql.Tx) error {
		// re-check inside the transaction
		var l string
		err := tx.QueryRow(`SELECT value FROM app_state WHERE key='last_day_opened'`).Scan(&l)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if l == today {
			return nil
		}
		// clear #today from every item; deliberately not audited
		if _, err := tx.Exec(`DELETE FROM item_tags WHERE tag=?`, TodayTag); err != nil {
			return err
		}
		if err := a.fireSchedules(tx); err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO app_state (key,value) VALUES ('last_day_opened',?)
			ON CONFLICT(key) DO UPDATE SET value=excluded.value`, today)
		return err
	})
}
