package mailservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/casdoor/casdoor/internal/mailrelay"
	"github.com/casdoor/casdoor/internal/portable"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestOwnerCanSkipWithoutSendingOrSaving(t *testing.T) {
	if _, err := portable.Protect([]byte("probe")); err != nil {
		t.Skip(err)
	}
	dir, err := os.MkdirTemp(os.Getenv("MYACCOUNTS_TEST_DATA"), "mail-skip-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	if err = os.Mkdir(filepath.Join(dir, "secrets"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = portable.ProtectDir(dir); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- wizard(ctx, dir, 0, func(link string) { opened <- link }, func(context.Context, mailrelay.SMTPConfig, mailrelay.Delivery) error {
			return errors.New("skip must never invoke delivery")
		})
	}()
	var link string
	select {
	case link = <-opened:
	case <-ctx.Done():
		t.Fatal("owner wizard did not open")
	}
	u, _ := url.Parse(link)
	origin := "http://" + u.Host
	post := func(token, from string) int {
		r, _ := http.NewRequest("POST", origin+"/owner/skip", strings.NewReader(`{}`))
		r.Header.Set("Origin", from)
		r.Header.Set("X-Setup-Token", token)
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}
	if post("wrong", origin) != 403 || post(u.Fragment, "https://evil.example") != 403 {
		t.Fatal("unauthorized owner skip accepted")
	}
	if post(u.Fragment, origin) != 200 {
		t.Fatal("skip required a sending account")
	}
	if err = <-done; !errors.Is(err, ErrSkipped) {
		t.Fatalf("expected skip result, got %v", err)
	}
	for _, path := range []string{"service.dpapi", "secrets/tunnel.dpapi", "secrets/owner-setup-url.dpapi", "relay.db"} {
		if _, err = os.Stat(filepath.Join(dir, path)); !os.IsNotExist(err) {
			t.Fatalf("skip retained %s", path)
		}
	}
}

func TestOwnerGmailPresetAndConnectionValidation(t *testing.T) {
	p := ownerRequest{Email: "sender@example.com", Provider: "gmail", Password: "a-test-app-password", PublicURL: "https://mail.example.com", Mode: "manual"}
	c, e := p.config(19890)
	if e != nil || c.SMTP.Host != "smtp.gmail.com" || c.SMTP.Username != p.Email || c.SMTP.From != p.Email || c.SMTP.Port != 465 {
		t.Fatalf("invalid preset %v", e)
	}
	for _, value := range []string{"http://mail.example.com", "https://secret@mail.example.com", "https://mail.example.com/path", "https://mail.example.com:8443"} {
		p.PublicURL = value
		if _, e = p.config(19890); e == nil {
			t.Fatal("unsafe public address accepted")
		}
	}
}
func TestOwnerWizardAuthorizationProofAndDPAPI(t *testing.T) {
	if _, e := portable.Protect([]byte("probe")); e != nil {
		t.Skip(e)
	}
	dir, e := os.MkdirTemp(os.Getenv("MYACCOUNTS_TEST_DATA"), "mail-owner-test-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	if e = os.Mkdir(filepath.Join(dir, "secrets"), 0700); e != nil {
		t.Fatal(e)
	}
	if e = portable.ProtectDir(dir); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	opened := make(chan string, 1)
	done := make(chan error, 1)
	deliveries := make(chan mailrelay.Delivery, 1)
	go func() {
		done <- wizard(ctx, dir, 0, func(link string) { opened <- link }, func(_ context.Context, _ mailrelay.SMTPConfig, m mailrelay.Delivery) error {
			deliveries <- m
			return nil
		})
	}()
	var link string
	select {
	case link = <-opened:
	case <-ctx.Done():
		t.Fatal("owner wizard did not open")
	}
	u, _ := url.Parse(link)
	origin := "http://" + u.Host
	post := func(path string, p ownerRequest, token, originHeader, host string) int {
		body, _ := json.Marshal(p)
		r, _ := http.NewRequest("POST", origin+path, bytes.NewReader(body))
		r.Header.Set("Origin", originHeader)
		r.Header.Set("X-Setup-Token", token)
		r.Header.Set("Content-Type", "application/json")
		if host != "" {
			r.Host = host
		}
		response, e := http.DefaultClient.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		return response.StatusCode
	}
	p := ownerRequest{Email: "sender@example.com", Provider: "gmail", Password: "synthetic-owner-app-password", Mode: "manual", PublicURL: "https://mail.example.com"}
	for _, auth := range [][3]string{{"wrong", origin, ""}, {u.Fragment, "https://evil.example", ""}, {u.Fragment, origin, "rebind.example"}} {
		if post("/owner/test", p, auth[0], auth[1], auth[2]) != 403 {
			t.Fatal("owner setup authorization bypassed")
		}
	}
	if len(deliveries) != 0 {
		t.Fatal("unauthorized owner setup sent email")
	}
	if post("/owner/test", p, u.Fragment, origin, "") != 200 {
		t.Fatal("owner test failed")
	}
	delivery := <-deliveries
	code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(delivery.Text)
	if code == "" {
		t.Fatal("owner proof missing")
	}
	if _, e = os.Stat(filepath.Join(dir, "service.dpapi")); !os.IsNotExist(e) {
		t.Fatal("unverified owner configuration saved")
	}
	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}
	if post("/owner/finish", ownerRequest{Code: wrong}, u.Fragment, origin, "") != 400 {
		t.Fatal("wrong owner code accepted")
	}
	// Altered fields cannot replace the exact sending configuration that was tested.
	if post("/owner/finish", ownerRequest{Code: code, Password: "attacker-new-password"}, u.Fragment, origin, "") != 200 {
		t.Fatal("verified owner configuration failed")
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	c, e := Load(dir)
	if e != nil || c.SMTP.Password != p.Password || c.SMTP.Username != p.Email {
		t.Fatal("owner configuration changed after proof")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "service.dpapi"))
	if strings.Contains(string(b), p.Password) {
		t.Fatal("unprotected owner password")
	}
	if _, e = os.Stat(filepath.Join(dir, "secrets", "owner-setup-url.dpapi")); !os.IsNotExist(e) {
		t.Fatal("owner setup credential retained after completion")
	}
}
