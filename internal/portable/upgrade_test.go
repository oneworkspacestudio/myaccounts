package portable

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedUpgradeRestoresStableIdentity(t *testing.T) {
	dir := t.TempDir()
	if e := ProtectDir(dir); e != nil {
		t.Fatal(e)
	}
	for _, sub := range []string{"secrets", "backups", "logs"} {
		if e := os.Mkdir(filepath.Join(dir, sub), 0700); e != nil {
			t.Fatal(e)
		}
	}
	if e := Save(dir, Config{Version: 1, Issuer: "http://localhost:18787", Port: 18787, Email: "admin@example.com", Name: "Test", TunnelMode: "local"}); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "users.db"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE identities(id TEXT);INSERT INTO identities VALUES ('original-subject')"); e != nil {
		t.Fatal(e)
	}
	db.Close()
	exe := filepath.Join(t.TempDir(), "synthetic-executable")
	os.WriteFile(exe, []byte("new-version"), 0600)
	guard, e := PrepareUpgrade(dir, exe, "test-new-version")
	if e != nil || !guard.HasSnapshot() {
		t.Fatal("missing pre-upgrade snapshot", e)
	}
	db, e = sql.Open("sqlite", filepath.Join(dir, "users.db"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec("DELETE FROM identities")
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	if e = guard.Rollback(); e != nil {
		t.Fatal(e)
	}
	db, e = sql.Open("sqlite", filepath.Join(dir, "users.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var id string
	if e = db.QueryRow("SELECT id FROM identities").Scan(&id); e != nil || id != "original-subject" {
		t.Fatal("identity lost during rollback", e)
	}
	if e = guard.Commit(); e != nil {
		t.Fatal(e)
	}
	again, e := PrepareUpgrade(dir, exe, "same-version")
	if e != nil || again.HasSnapshot() {
		t.Fatal("unchanged executable created redundant snapshot", e)
	}
}
