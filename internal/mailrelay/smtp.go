package mailrelay

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig is held only by the service operator, inside protected storage.
// Delivery uses implicit TLS; no plaintext fallback exists.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
}

func (p SMTPConfig) Validate() error {
	if !regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9.-]*[a-zA-Z0-9])?$`).MatchString(p.Host) || p.Port != 465 || p.Username == "" || len(p.Password) < 8 || !validEmail(p.From) {
		return errors.New("enter an encrypted SMTP host (port 465), username, app password and sender email")
	}
	return nil
}
func (p SMTPConfig) Deliver(ctx context.Context, m Delivery) error {
	if e := p.Validate(); e != nil {
		return e
	}
	if !validEmail(m.Recipient) || strings.ContainsAny(m.Subject, "\r\n") {
		return errors.New("invalid message")
	}
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 10 * time.Second}, Config: &tls.Config{ServerName: p.Host, MinVersion: tls.VersionTLS12}}
	conn, e := dialer.DialContext(ctx, "tcp", net.JoinHostPort(p.Host, strconv.Itoa(p.Port)))
	if e != nil {
		return errors.New("email server TLS connection failed")
	}
	defer conn.Close()
	deadline := time.Now().Add(25 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	c, e := smtp.NewClient(conn, p.Host)
	if e != nil {
		return errors.New("email server greeting failed")
	}
	defer c.Close()
	if e = c.Auth(smtp.PlainAuth("", p.Username, p.Password, p.Host)); e != nil {
		return errors.New("email authentication failed; check the app password")
	}
	if e = c.Mail(p.From); e != nil {
		return errors.New("email sender rejected")
	}
	if e = c.Rcpt(m.Recipient); e != nil {
		return errors.New("email recipient rejected")
	}
	w, e := c.Data()
	if e != nil {
		return errors.New("email server refused message")
	}
	_, e = fmt.Fprintf(w, "From: MyAccounts <%s>\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@%s>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n", p.From, m.Recipient, m.Subject, time.Now().Format(time.RFC1123Z), RandomSecret(), p.From[strings.LastIndex(p.From, "@")+1:], m.Text)
	if e != nil {
		return errors.New("email message could not be written")
	}
	if e = w.Close(); e != nil {
		return errors.New("email server did not accept message")
	}
	// DATA acceptance is the success boundary. A QUIT failure must not turn
	// an accepted message into a retry and a duplicate email.
	_ = c.Quit()
	return nil
}
