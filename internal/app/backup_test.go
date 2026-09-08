package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func backupFiles(t *testing.T, a *App) []string {
	t.Helper()
	entries, err := os.ReadDir(BackupDir(a.path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestBackupIsNamedForItsHourAndReadableOnItsOwn(t *testing.T) {
	a, now := newTestApp(t)
	if _, _, err := a.Capture("Pay the rent"); err != nil {
		t.Fatal(err)
	}
	file, err := a.Backup(*now)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := filepath.Base(file), "test-2026-09-04T10.db"; got != want {
		t.Errorf("backup named %q, want %q", got, want)
	}

	// the snapshot has to stand alone: no -wal beside it to replay, and the
	// capture made a moment ago in it
	if _, err := os.Stat(file + "-wal"); !os.IsNotExist(err) {
		t.Errorf("snapshot left a WAL file beside it")
	}
	restored, err := Open(file, time.UTC)
	if err != nil {
		t.Fatalf("snapshot does not open: %v", err)
	}
	defer restored.Close()
	items, err := restored.Inbox()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Text != "Pay the rent" {
		t.Errorf("snapshot holds %v, want the one capture", itemTexts(items))
	}
}

// The same hour twice is the same file: a restart mid-hour must not cost that
// hour a second name, and must not fail either, which is what VACUUM INTO on
// its own would do.
func TestBackupTwiceInAnHourOverwrites(t *testing.T) {
	a, now := newTestApp(t)
	if _, err := a.Backup(*now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Capture("Later in the hour"); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(30 * time.Minute)
	file, err := a.Backup(*now)
	if err != nil {
		t.Fatalf("second backup in the hour: %v", err)
	}
	if files := backupFiles(t, a); len(files) != 1 {
		t.Fatalf("want one file for the hour, got %v", files)
	}
	restored, err := Open(file, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if items, _ := restored.Inbox(); len(items) != 1 {
		t.Errorf("the second snapshot is not the later one: %v", itemTexts(items))
	}
}

func TestPruneKeepsTheNewest(t *testing.T) {
	a, now := newTestApp(t)
	keep := 2 * 24
	// two days and a bit of hours, taken one an hour as the app takes them
	for i := 0; i < keep+5; i++ {
		if _, err := a.Backup(*now); err != nil {
			t.Fatal(err)
		}
		if _, err := PruneBackups(a.path, keep); err != nil {
			t.Fatal(err)
		}
		*now = now.Add(time.Hour)
	}
	files := backupFiles(t, a)
	if len(files) != keep {
		t.Fatalf("kept %d files, want %d", len(files), keep)
	}
	// the oldest five are the ones that went, and the newest is still here
	if files[0] != "test-2026-09-04T15.db" {
		t.Errorf("oldest kept is %q, want the 5 before it gone", files[0])
	}
	if last := files[len(files)-1]; last != "test-2026-09-06T14.db" {
		t.Errorf("newest kept is %q, want the last one taken", last)
	}
}

// Keeping none is how the settings file turns backups off, and it has to take
// with it the ones already on disk — otherwise turning them off leaves a pile
// that nothing will ever tidy.
func TestPruneToNoneRemovesThemAll(t *testing.T) {
	a, now := newTestApp(t)
	for i := 0; i < 3; i++ {
		if _, err := a.Backup(*now); err != nil {
			t.Fatal(err)
		}
		*now = now.Add(time.Hour)
	}
	if _, err := PruneBackups(a.path, 0); err != nil {
		t.Fatal(err)
	}
	if files := backupFiles(t, a); len(files) != 0 {
		t.Errorf("keeping none left %v", files)
	}
}

// BackupHourly is the off switch's caller: backup.days = 0 has to reach the
// tidy-up above, not return before it.
func TestHourlyWithNoDaysTidiesUpAndReturns(t *testing.T) {
	a, now := newTestApp(t)
	if _, err := a.Backup(*now); err != nil {
		t.Fatal(err)
	}
	a.BackupHourly(0) // returns synchronously when off
	if files := backupFiles(t, a); len(files) != 0 {
		t.Errorf("turning backups off left %v", files)
	}
}

// The backup directory sits next to someone's data, so pruning deletes what
// this app wrote and nothing else.
func TestPruneLeavesForeignFilesAlone(t *testing.T) {
	a, now := newTestApp(t)
	if _, err := a.Backup(*now); err != nil {
		t.Fatal(err)
	}
	stranger := filepath.Join(BackupDir(a.path), "notes.txt")
	if err := os.WriteFile(stranger, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := PruneBackups(a.path, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stranger); err != nil {
		t.Errorf("a file this app did not write was removed: %v", err)
	}
}

func TestIsBackupName(t *testing.T) {
	db := "/data/test.db"
	for _, ok := range []string{"test-2026-09-04T10.db", "test-2000-01-01T00.db"} {
		if !isBackupName(db, ok) {
			t.Errorf("%q was not recognised as a snapshot", ok)
		}
	}
	for _, no := range []string{
		"test-2026-09-04T10.db.part", // a snapshot still being written
		"test-2026-09-04.db",         // no hour in it
		"test-notadate.db",
		"other-2026-09-04T10.db", // another database's snapshot
		"test.db",
	} {
		if isBackupName(db, no) {
			t.Errorf("%q was taken for a snapshot", no)
		}
	}
}
