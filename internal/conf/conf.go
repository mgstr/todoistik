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
	// KeysAnyLayout: whether a shortcut is a place on the keyboard rather
	// than a letter, so that the keys still work with the keyboard in a
	// Cyrillic layout (implementation.md, "Which key is which"). Off reads
	// the character and nothing else, which is what the app did before and
	// means no key at all answers while the layout is Russian.
	KeysAnyLayout bool
	// KeysLayoutMarker: whether the key bar says when the keyboard is in a
	// Cyrillic layout. It is about the boxes rather than the keys — what
	// gets typed into a capture is whatever the layout types (design.md,
	// "Panels"). Independent of KeysAnyLayout: either is worth having
	// without the other.
	KeysLayoutMarker bool
	// LinksReach: how far ctrl-o sees when it follows a link in the item
	// under the cursor. ReachAny is every link the item's text holds,
	// ReachShown only the ones the screen has actually drawn (design.md,
	// "Following a link").
	LinksReach string
	// KeysMode: whether a button's declared letter is pressed bare, with
	// ctrl, or bare except for the three that have to fire while a box is
	// being typed in. A control declares its letter and never its modifier,
	// so this is the one place that decides (keys.md, "The three modes").
	// It exists because the question is about hands rather than about the
	// code, and the only honest way to settle it is to work in each.
	KeysMode string
	// KeysBarStyle: how a key bar entry that presses something is painted,
	// now that the bar is where the buttons are and not merely a list of
	// what they answer to (implementation.md, "The key bar is the buttons").
	// The bar holds two kinds of entry — one that presses a control and one
	// that steers — and this decides how loudly it tells them apart, from
	// not at all to the button's own styling. Like KeysMode it is a question
	// about a week of use rather than about the code.
	KeysBarStyle string
	// AnimDone, AnimDelete, AnimBack: what each of the three keys that leave a
	// screen looks like on the way out (design.md, "A moment that shows
	// itself"). Three keys and not one, because the three moments make three
	// different claims — a thing finished, a thing thrown away, a place left —
	// and the motion that says one of them says the other two wrong. They do
	// not take the same set of answers for the same reason; see animTakes.
	AnimDone   string
	AnimDelete string
	AnimBack   string
	// AnimMS: how long any of them runs. One number for all three, because it
	// is not a fact about the moment, it is a fact about the hand. Zero is
	// "none" on all three at once, which is how the file turns the whole of it
	// off in one line.
	AnimMS int
	// Theme: which palette the screen is painted in before anything has been
	// chosen by hand. ThemeAuto follows the system, which is what the app did
	// when the scheme was not a question at all. It is the default rather than
	// the only answer because the light one is right at a desk in daylight and
	// the dark one at night, and the machine already knows which of those it
	// is — see design.md, "Theme".
	//
	// The file is the *default*, not the last word: the Settings screen
	// remembers what it was told, the way the panels remember what ctrl-v was
	// told, and a remembered answer wins (implementation.md, "Theme").
	Theme string
	// DupMatch: how a capture being processed is compared against the actions
	// and projects that already exist, so that writing down a thing you have
	// written down before is noticed at the one moment it can be (design.md,
	// "Matches while processing"). MatchNone asks nothing and shows nothing.
	//
	// It is a setting rather than a rule because the two answers are wrong in
	// different directions and only use settles which is worse here: words
	// miss a rewording, characters miss the short capture against the long
	// title. Both are one line away.
	DupMatch string
	// DupOverlap: with MatchOverlap, the share of the shorter title's words
	// that has to appear in the other one, as a percentage. 100 is "every
	// word of it"; the default is the lowest number that still says that of a
	// title of three words or fewer, and lets a four-word one differ by an
	// article — which is the shape a reworded repeat actually has.
	DupOverlap int
	// DupSimilar: with MatchSimilar, the share of the two titles' character
	// pairs that has to be shared, as a percentage. A separate key from
	// DupOverlap and not one number used by both: a share of words and a
	// share of characters are not the same quantity, and one key would move
	// the threshold you are not tuning every time you tune the one you are.
	DupSimilar int
}

// TimerAuto is the format that is not a pattern: minutes up to an hour, then
// hours and minutes. It is the default because it is the only rendering that
// is short while the answer is short and still right after an hour, which no
// single pattern can be.
const TimerAuto = "auto"

// The two answers links.reach takes. Any is the default because the key exists
// for the screens that show least of an item — a next action is a title and
// some badges, the doing screen is a title alone — and a reach that stopped at
// what was drawn would be missing on exactly those. Shown is the other
// defensible reading: that a link must be lookable-at before it is followed.
const (
	ReachAny   = "any"
	ReachShown = "shown"
)

// The three answers keys.mode takes. Hybrid is the default because it is what
// the app already did: bare letters everywhere, with ctrl on save, create and
// add — the three that are pressed with the hands still in a form. The other
// two are the whole answer in one direction or the other, and they are here to
// be worked in rather than reasoned about (keys.md, "The three modes").
const (
	ModeCommand  = "command"
	ModeModifier = "modifier"
	ModeHybrid   = "hybrid"
)

// The five answers keys.bar_style takes, quietest first. They differ only in
// paint: every one of them makes the same entries pressable, and none of them
// may paint an entry that presses nothing — that rule is in the bar itself,
// not in the flag (implementation.md, "The key bar is the buttons").
//
// Chip is the default. Once the buttons came off the forms the bar started
// holding two kinds of entry, a control and a way to steer, and it is the one
// answer that tells them apart with nothing hovered — which is the new thing
// the bar has to say. The other four are here because "how loud should the
// controls be" is a question about a week of use: Plain says nothing at all,
// Hover waits to be asked, Keycap draws the key rather than the control, and
// Button is the row that used to be on the form, moved down unchanged.
const (
	BarPlain  = "plain"
	BarHover  = "hover"
	BarChip   = "chip"
	BarKeycap = "keycap"
	BarButton = "button"
)

// The eight answers an anim key takes. They divide into three families, and
// the family is the whole of the difference between them (implementation.md,
// "A moment that shows itself"):
//
//   - leaving — Fade, Strike, Collapse, Sweep — plays on the item that was
//     acted on, before the request goes. The item is still on the screen while
//     it runs, which is what makes the press unmistakable, and the key is deaf
//     for exactly as long, which is what makes the second press impossible
//   - arriving — Rise, Flash — plays on the page that comes back. It costs
//     nothing on the clock, because that page is already there, and it has to
//     be handed the moment it is about the way the cursor already is
//   - over — Stamp — a wordless mark over the whole screen, and the only one
//     of the eight that says *which* of the three things happened
const (
	AnimNone     = "none"
	AnimFade     = "fade"
	AnimStrike   = "strike"
	AnimCollapse = "collapse"
	AnimSweep    = "sweep"
	AnimRise     = "rise"
	AnimFlash    = "flash"
	AnimStamp    = "stamp"
)

// The three answers theme takes. Auto is the default because the system
// already carries the answer — a laptop that turns its own screen dark in the
// evening is saying which of the two this is, and an app that ignored it would
// be the one bright window at midnight. The other two are here because the
// system's answer is about the machine and this one is about the room: a
// screen read in sunlight wants the light palette whatever the clock says.
const (
	ThemeAuto  = "auto"
	ThemeLight = "light"
	ThemeDark  = "dark"
)

// Themes is the three in the order they are offered and cycled through, which
// is the order they read in: the answer that is not a choice first, then the
// two that are. Exported because the screen that offers them must not keep a
// second copy of the list — a fourth answer added here has to reach the screen
// without anything else being edited (implementation.md, "Theme").
var Themes = []string{ThemeAuto, ThemeLight, ThemeDark}

// NextTheme is what one press of the theme key gives: the answer after this
// one, wrapping. An answer the app does not know — nothing can write one
// today, but a stored value outlives the code that wrote it — starts the cycle
// again rather than sticking.
func NextTheme(cur string) string {
	for i, t := range Themes {
		if t == cur {
			return Themes[(i+1)%len(Themes)]
		}
	}
	return Themes[0]
}

// The three answers duplicates.match takes. Overlap is the default because it
// is the one that catches the commonest real repeat — the short capture
// against the longer title it was written as before, "Call dentist" against
// "Call dentist about the crown" — which character similarity scores as a
// stranger. Similar is here because it catches what words cannot: a typo and
// a rewording. None is here because a person who does not want to be asked
// should not have to be, and it is the answer that costs nothing at all.
const (
	MatchNone    = "none"
	MatchOverlap = "overlap"
	MatchSimilar = "similar"
)

// animTakes is what each of the three keys is allowed to be set to, and the
// three lists are deliberately not the same. A settings file is read once, so
// a value that means nothing on the key it is written against would be a
// setting that looks set for as long as the process lives — the failure every
// string setting in this file is checked against.
//
// Strike is not offered on a delete: a line drawn through a title is the mark
// for finished, and putting it on a thing being thrown away says the wrong
// word. Collapse and Flash are not offered on back, because nothing is being
// removed and nothing new is being acted on — the screen is simply somewhere
// else, and an effect that says "this one, here" would be pointing at an item
// that had nothing done to it.
var animTakes = map[string][]string{
	"anim.done":   {AnimNone, AnimFade, AnimStrike, AnimCollapse, AnimSweep, AnimRise, AnimFlash, AnimStamp},
	"anim.delete": {AnimNone, AnimFade, AnimCollapse, AnimSweep, AnimRise, AnimFlash, AnimStamp},
	"anim.back":   {AnimNone, AnimFade, AnimSweep, AnimRise, AnimStamp},
}

// Defaults are what the app runs with when there is no file at all, and what
// any key left out of the file falls back to.
//
// Doing and processing open in zen mode, because both are screens you are in
// the middle of one item on: the rail is a list of other places you could be
// and the bar is a list of other things you could press, and neither question
// is the one being answered. The timer is off, because a clock on the wall is
// a thing you ask for. A someday/maybe item waits a month between reviews,
// because a parked idea does not change week to week (design.md, "Weekly
// review"). Both keyboard settings are on, because the app is used from the
// keyboard in two languages and a key that stops working when the layout
// changes is a bug rather than a mode — they are settings at all only so that
// either can be taken back out of the way without an edit to the code. The key
// bar paints its controls as chips, because it is the quietest answer that
// still says which of its entries are controls at all.
//
// The three moments are on, and each wears the effect that says its own word
// rather than a general "something happened": a line drawn through a title is
// what finished looks like, a thing folding shut while the list closes over it
// is what thrown away looks like, and a screen sliding aside is what leaving
// looks like. All three are leaving effects, which is the point — they are the
// only family that makes the second press impossible without a deaf window
// having to be built for them by hand (design.md, "A moment that shows
// itself"). 160ms because it is under the threshold where a press stops
// feeling instant and over the one where an effect is not seen at all.
//
// Every one of these is one line away from the opposite.
func Defaults() Config {
	return Config{
		ZenShowsTimer:     false,
		ZenTimerFormat:    TimerAuto,
		ZenViews:          []string{"doing", "processing"},
		BackupDays:        2,
		ReviewSomedayDays: 30,
		KeysAnyLayout:     true,
		KeysLayoutMarker:  true,
		LinksReach:        ReachAny,
		KeysMode:          ModeHybrid,
		KeysBarStyle:      BarChip,
		AnimDone:          AnimStrike,
		AnimDelete:        AnimCollapse,
		AnimBack:          AnimSweep,
		AnimMS:            160,
		Theme:             ThemeAuto,
		DupMatch:          MatchOverlap,
		DupOverlap:        70,
		DupSimilar:        75,
	}
}

// MaxBackupDays is the ceiling on backup.days. A year of hourly snapshots is
// 8760 files beside the database, which is past the point where this is a
// backup scheme rather than a hoard — at that scale the answer is something
// that copies the directory off this machine, not a bigger number here.
const MaxBackupDays = 365

// MaxAnimMS is the ceiling on anim.ms. Past about a third of a second an
// effect stops reading as the answer to a press and starts reading as the app
// being slow, which is the one thing design.md's principles say it may not be
// — 600 is twice that, and is already the far end of "I want to watch this
// happen" rather than a number anybody works at.
const MaxAnimMS = 600

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
		"zen.show_timer":     &c.ZenShowsTimer,
		"keys.any_layout":    &c.KeysAnyLayout,
		"keys.layout_marker": &c.KeysLayoutMarker,
	}
}

// ints are the settings that take a number. Like the strings below they bring
// their own check, since "backup.days = two" is a typo that would otherwise
// read as zero and quietly keep nothing.
func (c *Config) ints() map[string]*int {
	return map[string]*int{
		"backup.days":         &c.BackupDays,
		"review.someday_days": &c.ReviewSomedayDays,
		"anim.ms":             &c.AnimMS,
		"duplicates.overlap":  &c.DupOverlap,
		"duplicates.similar":  &c.DupSimilar,
	}
}

// intChecks is each number key's range, with the ends explained in the
// message — because the keys disagree about zero: keeping no backups is an
// answer and so is no motion at all, while a review period of no days is not.
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
	"anim.ms": func(ms int) error {
		if ms < 0 || ms > MaxAnimMS {
			return fmt.Errorf("it is %d, and the range is 0 (no motion at all) to %d", ms, MaxAnimMS)
		}
		return nil
	},
	// Both thresholds are shares, so both run 1 to 100 and neither takes
	// zero: a share of nothing would make every item in the app a match for
	// every capture, which is not "show me more", it is a list that says
	// nothing. Turning the question off is `duplicates.match = none`, which
	// says so in the key that is about whether it is asked at all.
	"duplicates.overlap": pct("duplicates.overlap", "every word of the shorter title"),
	"duplicates.similar": pct("duplicates.similar", "the same characters end to end"),
}

// pct is the check both thresholds take: a percentage, 1 to 100, with the top
// of the range named in words — because what 100 means differs between them
// and a message reading "the range is 1 to 100" would not say what asking for
// 100 would get you.
func pct(key, whole string) func(int) error {
	return func(n int) error {
		if n < 1 || n > 100 {
			return fmt.Errorf("it is %d, and the range is 1 to 100 (%s)", n, whole)
		}
		return nil
	}
}

// strs are the settings that take words rather than true/false. Each brings
// its own check, because a string setting with no check is a typo that reaches
// the screen — which for a value read once at startup means it stays there.
func (c *Config) strs() map[string]*string {
	return map[string]*string{
		"zen.timer_format": &c.ZenTimerFormat,
		"links.reach":      &c.LinksReach,
		"keys.mode":        &c.KeysMode,
		"keys.bar_style":   &c.KeysBarStyle,
		"anim.done":        &c.AnimDone,
		"anim.delete":      &c.AnimDelete,
		"anim.back":        &c.AnimBack,
		"theme":            &c.Theme,
		"duplicates.match": &c.DupMatch,
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
	"links.reach":      checkReach,
	"keys.mode":        checkKeysMode,
	"keys.bar_style":   checkBarStyle,
	"anim.done":        checkAnim("anim.done"),
	"anim.delete":      checkAnim("anim.delete"),
	"anim.back":        checkAnim("anim.back"),
	"theme":            checkTheme,
	"duplicates.match": checkMatch,
}

// checkAnim: one of the effects *this* key takes, which is not the same list
// for all three — so the message says the key's own list rather than all eight
// (see animTakes for why the lists differ).
func checkAnim(key string) func(string) error {
	return func(v string) error {
		for _, ok := range animTakes[key] {
			if v == ok {
				return nil
			}
		}
		return fmt.Errorf("%q is not an effect %s takes: they are %s", v, key, strings.Join(animTakes[key], ", "))
	}
}

// checkMatch: one of three words, for the reason every other string setting
// here is checked — a fourth would read as a fourth way of comparing and get
// none.
func checkMatch(v string) error {
	switch v {
	case MatchNone, MatchOverlap, MatchSimilar:
		return nil
	}
	return fmt.Errorf("%q is not a way of matching: it is %q (nothing is compared), %q (a share of the words) or %q (a share of the characters)",
		v, MatchNone, MatchOverlap, MatchSimilar)
}

// checkTheme: one of three words, for the reason every other string setting
// here is checked — a fourth would read as a fourth palette and get none.
func checkTheme(v string) error {
	for _, t := range Themes {
		if v == t {
			return nil
		}
	}
	return fmt.Errorf("%q is not a theme: it is %q (whichever the system is set to), %q or %q", v, ThemeAuto, ThemeLight, ThemeDark)
}

// checkBarStyle: one of five words, for the reason every other string setting
// here is checked — a sixth would read as a sixth paint and get none.
func checkBarStyle(v string) error {
	switch v {
	case BarPlain, BarHover, BarChip, BarKeycap, BarButton:
		return nil
	}
	return fmt.Errorf("%q is not a bar style: it is %q (no mark), %q (a pill under the pointer), %q (a bordered pill at rest), %q (the letter drawn as a key) or %q (the form button's own styling)",
		v, BarPlain, BarHover, BarChip, BarKeycap, BarButton)
}

// checkKeysMode: one of three words. A fourth would read as a fourth
// behaviour and get none — the failure every string setting here is checked
// against.
func checkKeysMode(v string) error {
	switch v {
	case ModeCommand, ModeModifier, ModeHybrid:
		return nil
	}
	return fmt.Errorf("%q is not a key mode: it is %q (bare letters), %q (ctrl and a letter) or %q (bare, with ctrl on save, create and add)",
		v, ModeCommand, ModeModifier, ModeHybrid)
}

// checkReach: one of two words and nothing else. A third word here would read
// as a third behaviour and get none — the failure every string setting in this
// file is checked against.
func checkReach(v string) error {
	if v == ReachAny || v == ReachShown {
		return nil
	}
	return fmt.Errorf("%q is not a reach: it is %q (every link the item holds) or %q (only the ones on the screen)", v, ReachAny, ReachShown)
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
