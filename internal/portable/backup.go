package portable

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"errors"
	"filippo.io/age"
	"fmt"
	"io"
	"io/fs"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
)

// Backups are offline, so SQLite and uploaded resources form one consistent snapshot.
func Backup(dir, output, password string) error {
	if _, e := Load(dir); e != nil {
		return e
	}
	if e := CheckDatabase(filepath.Join(dir, "users.db")); e != nil {
		return e
	}
	if _, e := os.Stat(filepath.Join(dir, "secrets", "bootstrap.dpapi")); !os.IsNotExist(e) {
		return errors.New("finish administrator initialization before backup")
	}
	if len(password) < 16 {
		return errors.New("backup passphrase must contain at least 16 characters")
	}
	r, e := age.NewScryptRecipient(password)
	if e != nil {
		return e
	}
	var archive bytes.Buffer
	z := zip.NewWriter(&archive)
	var sourceSize int64
	e = filepath.WalkDir(dir, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(dir, path)
		if e != nil {
			return e
		}
		if rel == "." {
			return nil
		}
		first := strings.Split(filepath.ToSlash(rel), "/")[0]
		if first == "backups" || first == "logs" || first == "web" || first == "tmp" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return errors.New("backup refuses symbolic links")
		}
		if d.IsDir() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		sourceSize += info.Size()
		if sourceSize > 512<<20 {
			return errors.New("backup exceeds the 512 MiB portable restore limit")
		}
		if strings.HasSuffix(rel, ".lock") {
			return nil
		}
		if filepath.ToSlash(rel) == "secrets/control.dpapi" {
			return nil
		}
		if filepath.ToSlash(rel) == "secrets/tunnel.dpapi" {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			plain, e := Unprotect(b)
			if e != nil {
				return e
			}
			w, e := z.Create("secrets/tunnel.portable")
			if e != nil {
				return e
			}
			_, e = w.Write(plain)
			return e
		}
		f, e := os.Open(path)
		if e != nil {
			return e
		}
		defer f.Close()
		w, e := z.Create(filepath.ToSlash(rel))
		if e != nil {
			return e
		}
		_, e = io.Copy(w, f)
		return e
	})
	if e != nil {
		return e
	}
	if e = z.Close(); e != nil {
		return e
	}
	if archive.Len() > 512<<20 {
		return errors.New("backup archive exceeds the restore limit")
	}
	var encrypted bytes.Buffer
	w, e := age.Encrypt(&encrypted, r)
	if e != nil {
		return e
	}
	if _, e = w.Write(archive.Bytes()); e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	if _, e = os.Stat(output); !os.IsNotExist(e) {
		return errors.New("backup output already exists or cannot be inspected")
	}
	return AtomicWrite(output, encrypted.Bytes())
}
func Restore(input, dir, password string) error {
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		return errors.New("restore destination must not exist; existing identities will never be overwritten")
	}
	info, e := os.Stat(input)
	if e != nil {
		return e
	}
	if info.Size() > 514<<20 {
		return errors.New("encrypted backup exceeds the restore limit")
	}
	b, e := os.ReadFile(input)
	if e != nil {
		return e
	}
	id, e := age.NewScryptIdentity(password)
	if e != nil {
		return e
	}
	r, e := age.Decrypt(bytes.NewReader(b), id)
	if e != nil {
		return e
	}
	plain, e := io.ReadAll(io.LimitReader(r, 512<<20+1))
	if e != nil {
		return e
	}
	if len(plain) > 512<<20 {
		return errors.New("backup exceeds 512 MiB restore limit")
	}
	z, e := zip.NewReader(bytes.NewReader(plain), int64(len(plain)))
	if e != nil {
		return e
	}
	stage, e := os.MkdirTemp(filepath.Dir(dir), ".myaccounts-restore-*")
	if e != nil {
		return e
	}
	defer os.RemoveAll(stage)
	if e = ProtectDir(stage); e != nil {
		return e
	}
	var total uint64
	seen := map[string]bool{}
	for _, f := range z.File {
		if !validArchiveName(f.Name) || f.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe archive member: %s", f.Name)
		}
		key := strings.ToLower(f.Name)
		if seen[key] {
			return errors.New("duplicate archive member")
		}
		seen[key] = true
		total += f.UncompressedSize64
		if total > 512<<20 {
			return errors.New("expanded archive exceeds limit")
		}
		target := filepath.Join(stage, filepath.FromSlash(f.Name))
		if f.FileInfo().IsDir() {
			if e = os.MkdirAll(target, 0700); e != nil {
				return e
			}
			continue
		}
		if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
			return e
		}
		r, e := f.Open()
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(r, 512<<20+1))
		r.Close()
		if e != nil {
			return e
		}
		if e = AtomicWrite(target, b); e != nil {
			return e
		}
	}
	if _, e = Load(stage); e != nil {
		return fmt.Errorf("backup configuration validation: %w", e)
	}
	if _, e = os.Stat(filepath.Join(stage, "users.db")); e != nil {
		return errors.New("backup has no identity database")
	}
	if e = CheckDatabase(filepath.Join(stage, "users.db")); e != nil {
		return e
	}
	if b, e := os.ReadFile(filepath.Join(stage, "secrets", "tunnel.portable")); e == nil {
		protected, e := Protect(b)
		if e != nil {
			return e
		}
		if e = AtomicWrite(filepath.Join(stage, "secrets", "tunnel.dpapi"), protected); e != nil {
			return e
		}
		if e = os.Remove(filepath.Join(stage, "secrets", "tunnel.portable")); e != nil {
			return e
		}
	}
	return os.Rename(stage, dir)
}
func validArchiveName(name string) bool {
	if !fs.ValidPath(name) || strings.ContainsAny(name, ":\\") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		// Windows normalizes trailing spaces/dots, including ".. "; reject before extraction.
		if strings.TrimRight(part, ". ") != part {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		switch base {
		case "CON", "PRN", "AUX", "NUL", "CLOCK$":
			return false
		}
		if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return false
		}
	}
	return true
}
func CheckDatabase(path string) error {
	db, e := sql.Open("sqlite", path+"?mode=ro")
	if e != nil {
		return e
	}
	defer db.Close()
	var result string
	if e = db.QueryRow("PRAGMA quick_check").Scan(&result); e != nil {
		return fmt.Errorf("SQLite integrity check: %w", e)
	}
	if result != "ok" {
		return errors.New("SQLite integrity check failed")
	}
	return nil
}
