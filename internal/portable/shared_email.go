package portable

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/casdoor/casdoor/internal/mailrelay"
)

func SaveSharedEmail(dir string, p mailrelay.Credential) error {
	if e := p.Validate(); e != nil {
		return e
	}
	previous, _ := LoadSharedEmail(dir)
	b, e := json.Marshal(p)
	if e != nil {
		return e
	}
	b, e = Protect(b)
	if e != nil {
		return e
	}
	if e = AtomicWrite(filepath.Join(dir, "secrets", "email-relay.dpapi"), b); e != nil {
		return e
	}
	if config, e := Load(dir); e == nil {
		config.SharedEmail = true
		if e = Save(dir, config); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if previous.ID != "" && previous.ID != p.ID {
		// Save the working replacement first. An offline prior service cannot
		// prevent reconnection; the operator can revoke it later if necessary.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if c, e := mailrelay.NewClient(previous.Endpoint); e == nil {
			_ = c.Revoke(ctx, previous)
		}
	}
	return nil
}
func LoadSharedEmail(dir string) (mailrelay.Credential, error) {
	var p mailrelay.Credential
	b, e := os.ReadFile(filepath.Join(dir, "secrets", "email-relay.dpapi"))
	if e != nil {
		return p, e
	}
	b, e = Unprotect(b)
	if e != nil {
		return p, e
	}
	e = json.Unmarshal(b, &p)
	if e == nil {
		e = p.Validate()
	}
	return p, e
}
func SendSharedEmail(ctx context.Context, dir, recipient, purpose, code string) error {
	config, e := Load(dir)
	if e != nil || !config.SharedEmail {
		return errors.New("shared email is not enabled for this installation")
	}
	p, e := LoadSharedEmail(dir)
	if e != nil {
		return errors.New("shared email is not connected; reconnect it in setup")
	}
	c, e := mailrelay.NewClient(p.Endpoint)
	if e != nil {
		return e
	}
	return c.Send(ctx, p, mailrelay.Message{ID: mailrelay.RandomSecret(), Recipient: recipient, Purpose: purpose, Code: code})
}
