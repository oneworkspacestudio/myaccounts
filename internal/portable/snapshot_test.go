package portable

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOnlineBackupCapturesCommittedWAL(t *testing.T) {
	dir := t.TempDir()
	if e := ProtectDir(dir); e != nil {
		t.Fatal(e)
	}
	if e := Save(dir, Config{Version: 1, Issuer: "http://localhost:18787", Port: 18787, Email: "admin@example.com", Name: "Test", TunnelMode: "local"}); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "users.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, stmt := range []string{"PRAGMA journal_mode=WAL", "PRAGMA wal_autocheckpoint=0", "CREATE TABLE identities(id TEXT)", "INSERT INTO identities VALUES ('stable-subject')"} {
		if _, e = db.Exec(stmt); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = os.Stat(filepath.Join(dir, "users.db-wal")); e != nil {
		t.Fatal("test did not exercise WAL", e)
	}
	archive := filepath.Join(t.TempDir(), "online.age")
	pass := "synthetic strong snapshot passphrase"
	if e = OnlineBackup(dir, archive, pass); e != nil {
		t.Fatal(e)
	}
	restored := filepath.Join(t.TempDir(), "restored")
	if e = Restore(archive, restored, pass); e != nil {
		t.Fatal(e)
	}
	copy, e := sql.Open("sqlite", filepath.Join(restored, "users.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer copy.Close()
	var id string
	if e = copy.QueryRow("SELECT id FROM identities").Scan(&id); e != nil || id != "stable-subject" {
		t.Fatal("committed WAL identity not restored", e)
	}
}
