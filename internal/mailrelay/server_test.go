package mailrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type fixture struct {
	s        *Server
	c        *Client
	host     *httptest.Server
	messages []Delivery
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{}
	s, e := Open(filepath.Join(t.TempDir(), "mail.db"), func(_ context.Context, m Delivery) error { f.messages = append(f.messages, m); return nil })
	if e != nil {
		t.Fatal(e)
	}
	f.s = s
	f.host = httptest.NewTLSServer(s.Handler())
	t.Cleanup(func() { f.host.Close(); s.Close() })
	f.c, e = NewClient(f.host.URL)
	if e != nil {
		t.Fatal(e)
	}
	f.c.http.Transport = f.host.Client().Transport
	return f
}
func (f *fixture) connect(t *testing.T, email string) Credential {
	t.Helper()
	p, e := f.c.Begin(context.Background(), email, "https://login.example.com")
	if e != nil {
		t.Fatal(e)
	}
	code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(f.messages[len(f.messages)-1].Text)
	if code == "" {
		t.Fatal("connection code missing")
	}
	if e = f.c.Confirm(context.Background(), p, code); e != nil {
		t.Fatal(e)
	}
	return p
}
func TestVerifiedEnrollmentAndRestrictedDelivery(t *testing.T) {
	f := newFixture(t)
	p, e := f.c.Begin(context.Background(), "admin@example.com", "https://login.example.com")
	if e != nil {
		t.Fatal(e)
	}
	if e = f.c.Send(context.Background(), p, Message{RandomSecret(), "person@example.com", "verification", "123456"}); e == nil {
		t.Fatal("unverified installation sent email")
	}
	code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(f.messages[0].Text)
	wrong := p
	wrong.Secret = RandomSecret()
	if e = f.c.Confirm(context.Background(), wrong, code); e == nil {
		t.Fatal("enrollment credential crossed installation")
	}
	badCode := "000000"
	if code == badCode {
		badCode = "000001"
	}
	if e = f.c.Confirm(context.Background(), p, badCode); e == nil {
		t.Fatal("wrong confirmation code accepted")
	}
	if e = f.c.Confirm(context.Background(), p, code); e != nil {
		t.Fatal(e)
	}
	if e = f.c.Status(context.Background(), p); e != nil {
		t.Fatal(e)
	}
	for _, m := range []Message{{RandomSecret(), "person@example.com", "invitation", "123456"}, {RandomSecret(), "person@example.com", "password-reset", "<script>"}, {RandomSecret(), "person@example.com\r\nBcc: attacker@example.com", "verification", "123456"}} {
		if e = f.c.Send(context.Background(), p, m); e == nil {
			t.Fatal("unsupported message accepted")
		}
	}
	m := Message{RandomSecret(), "person@example.com", "password-reset", "123456"}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- f.c.Send(context.Background(), p, m) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if len(f.messages) != 2 {
		t.Fatalf("concurrent retry delivered %d messages", len(f.messages))
	}
	changed := m
	changed.Purpose = "verification"
	if e = f.c.Send(context.Background(), p, changed); e == nil {
		t.Fatal("same message ID accepted a different payload")
	}
	m.ID = RandomSecret()
	if e = f.c.Send(context.Background(), p, m); e != nil {
		t.Fatal("new request with same numerical code was incorrectly deduplicated")
	}
	if len(f.messages) != 3 {
		t.Fatal("distinct code request was not delivered")
	}
	if strings.Contains(f.messages[1].Text, "https:") || strings.Contains(f.messages[1].Text, p.Secret) {
		t.Fatal("untrusted links or credentials in delivered mail")
	}
	var stored string
	if e = f.s.db.QueryRow("SELECT token_hash FROM installations WHERE id=?", p.ID).Scan(&stored); e != nil || stored == p.Secret || stored != digest(p.Secret) {
		t.Fatal("credential not stored as a hash")
	}
	if e = f.s.Revoke(p.ID); e != nil {
		t.Fatal(e)
	}
	if e = f.c.Status(context.Background(), p); e == nil {
		t.Fatal("revoked credential accepted")
	}
}
func TestConfirmationExpiryAndAttemptLimit(t *testing.T) {
	f := newFixture(t)
	for _, expiry := range []bool{false, true} {
		p, e := f.c.Begin(context.Background(), "admin@example.com", "http://localhost:18888")
		if e != nil {
			t.Fatal(e)
		}
		code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(f.messages[len(f.messages)-1].Text)
		if expiry {
			f.s.mu.Lock()
			f.s.now = func() time.Time { return time.Now().Add(11 * time.Minute) }
			f.s.mu.Unlock()
		} else {
			wrong := "000000"
			if code == wrong {
				wrong = "000001"
			}
			for i := 0; i < 5; i++ {
				if e = f.c.Confirm(context.Background(), p, wrong); e == nil {
					t.Fatal("wrong code accepted")
				}
			}
		}
		if e = f.c.Confirm(context.Background(), p, code); e == nil {
			t.Fatal("expired or exhausted enrollment accepted")
		}
	}
}
func TestLimitsSurviveRestartAndNeverDeliverAfterLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mail.db")
	delivered := 0
	s, e := Open(path, func(context.Context, Delivery) error { delivered++; return nil })
	if e != nil {
		t.Fatal(e)
	}
	if !s.reserve(map[string]int{"example": 1}) || s.reserve(map[string]int{"example": 1}) {
		t.Fatal("limit not enforced")
	}
	s.Close()
	s, e = Open(path, func(context.Context, Delivery) error { delivered++; return nil })
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if s.reserve(map[string]int{"example": 1}) {
		t.Fatal("restart reset rate limit")
	}
	f := newFixture(t)
	p := f.connect(t, "admin@example.com")
	for i := 0; i < 5; i++ {
		m := Message{RandomSecret(), "person@example.com", "verification", strings.Repeat(string(rune('1'+i)), 6)}
		if e = f.c.Send(context.Background(), p, m); e != nil {
			t.Fatal(e)
		}
	}
	if e = f.c.Send(context.Background(), p, Message{RandomSecret(), "person@example.com", "verification", "999999"}); e == nil {
		t.Fatal("recipient quota bypassed")
	}
	if len(f.messages) != 6 {
		t.Fatal("rate-limited message delivered")
	}
}
func TestProtocolBoundaryAndSafeErrors(t *testing.T) {
	f := newFixture(t)
	p := f.connect(t, "admin@example.com")
	for _, body := range []string{`{"recipient":"person@example.com","purpose":"verification","code":"123456","subject":"arbitrary"}`, `{} {}`, strings.Repeat("x", 5000)} {
		r, _ := http.NewRequest("POST", f.host.URL+"/v1/send", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+p.ID+"."+p.Secret)
		response, e := f.c.http.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatalf("invalid protocol returned %d", response.StatusCode)
		}
	}
	f.s.deliver = func(context.Context, Delivery) error { return errors.New("private-smtp-password") }
	e := f.c.Send(context.Background(), p, Message{RandomSecret(), "person@example.com", "verification", "123456"})
	if e == nil || strings.Contains(e.Error(), "private-smtp-password") {
		t.Fatal("unsafe provider error")
	}
	if _, e = NewClient("http://mail.example.com"); e == nil {
		t.Fatal("plaintext relay permitted")
	}
	if _, e = NewClient("https://user:password@mail.example.com"); e == nil {
		t.Fatal("URL credentials permitted")
	}
}
func TestClientNeverFollowsCredentialRedirect(t *testing.T) {
	received := 0
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received++; w.WriteHeader(200) }))
	defer target.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer source.Close()
	c, _ := NewClient(source.URL)
	c.http.Transport = source.Client().Transport
	if e := c.Status(context.Background(), Credential{ID: RandomSecret(), Secret: RandomSecret()}); e == nil {
		t.Fatal("redirect accepted")
	}
	if received != 0 {
		t.Fatal("credential forwarded on redirect")
	}
}
func TestUnconfiguredServiceRefusesEnrollment(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "mail.db"), nil)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r := httptest.NewRequest("POST", "/v1/enroll", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 503 {
		t.Fatal("unconfigured service accepted enrollment")
	}
}
func TestEnrollmentHasNoPlaintextTokenOnWire(t *testing.T) {
	var p Enrollment
	host := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&p)
		_, _ = io.WriteString(w, `{"id":"`+RandomSecret()+`"}`)
	}))
	defer host.Close()
	c, _ := NewClient(host.URL)
	c.http.Transport = host.Client().Transport
	credential, e := c.Begin(context.Background(), "admin@example.com", "https://login.example.com")
	if e != nil {
		t.Fatal(e)
	}
	if p.TokenHash != digest(credential.Secret) || p.TokenHash == credential.Secret {
		t.Fatal("raw token sent before ownership proof")
	}
}
func TestDeliveryPreservesMailboxLocalPart(t *testing.T) {
	f := newFixture(t)
	p := f.connect(t, "CaseSensitive@example.com")
	if f.messages[0].Recipient != "CaseSensitive@example.com" {
		t.Fatal("enrollment delivered to a different mailbox")
	}
	if e := f.c.Send(context.Background(), p, Message{RandomSecret(), "AnotherCase@example.com", "verification", "123456"}); e != nil {
		t.Fatal(e)
	}
	if f.messages[1].Recipient != "AnotherCase@example.com" {
		t.Fatal("verification delivered to a different mailbox")
	}
}
