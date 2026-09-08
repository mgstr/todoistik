// Package conf reads the settings file: the handful of choices that are not
// items and not screen state, and that a person (or an agent) sets by editing
// a file rather than by pressing anything.
//
// One `key = value` per line, `#` starts a comment anywhere on a line, blank
// lines are ignored.
// The format is deliberately the least there can be: no sections, no nesting,
// no types beyond what a key declares, so that changing a setting is one line
// found by grep and rewritten by anything.
package conf

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the whole of it. Every field is also a key in the file, and the
// zero value of this struct is not the default set — Defaults() is.
type Config struct {
	// ZenShowsTimer: the minutes since this action went on the screen are
	// shown beside it. The default only — ctrl-t flips it while it is up.
	ZenShowsTimer bool
	// ZenTimerFormat: how that number is written. "auto", or a pattern of
	// H/HH/M/MM with anything else taken literally.
	ZenTimerFormat string
	// ZenViews: the screens that open in zen mode — every panel off — without
	// being asked. Named by the same slugs the title bar shows a trail of.
	// Zen is still one keystroke away from being turned off again on any of
	// them, and the panels come back on the way out (see implementation.md,
	// "Zen mode").
	ZenViews []string
	// BackupDays: how many days of hourly database snapshots to keep. The
	// count of files is this times 24, and the oldest goes when a new one
	// takes it past that. Zero keeps none, which is how the file turns
	// backups off (see implementation.md, "Backups").
	BackupDays int
	// ReviewSomedayDays: how many days a someday/maybe item can go
	// unreviewed before the weekly review counts it as outstanding. Every
	// other item gets a week, and that week is not a setting; this one is,
	// because how long an idea may sit parked unasked-about is a choice
	// about the person's patience, not about the protocol (design.md,
	// "Weekly review").
	ReviewSomedayDays int
}

// TimerAuto is the format that is not a pattern: minutes up to an hour, then
// hours and minutes. It is the default because it is the only rendering that
// is short while the answer is short and still right after an hour, which no
// single pattern can be.
const TimerAuto = "auto"

// Defaults are what the app runs with when there is no file at all, and what
// any key left out of the file falls back to.
//
// Doing and processing open in zen mode, because both are screens you are in
// the middle of one item on: the rail is a list of other places you could be
// and the bar is a list of other things you could press, and neither question
// is the one being answered. The timer is off, because a clock on the wall is
// a thing you ask for. A someday/maybe item waits a month between reviews,
// because a parked idea does not change week to week (design.md, "Weekly
// review"). Every one of them is one line away from the opposite.
func Defaults() Config {
	return Config{
		ZenShowsTimer:     false,
		ZenTimerFormat:    TimerAuto,
		ZenViews:          []string{"doing", "processing"},
		BackupDays:        2,
		ReviewSomedayDays: 30,
	}
}

// MaxBackupDays is the ceiling on backup.days. A year of hourly snapshots is
// 8760 files beside the database, which is past the point where this is a
// backup scheme rather than a hoard — at that scale the answer is something
// that copies the directory off this machine, not a bigger number here.
const MaxBackupDays = 365

// MaxReviewDays is the ceiling on review.someday_days. An idea that can go
// more than a year without being asked about is not parked, it is buried —
// past that the number is not a cadence but a way of writing "never", and
// "never reviewed" is the failure the whole review exists to prevent.
const MaxReviewDays = 365

// bools maps a key in the file to the field it sets. Adding a setting is
// adding a line here (or to strs below); nothing else in this file knows any
// key's name.
func (c *Config) bools() map[string]*bool {
	return map[string]*bool{
		"zen.show_timer": &c.ZenShowsTimer,
	}
}

// ints are the settings that take a number. Like the strings below they bring
// their own check, since "backup.days = two" is a typo that would otherwise
// read as zero and quietly keep nothing.
func (c *Config) ints() map[string]*int {
	return map[string]*int{
		"backup.days":         &c.BackupDays,
		"review.someday_days": &c.ReviewSomedayDays,
	}
}

// intChecks is each number key's range, with the ends explained in the
// message — because the two keys disagree about zero: keeping no backups is
// an answer, a review period of no days is not.
var intChecks = map[string]func(int) error{
	"backup.days": func(d int) error {
		if d < 0 || d > MaxBackupDays {
			return fmt.Errorf("it is %d, and the range is 0 (keep none) to %d", d, MaxBackupDays)
		}
		return nil
	},
	"review.someday_days": func(d int) error {
		if d < 1 || d > MaxReviewDays {
			return fmt.Errorf("it is %d, and the range is 1 (back on every review) to %d", d, MaxReviewDays)
		}
		return nil
	},
}

// strs are the settings that take words rather than true/false. Each brings
// its own check, because a string setting with no check is a typo that reaches
// the screen — which for a value read once at startup means it stays there.
func (c *Config) strs() map[string]*string {
	return map[string]*string{
		"zen.timer_format": &c.ZenTimerFormat,
	}
}

// lists are the settings that take several names, written with commas between
// them. An empty value is a legal answer and means none — `zen.views =` is how
// a file says that no screen opens in zen mode.
func (c *Config) lists() map[string]*[]string {
	return map[string]*[]string{
		"zen.views": &c.ZenViews,
	}
}

var checks = map[string]func(string) error{
	"zen.timer_format": checkTimerFormat,
	"zen.views":        checkNames,
}

// checkNames: a comma-separated list of screen slugs. Only the shape is
// checked here — whether a name is a screen this app has is checked where the
// screens are known (see internal/web, ScreenNames), because a settings file
// that names a screen which does not exist has to fail at startup like any
// other typo, and conf is not the place that knows the list.
func checkNames(v string) error {
	for _, name := range Split(v) {
		for _, r := range name {
			if r >= 'a' && r <= 'z' || r == '-' {
				continue
			}
			return fmt.Errorf("%q is not a screen name: they are lower-case words", name)
		}
	}
	return nil
}

// Split reads a list value: names separated by commas, spaces around them
// ignored, empty entries dropped. Exported because the same string is read
// back by whoever validates the names.
func Split(v string) []string {
	var out []string
	for _, part := range strings.Split(v, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// checkTimerFormat: "auto", or a pattern. In a pattern, uppercase `H`/`HH` is
// the hours and `M`/`MM` the minutes — within the hour if the pattern also
// asks for hours, and the whole elapsed time if it does not. Everything else
// is literal, so `H:MM`, `HH:MM` and `M` all work, and so does `H h MM`.
//
// Every other capital is refused. A capital in a pattern reads as a field, and
// silently printing `HH:NN` because `N` is not one would be exactly the kind
// of wrong a settings file read once must not be.
func checkTimerFormat(v string) error {
	if strings.EqualFold(v, TimerAuto) {
		return nil
	}
	if len(v) > 24 {
		return fmt.Errorf("a format is at most 24 characters, got %d", len(v))
	}
	fields := 0
	for _, r := range v {
		switch {
		case r == 'H' || r == 'M':
			fields++
		case r >= 'A' && r <= 'Z':
			return fmt.Errorf("%q is not a field: the fields are H, HH, M and MM, and %q is capitalised like one", string(r), string(r))
		}
	}
	if fields == 0 {
		return fmt.Errorf("a format needs an H or an M in it, or the word %q", TimerAuto)
	}
	return nil
}

// Load reads path over the defaults. A missing file is not an error — running
// with no settings file is the ordinary case, and the defaults are the app.
//
// Anything else is: an unknown key, a line that is not `key = value`, or a
// value that is not `true` or `false` fails startup naming the file, the line
// number and what was wrong. A settings file is read once (see the package
// comment), so a typo that was quietly ignored would be a setting that looks
// set for as long as the process lives — the one failure this format could
// have, and the reason it fails loudly instead.
func Load(path string) (Config, error) {
	c := Defaults()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	defer f.Close()

	bools, ints, strs, lists := c.bools(), c.ints(), c.strs(), c.lists()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		// a comment runs to the end of the line, and there is nothing a value
		// could be that contains a "#" — so the whole line is cut at the first
		// one before anything else looks at it
		line, _, _ := strings.Cut(sc.Text(), "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return c, fmt.Errorf("%s:%d: not a key = value line: %q", path, n, line)
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if field, ok := lists[key]; ok {
			if err := checks[key](val); err != nil {
				return c, fmt.Errorf("%s:%d: %s: %v", path, n, key, err)
			}
			*field = Split(val)
			continue
		}
		if field, ok := ints[key]; ok {
			days, err := strconv.Atoi(val)
			if err != nil {
				return c, fmt.Errorf("%s:%d: %s wants a whole number of days, got %q", path, n, key, val)
			}
			if err := intChecks[key](days); err != nil {
				return c, fmt.Errorf("%s:%d: %s: %v", path, n, key, err)
			}
			*field = days
			continue
		}
		if field, ok := strs[key]; ok {
			if err := checks[key](val); err != nil {
				return c, fmt.Errorf("%s:%d: %s: %v", path, n, key, err)
			}
			*field = val
			continue
		}
		field, ok := bools[key]
		if !ok {
			return c, fmt.Errorf("%s:%d: unknown setting %q (known: %s)", path, n, key, strings.Join(Keys(), ", "))
		}
		switch strings.ToLower(val) {
		case "true":
			*field = true
		case "false":
			*field = false
		default:
			return c, fmt.Errorf("%s:%d: %s wants true or false, got %q", path, n, key, val)
		}
	}
	return c, sc.Err()
}

// Keys lists every setting name, sorted, for the message a wrong one gets.
func Keys() []string {
	var c Config
	out := make([]string, 0, len(c.bools())+len(c.ints())+len(c.strs())+len(c.lists()))
	for k := range c.bools() {
		out = append(out, k)
	}
	for k := range c.ints() {
		out = append(out, k)
	}
	for k := range c.strs() {
		out = append(out, k)
	}
	for k := range c.lists() {
		out = append(out, k)
	}
	// small and fixed, so an insertion sort is the whole of it
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
