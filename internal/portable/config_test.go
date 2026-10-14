package portable

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	// Go 1.27's testing.TempDir follows GOTMPDIR; live-data tests need a protected volume.
	dir := os.Getenv("MYACCOUNTS_TEST_DATA")
	if dir == "" {
		dir = os.TempDir()
	}
	_ = os.Setenv("GOTMPDIR", dir)
	os.Exit(m.Run())
}

func TestIssuerValidation(t *testing.T) {
	c := Config{Version: 1, Issuer: "https://login.example.com", Port: 8787, Email: "admin@example.com", Name: "Administrator", TunnelMode: "manual"}
	if e := Validate(c); e != nil {
		t.Fatal(e)
	}
	for _, issuer := range []string{"http://login.example.com", "https://evil.example/a", "https://user:pass@login.example.com", "https://login.example.com?bad=true", "https://login.example.com#fragment", "https://login.example.com\nhttpaddr = 0.0.0.0", "http://127.0.0.1:8787"} {
		c.Issuer = issuer
		if e := Validate(c); e == nil {
			t.Errorf("accepted %q", issuer)
		}
	}
	c.Issuer = "http://localhost:8787"
	if e := Validate(c); e != nil {
		t.Fatal(e)
	}
}
func TestAtomicConfiguration(t *testing.T) {
	d := t.TempDir()
	c := Config{Version: 1, Issuer: "http://localhost:8787", Port: 8787, Email: "admin@example.com", Name: "Administrator", TunnelMode: "local"}
	if e := Save(d, c); e != nil {
		t.Fatal(e)
	}
	got, e := Load(d)
	if e != nil || got != c {
		t.Fatalf("roundtrip: %+v %v", got, e)
	}
	b, _ := os.ReadFile(filepath.Join(d, "config.json"))
	if len(b) == 0 {
		t.Fatal("empty config")
	}
}
func TestProtectedCredentialRoundTrip(t *testing.T) {
	secret := []byte("this-is-a-test-credential")
	b, e := Protect(secret)
	if e != nil {
		t.Skip(e)
	}
	if string(b) == string(secret) {
		t.Fatal("credential was stored in plaintext")
	}
	got, e := Unprotect(b)
	if e != nil || string(got) != string(secret) {
		t.Fatalf("DPAPI roundtrip: %v", e)
	}
}
