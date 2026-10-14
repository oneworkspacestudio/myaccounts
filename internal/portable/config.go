package portable

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	InstallServiceRequested bool   `json:"installServiceRequested,omitempty"`
	BackupHours             int    `json:"backupHours,omitempty"`
	Version                 int    `json:"version"`
	Issuer                  string `json:"issuer"`
	Port                    int    `json:"port"`
	Email                   string `json:"adminEmail"`
	Name                    string `json:"adminName"`
	TunnelMode              string `json:"tunnelMode"`
}

var hostname = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

func Validate(c Config) error {
	if c.BackupHours < 0 || c.BackupHours > 168 {
		return errors.New("backup interval must be 0 to 168 hours")
	}
	if c.Version != 1 || c.Port < 1024 || c.Port > 65535 {
		return errors.New("unsupported configuration or invalid port")
	}
	a, e := mail.ParseAddress(c.Email)
	if e != nil || a.Address != c.Email {
		return errors.New("enter a valid email address")
	}
	if strings.TrimSpace(c.Name) == "" || len(c.Name) > 100 {
		return errors.New("enter your name (up to 100 characters)")
	}
	u, e := url.Parse(c.Issuer)
	if e != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("issuer must be an origin URL")
	}
	if u.Scheme == "http" && u.Host == fmt.Sprintf("localhost:%d", c.Port) {
		return nil
	}
	if u.Scheme != "https" || u.Port() != "" || !hostname.MatchString(u.Hostname()) {
		return errors.New("public issuer must use HTTPS and a valid domain")
	}
	return nil
}
func Load(dir string) (Config, error) {
	var c Config
	b, e := os.ReadFile(filepath.Join(dir, "config.json"))
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e == nil {
		e = Validate(c)
	}
	return c, e
}
func Save(dir string, c Config) error {
	if e := Validate(c); e != nil {
		return e
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return AtomicWrite(filepath.Join(dir, "config.json"), b)
}
func AtomicWrite(path string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(path), ".myaccounts-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(b)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(name, path)
}
func DefaultDataDir(exe string) string {
	// Prefer portable data beside the executable. The launcher offers a native folder picker on unsupported volumes.
	return filepath.Join(filepath.Dir(exe), "data")
}
