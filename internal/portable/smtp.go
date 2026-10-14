package portable

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// Setup uses implicit TLS to prevent opportunistic SMTP downgrade.
func TestEmail(p SetupRequest) error {
	if p.SMTPHost == "" || strings.ContainsAny(p.SMTPHost, "\r\n/: ") || p.SMTPUsername == "" || p.SMTPPassword == "" {
		return errors.New("enter an SMTP host, username and app password")
	}
	for _, address := range []string{p.SMTPFrom, p.Email} {
		a, e := mail.ParseAddress(address)
		if e != nil || a.Address != address {
			return errors.New("enter valid sender and recipient email addresses")
		}
	}
	conn, e := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", net.JoinHostPort(p.SMTPHost, "465"), &tls.Config{ServerName: p.SMTPHost, MinVersion: tls.VersionTLS12})
	if e != nil {
		return errors.New("SMTP TLS connection failed; check host and port 465")
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(20 * time.Second))
	c, e := smtp.NewClient(conn, p.SMTPHost)
	if e != nil {
		return errors.New("SMTP greeting failed")
	}
	defer c.Close()
	if e = c.Auth(smtp.PlainAuth("", p.SMTPUsername, p.SMTPPassword, p.SMTPHost)); e != nil {
		return errors.New("SMTP authentication failed")
	}
	if e = c.Mail(p.SMTPFrom); e != nil {
		return errors.New("SMTP sender rejected")
	}
	if e = c.Rcpt(p.Email); e != nil {
		return errors.New("SMTP recipient rejected")
	}
	w, e := c.Data()
	if e != nil {
		return e
	}
	_, e = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: MyAccounts email test\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nYour MyAccounts server can send email. Configure email verification before enabling registration.\r\n", p.SMTPFrom, p.Email)
	if e != nil {
		return e
	}
	if e = w.Close(); e != nil {
		return e
	}
	return c.Quit()
}
