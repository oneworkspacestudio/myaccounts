// Package mailrelay connects installations to the shared, restricted MyAccounts
// email service. Provider passwords are never distributed to installations.
package mailrelay

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// DefaultURL can be overridden at build time for another service operator.
var DefaultURL = "https://mail.m8e.co.uk"

type Credential struct {
	Endpoint string `json:"endpoint"`
	ID       string `json:"id"`
	Secret   string `json:"secret"`
	Email    string `json:"email"`
	Issuer   string `json:"issuer"`
}

func (p Credential) Validate() error {
	if _, e := NewClient(p.Endpoint); e != nil {
		return e
	}
	if !hexID.MatchString(p.ID) || !hexID.MatchString(p.Secret) || !validEmail(p.Email) || !validIssuer(p.Issuer) {
		return errors.New("invalid shared email credential")
	}
	return nil
}

type Enrollment struct {
	Email     string `json:"email"`
	Issuer    string `json:"issuer"`
	TokenHash string `json:"tokenHash"`
}
type Confirmation struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
	Code   string `json:"code"`
}
type Message struct {
	ID        string `json:"id"`
	Recipient string `json:"recipient"`
	Purpose   string `json:"purpose"`
	Code      string `json:"code"`
}
type Client struct {
	endpoint string
	http     *http.Client
}

func NewClient(endpoint string) (*Client, error) {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("shared email service must use an HTTPS origin")
	}
	return &Client{endpoint: strings.TrimSuffix(endpoint, "/"), http: &http.Client{Timeout: 35 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func RandomSecret() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

func (c *Client) request(ctx context.Context, path, bearer string, payload, result any) error {
	b, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	r, e := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	r.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, e := c.http.Do(r)
	if e != nil {
		return errors.New("shared email service could not be reached; try again or finish without email")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Never relay server bodies, which could contain upstream credentials.
		switch response.StatusCode {
		case 401, 403:
			return errors.New("email connection is not authorized; reconnect email")
		case 409:
			return errors.New("this email connection was already confirmed")
		case 429:
			return errors.New("email sending limit reached; try again later")
		case 503:
			return errors.New("shared email delivery is not configured yet; you can finish without email")
		case 400:
			return errors.New("email details or verification code are invalid or expired")
		default:
			return fmt.Errorf("shared email service failed (%d); try again later", response.StatusCode)
		}
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(result)
	}
	return nil
}

func (c *Client) Begin(ctx context.Context, email, issuer string) (Credential, error) {
	p := Credential{Endpoint: c.endpoint, Secret: RandomSecret(), Email: email, Issuer: issuer}
	var result struct {
		ID string `json:"id"`
	}
	e := c.request(ctx, "/v1/enroll", "", Enrollment{Email: email, Issuer: issuer, TokenHash: digest(p.Secret)}, &result)
	p.ID = result.ID
	if e == nil && !hexID.MatchString(p.ID) {
		e = errors.New("invalid email enrollment response")
	}
	return p, e
}
func (c *Client) Confirm(ctx context.Context, p Credential, code string) error {
	e := c.request(ctx, "/v1/confirm", "", Confirmation{ID: p.ID, Secret: p.Secret, Code: code}, nil)
	if e != nil {
		// If the confirmation response was lost, the installation still has its
		// own secret and can prove that its enrollment completed.
		if c.Status(ctx, p) == nil {
			return nil
		}
	}
	return e
}
func (c *Client) Status(ctx context.Context, p Credential) error {
	return c.request(ctx, "/v1/status", p.ID+"."+p.Secret, struct{}{}, nil)
}
func (c *Client) Revoke(ctx context.Context, p Credential) error {
	return c.request(ctx, "/v1/revoke", p.ID+"."+p.Secret, struct{}{}, nil)
}
func (c *Client) Send(ctx context.Context, p Credential, m Message) error {
	if !validMessage(m) {
		return errors.New("only six-digit verification and password-reset codes can use shared email")
	}
	return c.request(ctx, "/v1/send", p.ID+"."+p.Secret, m, nil)
}

var hexID = regexp.MustCompile(`^[a-f0-9]{64}$`)
var codePattern = regexp.MustCompile(`^[0-9]{6}$`)

func validMessage(m Message) bool {
	return hexID.MatchString(m.ID) && validEmail(m.Recipient) && codePattern.MatchString(m.Code) && (m.Purpose == "verification" || m.Purpose == "password-reset")
}
