// Package mailservice hosts the shared service and its one-time operator wizard.
package mailservice

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/casdoor/casdoor/internal/mailrelay"
	"github.com/casdoor/casdoor/internal/portable"
)

//go:embed setup.html
var pages embed.FS

// ErrSkipped means the owner chose to leave optional email delivery unconfigured.
var ErrSkipped = errors.New("email setup skipped")

type Config struct {
	PublicURL string               `json:"publicUrl"`
	Port      int                  `json:"port"`
	SMTP      mailrelay.SMTPConfig `json:"smtp"`
	Tunnel    bool                 `json:"tunnel"`
}
type ownerRequest struct {
	Email       string `json:"email"`
	Provider    string `json:"provider"`
	Password    string `json:"password"`
	Host        string `json:"host"`
	Username    string `json:"username"`
	From        string `json:"from"`
	PublicURL   string `json:"publicUrl"`
	Mode        string `json:"mode"`
	APIToken    string `json:"apiToken"`
	AccountID   string `json:"accountId"`
	ZoneID      string `json:"zoneId"`
	Authorize   string `json:"authorize"`
	TunnelToken string `json:"tunnelToken"`
	Code        string `json:"code"`
}

func (p ownerRequest) config(port int) (Config, error) {
	c := Config{PublicURL: strings.TrimSuffix(p.PublicURL, "/"), Port: port, SMTP: mailrelay.SMTPConfig{Host: p.Host, Port: 465, Username: p.Username, Password: p.Password, From: p.From}, Tunnel: p.Mode != "manual"}
	if _, e := mailrelay.NewClient(c.PublicURL); e != nil {
		return c, e
	}
	u, _ := url.Parse(c.PublicURL)
	if u.Port() != "" {
		return c, errors.New("use a public HTTPS domain without a port")
	}
	canonical, e := portable.DomainIssuer(u.Hostname(), "")
	if e != nil {
		return c, errors.New("enter the public email service domain")
	}
	c.PublicURL = canonical
	if p.Mode != "manual" && p.Mode != "api" && p.Mode != "token" {
		return c, errors.New("choose a connection method")
	}
	if p.Mode == "token" && p.TunnelToken == "" {
		return c, errors.New("enter the existing dedicated tunnel token")
	}
	if p.Provider == "gmail" {
		c.SMTP.Host = "smtp.gmail.com"
		c.SMTP.Username = p.Email
		c.SMTP.From = p.Email
	} else if p.Provider != "custom" {
		return c, errors.New("choose an email provider")
	}
	return c, c.SMTP.Validate()
}
func Load(dir string) (Config, error) {
	var c Config
	b, e := os.ReadFile(filepath.Join(dir, "service.dpapi"))
	if e != nil {
		return c, e
	}
	b, e = portable.Unprotect(b)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e == nil {
		e = c.SMTP.Validate()
	}
	if e == nil {
		_, e = mailrelay.NewClient(c.PublicURL)
	}
	if e == nil && (c.Port < 1024 || c.Port > 65535) {
		e = errors.New("invalid service port")
	}
	return c, e
}

// Wizard never listens publicly. The service's Cloudflare tunnel is created only
// when the operator explicitly selects and authorizes it in this wizard.
func Wizard(ctx context.Context, dir string, port int, open func(string)) error {
	return wizard(ctx, dir, port, open, func(ctx context.Context, p mailrelay.SMTPConfig, m mailrelay.Delivery) error {
		return p.Deliver(ctx, m)
	})
}
func wizard(ctx context.Context, dir string, port int, open func(string), send func(context.Context, mailrelay.SMTPConfig, mailrelay.Delivery) error) error {
	listener, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		return e
	}
	port = listener.Addr().(*net.TCPAddr).Port
	origin := fmt.Sprintf("http://localhost:%d", port)
	token := portable.RandomToken()
	var mu sync.Mutex
	var pending *ownerRequest
	var expected [32]byte
	var expires time.Time
	attempts := 0
	complete := false
	skipped := false
	done := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, _ := pages.ReadFile("setup.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("POST /owner/test", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if complete {
			http.Error(w, "setup complete", 409)
			return
		}
		var p ownerRequest
		if !readOwner(w, r, &p) {
			return
		}
		c, e := p.config(port)
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		// Limit owner test requests locally as well; double-clicks cannot spam.
		if pending != nil && time.Until(expires) > 9*time.Minute {
			http.Error(w, "wait one minute before resending", 429)
			return
		}
		n, e := rand.Int(rand.Reader, big.NewInt(1000000))
		if e != nil {
			http.Error(w, "random source unavailable", 500)
			return
		}
		code := fmt.Sprintf("%06d", n.Int64())
		expected = sha256.Sum256([]byte(code))
		pending = nil
		e = send(r.Context(), c.SMTP, mailrelay.Delivery{Recipient: p.Email, Subject: "Verify your MyAccounts email service", Text: "Your MyAccounts service setup code is " + code + ". Enter it in your local service setup wizard within 10 minutes."})
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		pending = &p
		expires = time.Now().Add(10 * time.Minute)
		attempts = 0
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	})
	mux.HandleFunc("POST /owner/skip", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if complete {
			http.Error(w, "setup complete", 409)
			return
		}
		pending = nil
		expected = [32]byte{}
		complete = true
		skipped = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
		close(done)
	})
	mux.HandleFunc("POST /owner/finish", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if complete {
			http.Error(w, "setup complete", 409)
			return
		}
		var p ownerRequest
		if !readOwner(w, r, &p) {
			return
		}
		if pending == nil || time.Now().After(expires) || attempts >= 5 {
			http.Error(w, "send a new test code", 400)
			return
		}
		attempts++
		actual := sha256.Sum256([]byte(p.Code))
		if subtle.ConstantTimeCompare(expected[:], actual[:]) != 1 {
			http.Error(w, "incorrect code", 400)
			return
		}
		// Use only the configuration that actually sent the confirmed test.
		c, e := pending.config(port)
		if e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		if _, e = os.Stat(filepath.Join(dir, "service.dpapi")); !os.IsNotExist(e) {
			http.Error(w, "service already configured; stop and use protected operator configuration", 409)
			return
		}
		tunnelToken := pending.TunnelToken
		if pending.Mode == "api" {
			u, _ := url.Parse(c.PublicURL)
			result, e := (portable.Cloudflare{Token: pending.APIToken}).Provision(r.Context(), portable.ProvisionRequest{AccountID: pending.AccountID, ZoneID: pending.ZoneID, Hostname: u.Hostname(), Port: port, Authorized: pending.Authorize == "on"})
			if e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			tunnelToken = result.Token
		}
		if tunnelToken != "" {
			b, e := portable.Protect([]byte(tunnelToken))
			if e == nil {
				e = portable.AtomicWrite(filepath.Join(dir, "secrets", "tunnel.dpapi"), b)
			}
			if e != nil {
				http.Error(w, "could not save tunnel credential", 500)
				return
			}
		}
		b, e := json.Marshal(c)
		if e == nil {
			b, e = portable.Protect(b)
		}
		if e == nil {
			e = portable.AtomicWrite(filepath.Join(dir, "service.dpapi"), b)
		}
		if e != nil {
			http.Error(w, "could not save protected service configuration", 500)
			return
		}
		pending = nil
		expected = [32]byte{}
		complete = true
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
		close(done)
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")
		if r.Host != strings.TrimPrefix(origin, "http://") || (r.Method == "POST" && (r.Header.Get("Origin") != origin || r.Header.Get("X-Setup-Token") != token)) {
			http.Error(w, "local setup authorization required", 403)
			return
		}
		mux.ServeHTTP(w, r)
	})
	link, e := portable.Protect([]byte(origin + "/#" + token))
	if e != nil {
		listener.Close()
		return e
	}
	path := filepath.Join(dir, "secrets", "owner-setup-url.dpapi")
	if e = portable.AtomicWrite(path, link); e != nil {
		listener.Close()
		return e
	}
	defer os.Remove(path)
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 30 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(listener) }()
	if open != nil {
		open(origin + "/#" + token)
	}
	select {
	case <-done:
		if skipped {
			e = ErrSkipped
		}
	case <-ctx.Done():
		e = ctx.Err()
	case e = <-errCh:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
	return e
}
func readOwner(w http.ResponseWriter, r *http.Request, p *ownerRequest) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(p) != nil {
		http.Error(w, "invalid configuration", 400)
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		http.Error(w, "invalid configuration", 400)
		return false
	}
	return true
}
