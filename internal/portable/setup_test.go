package portable

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFirstRunBoundary(t *testing.T) {
	if _, e := Protect([]byte("test")); e != nil {
		t.Skip(e)
	}
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "secrets"), 0700)
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- Setup(ctx, dir, port, func(s string) { opened <- s }) }()
	var browserURL string
	select {
	case browserURL = <-opened:
	case <-ctx.Done():
		t.Fatal("wizard did not open")
	}
	u, _ := url.Parse(browserURL)
	origin := "http://" + u.Host
	p := SetupRequest{Name: "Test Admin", Email: "admin@example.com", Password: "a very long test password", Confirm: "a very long test password", Mode: "token", Issuer: "https://login.example.com", TunnelToken: "test-operational-token"}
	b, _ := json.Marshal(p)
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: 10 * time.Second}
	post := func(originHeader, token, host string) int {
		req, _ := http.NewRequest("POST", origin+"/setup", bytes.NewReader(b))
		req.Header.Set("Origin", originHeader)
		req.Header.Set("X-Setup-Token", token)
		if host != "" {
			req.Host = host
		}
		r, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		_, _ = io.Copy(io.Discard, r.Body)
		return r.StatusCode
	}
	for _, test := range [][3]string{{"https://evil.example", u.Fragment, ""}, {origin, "incorrect", ""}, {origin, u.Fragment, "rebind.evil"}} {
		if got := post(test[0], test[1], test[2]); got != 403 {
			t.Fatalf("setup boundary returned %d", got)
		}
	}
	if _, e = os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(e) {
		t.Fatal("unauthorized setup wrote config")
	}
	if got := post(origin, u.Fragment, ""); got != 200 {
		t.Fatalf("setup returned %d", got)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	c, e := Load(dir)
	if e != nil || c.Email != p.Email {
		t.Fatalf("configured: %+v %v", c, e)
	}
	secret, e := os.ReadFile(filepath.Join(dir, "secrets", "bootstrap.dpapi"))
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(secret), p.Password) {
		t.Fatal("bootstrap credential leaked")
	}
	b, e = os.ReadFile(filepath.Join(dir, "secrets", "tunnel.dpapi"))
	if e != nil {
		t.Fatal(e)
	}
	plain, e := Unprotect(b)
	if e != nil || string(plain) != p.TunnelToken {
		t.Fatal("operational credential did not persist")
	}
	plain, e = Unprotect(secret)
	if e != nil || strings.Contains(string(plain), p.TunnelToken) {
		t.Fatal("operational token leaked into bootstrap")
	}
}
