package portable

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var snapshotMutex sync.Mutex

// OnlineBackup uses SQLite's consistent VACUUM INTO snapshot rather than copying
// a live database while committed pages may still be in WAL.
func OnlineBackup(dir, output, password string) error {
	snapshotMutex.Lock()
	defer snapshotMutex.Unlock()
	if len(password) < 16 {
		return errors.New("backup passphrase must contain at least 16 characters")
	}
	stage, e := os.MkdirTemp(dir, ".myaccounts-snapshot-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	if e = ProtectDir(stage); e != nil {
		return e
	}
	db, e := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(dir, "users.db"))+"?mode=ro&_pragma=busy_timeout(10000)")
	if e != nil {
		return e
	}
	defer db.Close()
	if _, e = db.Exec("VACUUM INTO ?", filepath.Join(stage, "users.db")); e != nil {
		return errors.New("cannot create consistent SQLite snapshot")
	}
	e = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		first := strings.Split(filepath.ToSlash(rel), "/")[0]
		if first == "logs" || first == "tmp" || first == "backups" || first == "web" || first == "tools" || strings.HasPrefix(first, ".myaccounts-") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return errors.New("snapshot refuses symbolic links")
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(stage, rel), 0700)
		}
		if rel == ".owner-sid" {
			return nil
		}
		if first == "users.db" || first == "users.db-wal" || first == "users.db-shm" || strings.HasSuffix(rel, ".lock") || filepath.ToSlash(rel) == "secrets/control.dpapi" || filepath.ToSlash(rel) == "secrets/setup-url.dpapi" || filepath.ToSlash(rel) == "secrets/backup-passphrase.dpapi" || filepath.ToSlash(rel) == "secrets/migration-key.dpapi" {
			return nil
		}
		before, err := d.Info()
		if err != nil {
			return err
		}
		if before.Size() > 512<<20 {
			return errors.New("snapshot file exceeds restore size limit")
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		after, err := os.Stat(path)
		if err != nil {
			return err
		}
		if before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || int64(len(b)) != before.Size() {
			return errors.New("a file changed during snapshot; retry after uploads finish")
		}
		return AtomicWrite(filepath.Join(stage, rel), b)
	})
	if e != nil {
		return e
	}
	return Backup(stage, output, password)
}
