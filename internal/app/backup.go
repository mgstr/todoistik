package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backups: one snapshot of the database an hour, kept beside it, oldest first
// out. See implementation.md, "Backups" for why they are taken here rather
// than left to whatever else runs on the machine.

// BackupSuffix names the directory the snapshots live in: the database file's
// own name plus this, so a directory listing says which database it holds and
// two databases in one directory cannot share a pile of backups.
const BackupSuffix = ".backups"

// backupStamp is the hour a snapshot is of. No colon, because a colon in a
// filename is a fight with some filesystem eventually, and this sorts
// lexicographically into chronological order — which is what makes "the
// oldest" a matter of sorting the names rather than trusting an mtime.
const backupStamp = "2006-01-02T15"

// BackupDir is where snapshots of the database at path are kept.
func BackupDir(path string) string { return path + BackupSuffix }

// backupName is the file one hour's snapshot goes in: the database's own base
// name, the hour, and the extension it had. One name per hour, so an hour
// backed up twice is backed up over rather than twice.
func backupName(path string, at time.Time) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext) + "-" + at.Format(backupStamp) + ext
}

// Backup writes a snapshot of the database as of now into its backup
// directory and returns the file it wrote.
//
// VACUUM INTO rather than copying the file: with WAL there are three files on
// disk and the newest writes are in the one that is not the database, so a
// copy of the database alone is a copy of some older moment. What this writes
// is a single consistent file with nothing to replay.
func (a *App) Backup(at time.Time) (string, error) {
	dir := BackupDir(a.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	final := filepath.Join(dir, backupName(a.path, at.In(a.loc)))
	// VACUUM INTO refuses to write a file that exists, and an hour can be
	// backed up again — a restart, a clock change. Writing beside it and
	// renaming over is also what stops a snapshot interrupted halfway from
	// being left behind looking like a good one.
	tmp := final + ".part"
	if err := os.Remove(tmp); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if _, err := a.db.Exec(`VACUUM INTO ?`, tmp); err != nil {
		return "", fmt.Errorf("vacuum into %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", err
	}
	return final, nil
}

// PruneBackups deletes the oldest snapshots until keep of them are left, and
// returns how many it removed. Keeping none removes them all, which is what
// backup.days = 0 asks for.
//
// Oldest is decided by the names, which carry the hour and sort into order —
// not by mtime, which a copy or a restore would rewrite. Anything in the
// directory that is not a snapshot of this database is left alone: it is a
// directory beside someone's data, and deleting what it did not write is not
// this function's business.
func PruneBackups(path string, keep int) (int, error) {
	dir := BackupDir(path)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && isBackupName(path, e.Name()) {
			names = append(names, e.Name())
		}
	}
	if len(names) <= keep {
		return 0, nil
	}
	sort.Strings(names)
	removed := 0
	for _, name := range names[:len(names)-keep] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// isBackupName reports whether a file in the backup directory is one of this
// database's snapshots: its base name, a stamp that parses as an hour, and its
// extension. A half-written ".part" fails on the extension, as it should.
func isBackupName(path, name string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext) + "-"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ext) {
		return false
	}
	stamp := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ext)
	_, err := time.ParseInLocation(backupStamp, stamp, time.UTC)
	return err == nil
}

// BackupHourly takes a snapshot now and then on every hour, keeping days×24 of
// them. It runs until the process does not, and never stops for an error: a
// backup that fails is worth a line in the log and nothing else, because the
// thing it protects is still running and losing that is the failure that
// matters.
//
// Days of zero (or less) means no backups at all: the existing snapshots go
// too, because turning backups off has to tidy up or it leaves a pile nothing
// will ever come back for — and then this returns.
func (a *App) BackupHourly(days int) {
	if days <= 0 {
		if _, err := PruneBackups(a.path, 0); err != nil {
			log.Printf("backup prune: %v", err)
		}
		return
	}
	keep := days * 24
	for {
		a.backupNow(keep)
		// on the hour, not an hour from whenever this started: the file is
		// named for its hour, so a run drifting past one would leave that hour
		// with no snapshot and some other hour with two names for the same one
		now := a.now()
		time.Sleep(now.Truncate(time.Hour).Add(time.Hour).Sub(now))
	}
}

func (a *App) backupNow(keep int) {
	if _, err := a.Backup(a.now()); err != nil {
		log.Printf("backup: %v", err)
		return
	}
	if _, err := PruneBackups(a.path, keep); err != nil {
		log.Printf("backup prune: %v", err)
	}
}
