package portable

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"filippo.io/age"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreRefusesWindowsPathTraversal(t *testing.T) {
	root := t.TempDir()
	password := "synthetic archive test passphrase"
	for i, name := range []string{"../escaped", ".. /escaped", "folder/.. /escaped", "C:/escaped", "folder\\escaped", "CON", "NUL.txt"} {
		var plain bytes.Buffer
		z := zip.NewWriter(&plain)
		w, e := z.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		w.Write([]byte("must not escape"))
		z.Close()
		r, e := age.NewScryptRecipient(password)
		if e != nil {
			t.Fatal(e)
		}
		r.SetWorkFactor(10)
		var encrypted bytes.Buffer
		out, e := age.Encrypt(&encrypted, r)
		if e != nil {
			t.Fatal(e)
		}
		out.Write(plain.Bytes())
		out.Close()
		file := filepath.Join(root, "malicious.age")
		os.WriteFile(file, encrypted.Bytes(), 0600)
		dest := filepath.Join(root, fmt.Sprint("restored-", i))
		if e = Restore(file, dest, password); e == nil {
			t.Fatalf("unsafe path accepted: %q", name)
		}
		if _, e = os.Stat(filepath.Join(root, "escaped")); !os.IsNotExist(e) {
			t.Fatal("restore wrote outside its staging directory")
		}
		if _, e = os.Stat(dest); !os.IsNotExist(e) {
			t.Fatal("failed restore created its destination")
		}
	}
}

func TestEncryptedBackupRestore(t *testing.T) {
	root := t.TempDir()
	d := filepath.Join(root, "source")
	os.Mkdir(d, 0700)
	c := Config{Version: 1, Issuer: "http://localhost:8787", Port: 8787, Email: "admin@example.com", Name: "Administrator", TunnelMode: "local"}
	if e := Save(d, c); e != nil {
		t.Fatal(e)
	}
	db, e := sql.Open("sqlite", filepath.Join(d, "users.db"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec("CREATE TABLE identity(id TEXT PRIMARY KEY); INSERT INTO identity VALUES ('stable-test-id')")
	db.Close()
	if e != nil {
		t.Fatal(e)
	}
	original, _ := os.ReadFile(filepath.Join(d, "users.db"))
	if protected, e := Protect([]byte("test-backup-tunnel-token")); e == nil {
		os.Mkdir(filepath.Join(d, "secrets"), 0700)
		os.WriteFile(filepath.Join(d, "secrets", "tunnel.dpapi"), protected, 0600)
		emailSecret, _ := Protect([]byte("synthetic-email-credential"))
		os.WriteFile(filepath.Join(d, "secrets", "email-relay.dpapi"), emailSecret, 0600)
	}
	backup := filepath.Join(root, "backup.age")
	password := "long portable test passphrase"
	if e := Backup(d, backup, password); e != nil {
		t.Fatal(e)
	}
	dest := filepath.Join(root, "restored")
	if e := Restore(backup, dest, "wrong password"); e == nil {
		t.Fatal("wrong passphrase accepted")
	}
	if _, e := os.Stat(dest); !os.IsNotExist(e) {
		t.Fatal("failed restore created destination")
	}
	if e := Restore(backup, dest, password); e != nil {
		t.Fatal(e)
	}
	if b, e := os.ReadFile(filepath.Join(dest, "secrets", "email-relay.dpapi")); e == nil {
		plain, e := Unprotect(b)
		if e != nil || string(plain) != "synthetic-email-credential" {
			t.Fatal("restored email credential could not be decrypted")
		}
		if _, e = os.Stat(filepath.Join(dest, "secrets", "email-relay.portable")); !os.IsNotExist(e) {
			t.Fatal("portable plaintext email credential left after restore")
		}
	}
	b, e := os.ReadFile(filepath.Join(dest, "users.db"))
	if e != nil || string(b) != string(original) {
		t.Fatal("identities not preserved")
	}
	if protected, e := os.ReadFile(filepath.Join(dest, "secrets", "tunnel.dpapi")); e == nil {
		plain, e := Unprotect(protected)
		if e != nil || string(plain) != "test-backup-tunnel-token" {
			t.Fatal("restored tunnel credential cannot be decrypted")
		}
		if _, e = os.Stat(filepath.Join(dest, "secrets", "tunnel.portable")); !os.IsNotExist(e) {
			t.Fatal("plaintext tunnel credential left in restore")
		}
	}
	if e := Restore(backup, dest, password); e == nil {
		t.Fatal("existing identities overwritten")
	}
	if e := Backup(d, backup, password); e == nil {
		t.Fatal("existing backup overwritten")
	}
	b, _ = os.ReadFile(backup)
	b[len(b)-1] ^= 1
	os.WriteFile(backup, b, 0600)
	if e := Restore(backup, filepath.Join(root, "corrupt"), password); e == nil {
		t.Fatal("tampered backup accepted")
	}
}
