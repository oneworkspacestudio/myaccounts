package portable

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/casdoor/casdoor/internal/mailrelay"
)

func TestWizardSharedEmailOwnershipAndProtectedStorage(t *testing.T) {
	if _, e := Protect([]byte("test")); e != nil {
		t.Skip(e)
	}
	dir := t.TempDir()
	if e := os.Mkdir(filepath.Join(dir, "secrets"), 0700); e != nil {
		t.Fatal(e)
	}
	var delivery mailrelay.Delivery
	relay, e := mailrelay.Open(filepath.Join(t.TempDir(), "relay.db"), func(_ context.Context, m mailrelay.Delivery) error { delivery = m; return nil })
	if e != nil {
		t.Fatal(e)
	}
	defer relay.Close()
	host := httptest.NewTLSServer(relay.Handler())
	defer host.Close()
	previousURL, previousTransport := mailrelay.DefaultURL, http.DefaultTransport
	mailrelay.DefaultURL = host.URL
	http.DefaultTransport = host.Client().Transport
	defer func() { mailrelay.DefaultURL = previousURL; http.DefaultTransport = previousTransport }()
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() { done <- Setup(ctx, dir, port, func(link string) { opened <- link }) }()
	var link string
	select {
	case link = <-opened:
	case <-ctx.Done():
		t.Fatal("setup not opened")
	}
	u, _ := url.Parse(link)
	origin := "http://" + u.Host
	p := SetupRequest{Name: "Fixture Admin", Email: "admin@example.com", Password: "a long fixture password", Confirm: "a long fixture password", Mode: "local", EmailMode: "shared"}
	post := func(path string, goodToken bool) int {
		b, _ := json.Marshal(p)
		r, _ := http.NewRequest("POST", origin+path, bytes.NewReader(b))
		r.Header.Set("Origin", origin)
		r.Header.Set("Content-Type", "application/json")
		if goodToken {
			r.Header.Set("X-Setup-Token", u.Fragment)
		}
		response, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode
	}
	if post("/email/begin", false) != 403 {
		t.Fatal("anonymous email enrollment permitted")
	}
	if post("/setup", true) != 400 {
		t.Fatal("unconfirmed shared email accepted")
	}
	if post("/email/begin", true) != 200 {
		t.Fatal("enrollment failed")
	}
	p.EmailCode = regexp.MustCompile(`\b[0-9]{6}\b`).FindString(delivery.Text)
	if p.EmailCode == "" || post("/email/confirm", true) != 200 {
		t.Fatal("email verification failed")
	}
	p.Email = "Admin@example.com"
	if post("/setup", true) != 400 {
		t.Fatal("email proof crossed a case-sensitive mailbox")
	}
	p.Email = "other@example.com"
	if post("/setup", true) != 400 {
		t.Fatal("confirmed email applied to another administrator")
	}
	p.Email = "admin@example.com"
	if post("/setup", true) != 200 {
		t.Fatal("verified setup failed")
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	credential, e := LoadSharedEmail(dir)
	if e != nil {
		t.Fatal(e)
	}
	if credential.Email != p.Email || credential.Issuer != origin || credential.ID == "" || len(credential.Secret) != 64 {
		t.Fatal("email connection not saved")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "secrets", "email-relay.dpapi"))
	if strings.Contains(string(b), credential.Secret) {
		t.Fatal("unprotected installation credential")
	}
	b, _ = os.ReadFile(filepath.Join(dir, "secrets", "bootstrap.dpapi"))
	plain, e := Unprotect(b)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(plain), credential.Secret) || strings.Contains(string(plain), p.EmailCode) {
		t.Fatal("credential or code copied into administrator bootstrap")
	}
	if e = SendSharedEmail(context.Background(), dir, "user@example.com", "password-reset", "123456"); e != nil {
		t.Fatal(e)
	}
	client, _ := mailrelay.NewClient(host.URL)
	replacement, e := client.Begin(ctx, p.Email, origin)
	if e != nil {
		t.Fatal(e)
	}
	code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(delivery.Text)
	if e = client.Confirm(ctx, replacement, code); e != nil {
		t.Fatal(e)
	}
	if e = SaveSharedEmail(dir, replacement); e != nil {
		t.Fatal(e)
	}
	if e = client.Status(ctx, credential); e == nil {
		t.Fatal("reconnection left prior credential active")
	}
	if e = client.Status(ctx, replacement); e != nil {
		t.Fatal("replacement credential not active")
	}
}
