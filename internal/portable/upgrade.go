package portable

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type UpgradeGuard struct{ dir, hash, version, archive, password string }
type InstallVersion struct {
	Hash    string `json:"sha256"`
	Version string `json:"version"`
}

// PrepareUpgrade keeps a protected encrypted snapshot before a changed executable
// can run upstream schema synchronization. A first installation has no old DB.
func PrepareUpgrade(dir, exe, version string) (*UpgradeGuard, error) {
	f, e := os.Open(exe)
	if e != nil {
		return nil, e
	}
	h := sha256.New()
	_, e = io.Copy(h, f)
	f.Close()
	if e != nil {
		return nil, e
	}
	guard := &UpgradeGuard{dir: dir, hash: hex.EncodeToString(h.Sum(nil)), version: version}
	var previous InstallVersion
	if b, e := os.ReadFile(filepath.Join(dir, "install-version.json")); e == nil {
		_ = json.Unmarshal(b, &previous)
	}
	if previous.Hash == guard.hash {
		return guard, nil
	}
	if _, e = os.Stat(filepath.Join(dir, "users.db")); os.IsNotExist(e) {
		return guard, nil
	} else if e != nil {
		return nil, e
	}
	keyPath := filepath.Join(dir, "secrets", "migration-key.dpapi")
	if b, e := os.ReadFile(keyPath); e == nil {
		plain, e := Unprotect(b)
		if e != nil {
			return nil, errors.New("migration snapshot key needs recovery")
		}
		guard.password = string(plain)
	} else if os.IsNotExist(e) {
		guard.password = RandomToken()
		b, e := Protect([]byte(guard.password))
		if e != nil {
			return nil, e
		}
		if e = AtomicWrite(keyPath, b); e != nil {
			return nil, e
		}
	} else {
		return nil, e
	}
	guard.archive = filepath.Join(dir, "backups", "upgrade-"+BackupName())
	if e = OnlineBackup(dir, guard.archive, guard.password); e != nil {
		return nil, e
	}
	return guard, nil
}
func (g *UpgradeGuard) Commit() error {
	b, e := json.Marshal(InstallVersion{Hash: g.hash, Version: g.version})
	if e != nil {
		return e
	}
	return AtomicWrite(filepath.Join(g.dir, "install-version.json"), b)
}
func (g *UpgradeGuard) HasSnapshot() bool { return g != nil && g.archive != "" }
func (g *UpgradeGuard) Rollback() error {
	if !g.HasSnapshot() {
		return nil
	}
	lock := flock.New(filepath.Join(g.dir, "engine.lock"))
	ok, e := lock.TryLock()
	if e != nil {
		return e
	}
	if !ok {
		return errors.New("engine still owns the database; encrypted pre-upgrade snapshot retained")
	}
	defer lock.Unlock()
	restore := filepath.Join(g.dir, ".myaccounts-rollback-"+RandomToken()[:8])
	if e = Restore(g.archive, restore, g.password); e != nil {
		return e
	}
	defer os.RemoveAll(restore)
	quarantine := filepath.Join(g.dir, "backups", "failed-migration-"+RandomToken()[:8]+".db")
	if e = os.Rename(filepath.Join(g.dir, "users.db"), quarantine); e != nil {
		return e
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		path := filepath.Join(g.dir, "users.db"+suffix)
		if _, e = os.Stat(path); e == nil {
			if e = os.Rename(path, quarantine+suffix); e != nil {
				return e
			}
		}
	}
	if e = os.Rename(filepath.Join(restore, "users.db"), filepath.Join(g.dir, "users.db")); e != nil {
		_ = os.Rename(quarantine, filepath.Join(g.dir, "users.db"))
		return e
	}
	return CheckDatabase(filepath.Join(g.dir, "users.db"))
}
