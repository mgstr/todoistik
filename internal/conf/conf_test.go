package conf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "todoistik.conf")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileIsTheDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nothing-here.conf"))
	if err != nil {
		t.Fatalf("missing file: %v", err)
	}
	if !reflect.DeepEqual(c, Defaults()) {
		t.Errorf("missing file gave %+v, want the defaults %+v", c, Defaults())
	}
}

func TestReadsOverTheDefaults(t *testing.T) {
	c, err := Load(write(t, `
# the timer is up from the first second
zen.show_timer = true

  zen.views = doing , someday   # a comment can trail a value too
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !c.ZenShowsTimer {
		t.Error("zen.show_timer = true was not read")
	}
	if !reflect.DeepEqual(c.ZenViews, []string{"doing", "someday"}) {
		t.Errorf("zen.views = %q, want the two names with the spaces off", c.ZenViews)
	}
}

func TestOmittedKeyKeepsItsDefault(t *testing.T) {
	c, err := Load(write(t, "zen.show_timer = true\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(c.ZenViews, Defaults().ZenViews) {
		t.Error("a key left out of the file did not keep its default")
	}
}

// An empty list is an answer and not an omission: it is how the file says that
// no screen opens in zen mode, which the defaults cannot say.
func TestEmptyListIsNone(t *testing.T) {
	c, err := Load(write(t, "zen.views =\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(c.ZenViews) != 0 {
		t.Errorf("zen.views = %q, want none", c.ZenViews)
	}
}

func TestTimerFormat(t *testing.T) {
	c, err := Load(write(t, "zen.timer_format = H:MM\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ZenTimerFormat != "H:MM" {
		t.Errorf("format = %q, want %q", c.ZenTimerFormat, "H:MM")
	}
	if Defaults().ZenTimerFormat != TimerAuto {
		t.Errorf("default format = %q, want %q", Defaults().ZenTimerFormat, TimerAuto)
	}
	for _, ok := range []string{"auto", "AUTO", "M", "MM", "HH:MM", "H h MM"} {
		if err := checkTimerFormat(ok); err != nil {
			t.Errorf("%q was refused: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "hh:mm", "HH:NN", strings.Repeat("M", 25)} {
		if err := checkTimerFormat(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}

func TestBackupDays(t *testing.T) {
	if got := Defaults().BackupDays; got != 2 {
		t.Errorf("default backup.days = %d, want 2 — two days of hourly snapshots", got)
	}
	c, err := Load(write(t, "backup.days = 5\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.BackupDays != 5 {
		t.Errorf("backup.days = %d, want 5", c.BackupDays)
	}
	// zero is an answer: it is how the file says "keep none"
	c, err = Load(write(t, "backup.days = 0\n"))
	if err != nil {
		t.Fatalf("zero was refused: %v", err)
	}
	if c.BackupDays != 0 {
		t.Errorf("backup.days = %d, want 0", c.BackupDays)
	}
}

func TestReviewSomedayDays(t *testing.T) {
	if got := Defaults().ReviewSomedayDays; got != 30 {
		t.Errorf("default review.someday_days = %d, want 30 — a month between someday reviews", got)
	}
	c, err := Load(write(t, "review.someday_days = 90\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.ReviewSomedayDays != 90 {
		t.Errorf("review.someday_days = %d, want 90", c.ReviewSomedayDays)
	}
	// one is the floor: it means back on every review, and zero would be a
	// period of no days, which means nothing
	c, err = Load(write(t, "review.someday_days = 1\n"))
	if err != nil {
		t.Fatalf("one was refused: %v", err)
	}
	if c.ReviewSomedayDays != 1 {
		t.Errorf("review.someday_days = %d, want 1", c.ReviewSomedayDays)
	}
}

// The three ways a settings file can be wrong all have to stop startup, since
// the file is read once and a quietly-ignored line looks set forever.
func TestBadLinesAreRefused(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"unknown key", "zen.show_everything = true\n", "unknown setting"},
		{"no equals", "zen.show_timer true\n", "not a key = value line"},
		{"not a bool", "zen.show_timer = sometimes\n", "wants true or false"},
		{"bad format", "zen.timer_format = HH:NN\n", "is not a field"},
		{"bad screen name", "zen.views = Doing\n", "is not a screen name"},
		{"days not a number", "backup.days = two\n", "whole number of days"},
		{"days negative", "backup.days = -1\n", "the range is 0"},
		{"days too many", "backup.days = 400\n", "the range is 0"},
		{"review days zero", "review.someday_days = 0\n", "the range is 1"},
		{"review days too many", "review.someday_days = 400\n", "the range is 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(write(t, tc.body))
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not say %q", err, tc.want)
			}
			if !strings.Contains(err.Error(), ":1:") {
				t.Errorf("error %q does not point at the line", err)
			}
		})
	}
}

func TestLinksReach(t *testing.T) {
	if got := Defaults().LinksReach; got != ReachAny {
		t.Errorf("default links.reach = %q, want %q — the key exists for the screens that draw no link", got, ReachAny)
	}
	c, err := Load(write(t, "links.reach = shown\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.LinksReach != ReachShown {
		t.Errorf("links.reach = %q, want %q", c.LinksReach, ReachShown)
	}
	// a third word would read as a third behaviour and get none
	for _, bad := range []string{"", "all", "Any", "any, shown"} {
		if _, err := Load(write(t, "links.reach = "+bad+"\n")); err == nil {
			t.Errorf("links.reach = %q was accepted", bad)
		}
	}
}

// Both keyboard settings are on out of the box: a key that stops working when
// the layout changes is a bug rather than a mode, and the file exists to take
// either behaviour back out of the way rather than to switch it on.
func TestKeyboardSettings(t *testing.T) {
	d := Defaults()
	if !d.KeysAnyLayout {
		t.Error("default keys.any_layout = false, want true — the keys must work in either layout unasked")
	}
	if !d.KeysLayoutMarker {
		t.Error("default keys.layout_marker = false, want true")
	}
	c, err := Load(write(t, "keys.any_layout = false\nkeys.layout_marker = false\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.KeysAnyLayout || c.KeysLayoutMarker {
		t.Errorf("both were turned off and got %+v", c)
	}
	// and each is independent of the other, since either is worth having alone
	c, err = Load(write(t, "keys.layout_marker = false\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !c.KeysAnyLayout {
		t.Error("turning the marker off also turned the keys off")
	}
	for _, bad := range []string{"", "yes", "1", "on"} {
		if _, err := Load(write(t, "keys.any_layout = "+bad+"\n")); err == nil {
			t.Errorf("keys.any_layout = %q was accepted", bad)
		}
	}
}

// The bar paints its controls as chips unless told otherwise: once the buttons
// came off the forms the bar began holding two kinds of entry, and chip is the
// quietest of the five that still tells them apart with nothing hovered.
func TestKeysBarStyle(t *testing.T) {
	if got := Defaults().KeysBarStyle; got != BarChip {
		t.Errorf("default keys.bar_style = %q, want %q \u2014 the bar has to say which of its entries are controls", got, BarChip)
	}
	for _, good := range []string{BarPlain, BarHover, BarChip, BarKeycap, BarButton} {
		c, err := Load(write(t, "keys.bar_style = "+good+"\n"))
		if err != nil {
			t.Fatalf("keys.bar_style = %q: %v", good, err)
		}
		if c.KeysBarStyle != good {
			t.Errorf("keys.bar_style = %q, want %q", c.KeysBarStyle, good)
		}
	}
	// a sixth word would read as a sixth paint and get none
	for _, bad := range []string{"", "Chip", "segmented", "chip, hover", "none"} {
		if _, err := Load(write(t, "keys.bar_style = "+bad+"\n")); err == nil {
			t.Errorf("keys.bar_style = %q was accepted", bad)
		}
	}
}

// The three moments are on by default and each wears the effect that says its
// own word — and the three keys do not take the same eight words, which is the
// part the checker has to carry: a value that means nothing on the key it is
// written against would be a setting that looks set for the life of the
// process (see animTakes).
func TestAnim(t *testing.T) {
	d := Defaults()
	if d.AnimDone != AnimStrike || d.AnimDelete != AnimCollapse || d.AnimBack != AnimSweep {
		t.Errorf("defaults are %q/%q/%q, want %q/%q/%q — each moment says its own word",
			d.AnimDone, d.AnimDelete, d.AnimBack, AnimStrike, AnimCollapse, AnimSweep)
	}
	if d.AnimMS != 160 {
		t.Errorf("default anim.ms = %d, want 160", d.AnimMS)
	}

	for key, legal := range animTakes {
		for _, good := range legal {
			if _, err := Load(write(t, key+" = "+good+"\n")); err != nil {
				t.Errorf("%s = %q: %v", key, good, err)
			}
		}
	}

	// the differences between the three lists are the whole reason there are
	// three: a line through a title means finished, and leaving a screen
	// removes nothing and acts on nothing
	for _, bad := range []struct{ key, val string }{
		{"anim.delete", AnimStrike},
		{"anim.back", AnimStrike},
		{"anim.back", AnimCollapse},
		{"anim.back", AnimFlash},
		{"anim.done", ""},
		{"anim.done", "Strike"},
		{"anim.done", "slide"},
	} {
		if _, err := Load(write(t, bad.key+" = "+bad.val+"\n")); err == nil {
			t.Errorf("%s = %q was accepted", bad.key, bad.val)
		}
	}

	// zero is an answer here the way it is for backups: it is how the file
	// turns the whole of it off in one line
	c, err := Load(write(t, "anim.ms = 0\n"))
	if err != nil || c.AnimMS != 0 {
		t.Errorf("anim.ms = 0: got %d, %v — no motion at all is a legal answer", c.AnimMS, err)
	}
	for _, bad := range []string{"-1", "601", "fast", ""} {
		if _, err := Load(write(t, "anim.ms = "+bad+"\n")); err == nil {
			t.Errorf("anim.ms = %q was accepted", bad)
		}
	}
}

// The theme is the default the app opens with, not the last word — what the
// Settings screen was told beats it (implementation.md, "Theme"). What this
// file has to carry is the three words and the cycle through them, since the
// screen offers them in this order and presses them in this order.
func TestTheme(t *testing.T) {
	if got := Defaults().Theme; got != ThemeAuto {
		t.Errorf("default theme = %q, want %q — the system already knows whether it is night", got, ThemeAuto)
	}
	for _, good := range Themes {
		c, err := Load(write(t, "theme = "+good+"\n"))
		if err != nil {
			t.Fatalf("theme = %q: %v", good, err)
		}
		if c.Theme != good {
			t.Errorf("theme = %q, want %q", c.Theme, good)
		}
	}
	// a fourth word would read as a fourth palette and get none
	for _, bad := range []string{"", "Dark", "solarized", "light, dark", "system"} {
		if _, err := Load(write(t, "theme = "+bad+"\n")); err == nil {
			t.Errorf("theme = %q was accepted", bad)
		}
	}

	// the cycle is a ring, and a word the app does not know starts it again
	// rather than sticking on itself
	for i, cur := range Themes {
		if got, want := NextTheme(cur), Themes[(i+1)%len(Themes)]; got != want {
			t.Errorf("after %q comes %q, want %q", cur, got, want)
		}
	}
	if got := NextTheme("solarized"); got != Themes[0] {
		t.Errorf("after a theme nobody knows comes %q, want %q", got, Themes[0])
	}
}

func TestDuplicates(t *testing.T) {
	d := Defaults()
	if d.DupMatch != MatchOverlap || d.DupOverlap != 70 || d.DupSimilar != 75 {
		t.Errorf("defaults are %q %d %d, want overlap 70 75 — words by default, characters on request",
			d.DupMatch, d.DupOverlap, d.DupSimilar)
	}
	c, err := Load(write(t, "duplicates.match = similar\nduplicates.similar = 80\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.DupMatch != MatchSimilar || c.DupSimilar != 80 {
		t.Errorf("read %q %d, want similar 80", c.DupMatch, c.DupSimilar)
	}
	if c.DupOverlap != d.DupOverlap {
		t.Errorf("the threshold not named kept %d, want its default %d", c.DupOverlap, d.DupOverlap)
	}
	// a fourth word would read as a fourth way of comparing and get none
	for _, bad := range []string{"", "fuzzy", "Overlap", "words"} {
		if _, err := Load(write(t, "duplicates.match = "+bad+"\n")); err == nil {
			t.Errorf("duplicates.match = %q was accepted", bad)
		}
	}
	// both thresholds are shares, so both refuse zero: a share of nothing
	// would make everything in the app a match for every capture, which is not
	// more answers but no answer
	for _, bad := range []string{"0", "-1", "101", "half"} {
		for _, key := range []string{"duplicates.overlap", "duplicates.similar"} {
			if _, err := Load(write(t, key+" = "+bad+"\n")); err == nil {
				t.Errorf("%s = %q was accepted", key, bad)
			}
		}
	}
	// and the threshold that is not in use is checked too: a typo that only
	// bites the day you change your mind is what this file is loud about
	if _, err := Load(write(t, "duplicates.match = none\nduplicates.overlap = 0\n")); err == nil {
		t.Error("a bad threshold beside match = none was accepted")
	}
}
