package mailrelay

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Delivery struct{ Recipient, Subject, Text string }
type Deliver func(context.Context, Delivery) error

type Server struct {
	db              *sql.DB
	deliver         Deliver
	mu              sync.Mutex
	TrustCloudflare bool
	now             func() time.Time
}

func Open(path string, deliver Deliver) (*Server, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS installations (id TEXT PRIMARY KEY, email TEXT NOT NULL, issuer TEXT NOT NULL, token_hash TEXT NOT NULL, code_hash TEXT NOT NULL, expires INTEGER NOT NULL, attempts INTEGER NOT NULL DEFAULT 0, active INTEGER NOT NULL DEFAULT 0, revoked INTEGER NOT NULL DEFAULT 0, created INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS limits (key TEXT NOT NULL, bucket INTEGER NOT NULL, count INTEGER NOT NULL, PRIMARY KEY(key,bucket));
CREATE TABLE IF NOT EXISTS delivered (id TEXT PRIMARY KEY, request_hash TEXT NOT NULL, created INTEGER NOT NULL);`)
	if e != nil {
		db.Close()
		return nil, e
	}
	return &Server{db: db, deliver: deliver, now: time.Now}, nil
}
func (s *Server) Close() error { return s.db.Close() }
func (s *Server) Revoke(id string) error {
	if !hexID.MatchString(id) {
		return errors.New("invalid installation ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, e := s.db.Exec("UPDATE installations SET revoked=1 WHERE id=?", id)
	return e
}
func validEmail(v string) bool {
	a, e := mail.ParseAddress(v)
	return e == nil && a.Address == v && len(v) <= 254 && !strings.ContainsAny(v, "\r\n")
}
func validIssuer(v string) bool {
	u, e := url.Parse(v)
	if e != nil || len(v) > 300 || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	if u.Scheme == "http" && u.Hostname() == "localhost" && u.Port() != "" {
		return true
	}
	return u.Scheme == "https" && u.Hostname() != "" && !strings.ContainsAny(u.Host, "\r\n<>\" ")
}
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "JSON required", 400)
		return false
	}
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		http.Error(w, "invalid request", 400)
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		http.Error(w, "invalid request", 400)
		return false
	}
	return true
}
func reply(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if s.deliver == nil {
			http.Error(w, "email delivery not configured", 503)
			return
		}
		reply(w, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/enroll", s.enroll)
	mux.HandleFunc("POST /v1/confirm", s.confirm)
	mux.HandleFunc("POST /v1/status", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if _, _, ok := s.authorize(w, r); !ok {
			return
		}
		reply(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /v1/send", s.send)
	mux.HandleFunc("POST /v1/revoke", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		id, _, ok := s.authorize(w, r)
		if !ok {
			return
		}
		if _, e := s.db.Exec("UPDATE installations SET revoked=1 WHERE id=?", id); e != nil {
			http.Error(w, "revocation unavailable", 500)
			return
		}
		reply(w, map[string]bool{"ok": true})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if s.deliver == nil && r.URL.Path != "/healthz" {
			http.Error(w, "email delivery not configured", 503)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// Limits are persistent, atomic across concurrent requests, and charged before
// attempting delivery. A failing provider must not create an unlimited retry loop.
func (s *Server) reserve(keys map[string]int) bool {
	now := s.now().Unix()
	bucket := now / 3600
	tx, e := s.db.Begin()
	if e != nil {
		return false
	}
	defer tx.Rollback()
	if _, e = tx.Exec("DELETE FROM limits WHERE bucket < ?", bucket-24); e != nil {
		return false
	}
	if _, e = tx.Exec("DELETE FROM delivered WHERE created < ?", now-86400); e != nil {
		return false
	}
	if _, e = tx.Exec("DELETE FROM installations WHERE active=0 AND expires < ?", now-86400); e != nil {
		return false
	}
	for key, max := range keys {
		key = digest(key)
		var count int
		e = tx.QueryRow("SELECT count FROM limits WHERE key=? AND bucket=?", key, bucket).Scan(&count)
		if e != nil && e != sql.ErrNoRows {
			return false
		}
		if count >= max {
			return false
		}
		if _, e = tx.Exec("INSERT INTO limits(key,bucket,count) VALUES(?,?,1) ON CONFLICT(key,bucket) DO UPDATE SET count=count+1", key, bucket); e != nil {
			return false
		}
	}
	return tx.Commit() == nil
}
func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	var p Enrollment
	if !readJSON(w, r, &p) {
		return
	}
	// Preserve the local part: some email servers treat it as case sensitive.
	// Case folding is used only for conservative quota keys, never delivery.
	p.Email = strings.TrimSpace(p.Email)
	if !validEmail(p.Email) || !validIssuer(p.Issuer) || !hexID.MatchString(p.TokenHash) {
		http.Error(w, "invalid enrollment", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	peer, _, _ := net.SplitHostPort(r.RemoteAddr)
	if s.TrustCloudflare && net.ParseIP(peer) != nil && net.ParseIP(peer).IsLoopback() {
		if ip := net.ParseIP(r.Header.Get("CF-Connecting-IP")); ip != nil {
			peer = ip.String()
		}
	}
	var count int
	if e := s.db.QueryRow("SELECT COUNT(*) FROM installations WHERE active=1 AND revoked=0").Scan(&count); e != nil || count >= 1000 {
		http.Error(w, "installation capacity reached", 503)
		return
	}
	if !s.reserve(map[string]int{"all": 300, "enroll-ip:" + peer: 10, "enroll-email:" + strings.ToLower(p.Email): 3}) {
		http.Error(w, "sending limit reached", 429)
		return
	}
	id := RandomSecret()
	n, e := rand.Int(rand.Reader, big.NewInt(1000000))
	if e != nil {
		http.Error(w, "random source unavailable", 500)
		return
	}
	code := fmt.Sprintf("%06d", n.Int64())
	now := s.now().Unix()
	_, e = s.db.Exec("INSERT INTO installations(id,email,issuer,token_hash,code_hash,expires,created) VALUES(?,?,?,?,?,?,?)", id, p.Email, p.Issuer, p.TokenHash, digest(id+":"+code), now+600, now)
	if e != nil {
		http.Error(w, "enrollment unavailable", 500)
		return
	}
	e = s.deliver(r.Context(), Delivery{p.Email, "Connect MyAccounts email", "Your MyAccounts email connection code is " + code + ".\n\nEnter this code in the setup screen where you requested it. It expires in 10 minutes.\nIf you did not request this, ignore this email. Never give this code to another person."})
	if e != nil {
		_, _ = s.db.Exec("DELETE FROM installations WHERE id=?", id)
		http.Error(w, "delivery failed", 502)
		return
	}
	reply(w, map[string]string{"id": id})
}
func (s *Server) confirm(w http.ResponseWriter, r *http.Request) {
	var p Confirmation
	if !readJSON(w, r, &p) {
		return
	}
	if !hexID.MatchString(p.ID) || !hexID.MatchString(p.Secret) || !codePattern.MatchString(p.Code) {
		http.Error(w, "invalid confirmation", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result, e := s.db.Exec(`UPDATE installations SET attempts=attempts+1 WHERE id=? AND token_hash=? AND active=0 AND revoked=0 AND expires>? AND attempts<5`, p.ID, digest(p.Secret), s.now().Unix())
	if e != nil {
		http.Error(w, "confirmation unavailable", 500)
		return
	}
	rows, _ := result.RowsAffected()
	if rows != 1 {
		http.Error(w, "invalid or expired confirmation", 400)
		return
	}
	result, e = s.db.Exec(`UPDATE installations SET active=1,code_hash='' WHERE id=? AND token_hash=? AND code_hash=? AND active=0 AND revoked=0 AND expires>?`, p.ID, digest(p.Secret), digest(p.ID+":"+p.Code), s.now().Unix())
	if e != nil {
		http.Error(w, "confirmation unavailable", 500)
		return
	}
	rows, _ = result.RowsAffected()
	if rows != 1 {
		http.Error(w, "incorrect code", 400)
		return
	}
	reply(w, map[string]bool{"ok": true})
}
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || len(parts) != 2 || !hexID.MatchString(parts[0]) || !hexID.MatchString(parts[1]) {
		http.Error(w, "not authorized", 401)
		return "", "", false
	}
	var email string
	e := s.db.QueryRow("SELECT email FROM installations WHERE id=? AND token_hash=? AND active=1 AND revoked=0", parts[0], digest(parts[1])).Scan(&email)
	if e != nil {
		http.Error(w, "not authorized", 401)
		return "", "", false
	}
	return parts[0], email, true
}
func (s *Server) send(w http.ResponseWriter, r *http.Request) {
	var p Message
	if !readJSON(w, r, &p) {
		return
	}
	p.Recipient = strings.TrimSpace(p.Recipient)
	if !validMessage(p) {
		http.Error(w, "unsupported message", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id, admin, ok := s.authorize(w, r)
	if !ok {
		return
	}
	messageID := digest(id + ":" + p.ID)
	requestHash := digest(p.Recipient + ":" + p.Purpose + ":" + p.Code)
	var prior string
	if e := s.db.QueryRow("SELECT request_hash FROM delivered WHERE id=? AND created>=?", messageID, s.now().Unix()-86400).Scan(&prior); e != nil && e != sql.ErrNoRows {
		http.Error(w, "delivery unavailable", 500)
		return
	}
	if prior != "" {
		if prior != requestHash {
			http.Error(w, "message ID already used", 409)
			return
		}
		reply(w, map[string]bool{"ok": true})
		return
	}
	if !s.reserve(map[string]int{"all": 300, "installation:" + id: 20, "operator-email:" + strings.ToLower(admin): 40, "recipient:" + strings.ToLower(p.Recipient): 5}) {
		http.Error(w, "sending limit reached", 429)
		return
	}
	label := "verification"
	if p.Purpose == "password-reset" {
		label = "password reset"
	}
	e := s.deliver(r.Context(), Delivery{p.Recipient, "MyAccounts " + label + " code", "Your MyAccounts " + label + " code is " + p.Code + ".\n\nEnter it in the MyAccounts screen where you requested it. It expires in 5 minutes.\nIf you did not request this, ignore this email. Never give this code to another person."})
	if e != nil {
		http.Error(w, "delivery failed", 502)
		return
	}
	if _, e = s.db.Exec("INSERT INTO delivered(id,request_hash,created) VALUES(?,?,?)", messageID, requestHash, s.now().Unix()); e != nil {
		http.Error(w, "delivery status unavailable", 500)
		return
	}
	reply(w, map[string]bool{"ok": true})
}
