package portable

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

//go:embed setup.html
var setupFiles embed.FS

type SetupRequest struct {
	InstallService string `json:"installService"`
	RootDomain     string `json:"rootDomain"`
	Subdomain      string `json:"subdomain"`
	SMTPPort       int    `json:"smtpPort"`
	SMTPHost       string `json:"smtpHost"`
	SMTPUsername   string `json:"smtpUsername"`
	SMTPPassword   string `json:"smtpPassword"`
	SMTPFrom       string `json:"smtpFrom"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	Confirm        string `json:"confirm"`
	Issuer         string `json:"issuer"`
	Mode           string `json:"mode"`
	TunnelToken    string `json:"tunnelToken"`
	APIToken       string `json:"apiToken"`
	AccountID      string `json:"accountId"`
	ZoneID         string `json:"zoneId"`
	Authorize      string `json:"authorize"`
}

func RandomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Setup(ctx context.Context, dir string, port int, open func(string)) error {
	token := RandomToken()
	done := make(chan struct{})
	var mu sync.Mutex
	complete := false
	origin := fmt.Sprintf("http://localhost:%d", port)
	// Keep unattended setup access inside the already protected data boundary.
	link, e := Protect([]byte(origin + "/#" + token))
	if e != nil {
		return e
	}
	linkPath := filepath.Join(dir, "secrets", "setup-url.dpapi")
	if e = AtomicWrite(linkPath, link); e != nil {
		return e
	}
	defer os.Remove(linkPath)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /setup-info", func(w http.ResponseWriter, r *http.Request) {
		var defaults struct {
			Domain string `json:"domain"`
		}
		if b, e := os.ReadFile(filepath.Join(dir, "setup-defaults.json")); e == nil {
			_ = json.Unmarshal(b, &defaults)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"os": runtime.GOOS, "architecture": runtime.GOARCH, "dataPath": dir, "protectedStorage": true, "suggestedDomain": defaults.Domain, "connectorSeparate": true})
	})
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		b, _ := setupFiles.ReadFile("setup.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	})
	mux.HandleFunc("POST /setup", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != origin || r.Header.Get("X-Setup-Token") != token {
			http.Error(w, "invalid setup authorization", 403)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if complete {
			http.Error(w, "setup already completed", 409)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		var p SetupRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if e := dec.Decode(&p); e != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		if len(p.Password) < 12 || len(p.Password) > 128 || p.Password != p.Confirm {
			http.Error(w, "passwords must match and contain 12 to 128 characters", 400)
			return
		}
		if p.Mode != "local" && p.Mode != "manual" && p.Mode != "token" && p.Mode != "api" {
			http.Error(w, "invalid connection method", 400)
			return
		}
		if p.Mode != "local" && p.RootDomain != "" {
			canonical, e := DomainIssuer(p.RootDomain, p.Subdomain)
			if e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			if p.Issuer != "" && p.Issuer != canonical {
				http.Error(w, "domain and issuer disagree", 400)
				return
			}
			p.Issuer = canonical
		}
		if p.Mode == "local" {
			p.Issuer = origin
		}
		if p.Mode == "token" && p.TunnelToken == "" {
			http.Error(w, "tunnel token is required", 400)
			return
		}
		c := Config{Version: 1, Issuer: p.Issuer, Port: port, Email: p.Email, Name: p.Name, TunnelMode: p.Mode, InstallServiceRequested: p.InstallService == "on"}
		if e := Validate(c); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		if _, e := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(e) {
			http.Error(w, "configuration already exists", 409)
			return
		}
		if p.Mode == "api" {
			u, _ := url.Parse(c.Issuer)
			provisionCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			result, e := (Cloudflare{Token: p.APIToken}).Provision(provisionCtx, ProvisionRequest{AccountID: p.AccountID, ZoneID: p.ZoneID, Hostname: u.Hostname(), Port: port, Authorized: p.Authorize == "on"})
			if e != nil {
				http.Error(w, e.Error(), 400)
				return
			}
			p.TunnelToken = result.Token
			c.TunnelMode = "token"
		}
		// The scoped API token is deliberately not persisted, even in the protected bootstrap.
		operationalToken := p.TunnelToken
		p.APIToken = ""
		p.TunnelToken = ""
		p.Confirm = ""
		b, _ := json.Marshal(p)
		secret, e := Protect(b)
		if e == nil {
			e = AtomicWrite(filepath.Join(dir, "secrets", "bootstrap.dpapi"), secret)
		}
		if e == nil && operationalToken != "" {
			secret, e = Protect([]byte(operationalToken))
			if e == nil {
				e = AtomicWrite(filepath.Join(dir, "secrets", "tunnel.dpapi"), secret)
			}
		}
		if e == nil {
			e = Save(dir, c)
		}
		if e != nil {
			http.Error(w, "unable to save protected setup", 500)
			return
		}
		complete = true
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
		close(done)
	})
	mux.HandleFunc("POST /test-email", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != origin || r.Header.Get("X-Setup-Token") != token {
			http.Error(w, "invalid setup authorization", 403)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if complete {
			http.Error(w, "setup is complete", 409)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 16384)
		var p SetupRequest
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if e := dec.Decode(&p); e != nil {
			http.Error(w, "invalid email settings", 400)
			return
		}
		if e := TestEmail(p); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
	})

	secured := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		if r.Host != fmt.Sprintf("localhost:%d", port) {
			http.Error(w, "localhost setup only", 403)
			return
		}
		mux.ServeHTTP(w, r)
	})
	listener, e := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
	if e != nil {
		return e
	}
	srv := &http.Server{Handler: secured, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 30 * time.Second}
	errs := make(chan error, 1)
	go func() { errs <- srv.Serve(listener) }()
	if open != nil {
		open(origin + "/#" + token)
	}
	var result error
	select {
	case <-done:
	case <-ctx.Done():
		result = ctx.Err()
	case e = <-errs:
		if !errors.Is(e, http.ErrServerClosed) {
			result = e
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e = srv.Shutdown(shutdown); result == nil {
		result = e
	}
	return result
}
