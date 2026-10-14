// A disposable TLS mail sink for the native identity smoke test. Never deploy it
// as the shared service. No message leaves this loopback process.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/casdoor/casdoor/internal/mailrelay"
	"github.com/casdoor/casdoor/internal/portable"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	dir := flag.String("data", "", "fresh protected fixture directory")
	flag.Parse()
	if *dir == "" {
		return fmt.Errorf("fixture requires explicit data directory")
	}
	if _, e := os.Stat(*dir); !os.IsNotExist(e) {
		return fmt.Errorf("fixture requires a fresh directory")
	}
	if e := os.MkdirAll(filepath.Join(*dir, "secrets"), 0700); e != nil {
		return e
	}
	if e := portable.ProtectDir(*dir); e != nil {
		return e
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		return e
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "MyAccounts disposable fixture"}, DNSNames: []string{"localhost"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		return e
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	cert, e := tls.X509KeyPair(certPEM, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	if e != nil {
		return e
	}
	caPath := filepath.Join(*dir, "fixture-ca.pem")
	if e = portable.AtomicWrite(caPath, certPEM); e != nil {
		return e
	}
	var mu sync.Mutex
	messages := []mailrelay.Delivery{}
	relay, e := mailrelay.Open(filepath.Join(*dir, "relay.db"), func(_ context.Context, m mailrelay.Delivery) error {
		mu.Lock()
		defer mu.Unlock()
		messages = append(messages, m)
		return nil
	})
	if e != nil {
		return e
	}
	defer relay.Close()
	control := mailrelay.RandomSecret()
	relayHandler := relay.Handler()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fixture/messages" {
			if r.Header.Get("Authorization") != "Bearer "+control {
				http.Error(w, "fixture authorization required", 403)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(messages)
			return
		}
		relayHandler.ServeHTTP(w, r)
	})
	listener, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return e
	}
	endpoint := fmt.Sprintf("https://localhost:%d", listener.Addr().(*net.TCPAddr).Port)
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		_ = srv.Serve(tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}))
	}()
	defer srv.Close()
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certPEM)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}, Timeout: 10 * time.Second}
	post := func(path string, body, result any) error {
		b, _ := json.Marshal(body)
		r, e := client.Post(endpoint+path, "application/json", bytes.NewReader(b))
		if e != nil {
			return e
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			return fmt.Errorf("fixture enrollment failed (%d)", r.StatusCode)
		}
		if result != nil {
			return json.NewDecoder(r.Body).Decode(result)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		return nil
	}
	credential := mailrelay.Credential{Endpoint: endpoint, Email: "admin@example.com", Issuer: "http://localhost:18787", Secret: mailrelay.RandomSecret()}
	hash := sha256.Sum256([]byte(credential.Secret))
	var enrolled struct {
		ID string `json:"id"`
	}
	if e = post("/v1/enroll", mailrelay.Enrollment{Email: credential.Email, Issuer: credential.Issuer, TokenHash: hex.EncodeToString(hash[:])}, &enrolled); e != nil {
		return e
	}
	credential.ID = enrolled.ID
	mu.Lock()
	code := regexp.MustCompile(`\b[0-9]{6}\b`).FindString(messages[0].Text)
	mu.Unlock()
	if e = post("/v1/confirm", mailrelay.Confirmation{ID: credential.ID, Secret: credential.Secret, Code: code}, nil); e != nil {
		return e
	}
	if e = portable.SaveSharedEmail(*dir, credential); e != nil {
		return e
	}
	info := map[string]string{"endpoint": endpoint, "ca": caPath, "control": control, "credential": filepath.Join(*dir, "secrets", "email-relay.dpapi")}
	b, _ := json.Marshal(info)
	if e = portable.AtomicWrite(filepath.Join(*dir, "fixture.json"), b); e != nil {
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	<-ctx.Done()
	return nil
}
