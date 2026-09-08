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
