package portable

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Cloudflare struct {
	BaseURL, Token string
	Client         *http.Client
}

func (c Cloudflare) call(ctx context.Context, method, path string, body, out any) error {
	var data []byte
	var e error
	if body != nil {
		data, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	base := c.BaseURL
	if base == "" {
		base = "https://api.cloudflare.com/client/v4"
	}
	req, e := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(data))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, e := client.Do(req)
	if e != nil {
		return errors.New("Cloudflare API is unreachable")
	}
	defer resp.Body.Close()
	var result struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
	}
	e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result)
	if e != nil || resp.StatusCode >= 300 || !result.Success {
		return fmt.Errorf("Cloudflare API rejected %s %s (HTTP %d); check token scopes and ownership", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(result.Result, out)
	}
	return nil
}

var cloudID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type ProvisionRequest struct {
	AccountID, ZoneID, Hostname string
	Port                        int
	Authorized                  bool
}
type ProvisionResult struct{ TunnelID, Token string }

// Only creates fresh resources. Never overwrites a DNS record or reconfigures someone else's tunnel.
func (c Cloudflare) Provision(ctx context.Context, p ProvisionRequest) (ProvisionResult, error) {
	var result ProvisionResult
	if !p.Authorized {
		return result, errors.New("explicit Cloudflare resource creation authorization is required")
	}
	if !cloudID.MatchString(p.AccountID) || !cloudID.MatchString(p.ZoneID) || !hostname.MatchString(p.Hostname) || p.Port < 1024 || p.Port > 65535 {
		return result, errors.New("invalid account, zone, hostname or port")
	}
	var verify struct {
		Status string `json:"status"`
	}
	if e := c.call(ctx, "GET", "/user/tokens/verify", nil, &verify); e != nil {
		return result, e
	}
	if verify.Status != "active" {
		return result, errors.New("Cloudflare token is not active")
	}
	var zone struct {
		Name, Status string
		Account      struct {
			ID string `json:"id"`
		}
	}
	if e := c.call(ctx, "GET", "/zones/"+p.ZoneID, nil, &zone); e != nil {
		return result, e
	}
	if zone.Account.ID != p.AccountID || zone.Status != "active" || !strings.HasSuffix(p.Hostname, "."+zone.Name) {
		return result, errors.New("hostname must be a subdomain of an active zone in the selected account")
	}
	dnsPath := "/zones/" + p.ZoneID + "/dns_records"
	var records []struct{ ID string }
	if e := c.call(ctx, "GET", dnsPath+"?name="+url.QueryEscape(p.Hostname), nil, &records); e != nil {
		return result, e
	}
	if len(records) != 0 {
		return result, errors.New("DNS collision: hostname already exists; no changes made")
	}
	var tunnel struct {
		ID string `json:"id"`
	}
	accountPath := "/accounts/" + p.AccountID + "/cfd_tunnel"
	if e := c.call(ctx, "POST", accountPath, map[string]any{"name": "myaccounts-" + RandomToken()[:12], "remote_config": true}, &tunnel); e != nil {
		return result, e
	}
	result.TunnelID = tunnel.ID
	fail := func(e error) (ProvisionResult, error) {
		return result, fmt.Errorf("%w; newly created tunnel %s may need manual cleanup", e, tunnel.ID)
	}
	config := map[string]any{"config": map[string]any{"ingress": []any{map[string]any{"hostname": p.Hostname, "service": fmt.Sprintf("http://127.0.0.1:%d", p.Port)}, map[string]any{"service": "http_status:404"}}}}
	if e := c.call(ctx, "PUT", accountPath+"/"+tunnel.ID+"/configurations", config, nil); e != nil {
		return fail(e)
	}
	// Recheck after creation; fail without overwriting if another actor added DNS.
	if e := c.call(ctx, "GET", dnsPath+"?name="+url.QueryEscape(p.Hostname), nil, &records); e != nil {
		return fail(e)
	}
	if len(records) != 0 {
		return fail(errors.New("DNS hostname appeared during provisioning"))
	}
	if e := c.call(ctx, "POST", dnsPath, map[string]any{"type": "CNAME", "name": p.Hostname, "content": tunnel.ID + ".cfargotunnel.com", "proxied": true, "ttl": 1}, nil); e != nil {
		return fail(e)
	}
	if e := c.call(ctx, "GET", accountPath+"/"+tunnel.ID+"/token", nil, &result.Token); e != nil {
		return fail(e)
	}
	return result, nil
}
func EnsureConnector(ctx context.Context, dir string) (string, error) {
	target := filepath.Join(dir, "tools", "cloudflared.exe")
	if e := os.MkdirAll(filepath.Dir(target), 0700); e != nil {
		return "", e
	}
	client := &http.Client{Timeout: 120 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/cloudflare/cloudflared/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, e := client.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("official connector release metadata unavailable")
	}
	var release struct {
		Assets []struct {
			Name, Digest string
			URL          string `json:"browser_download_url"`
		}
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&release); e != nil {
		return "", e
	}
	for _, asset := range release.Assets {
		if asset.Name != "cloudflared-windows-amd64.exe" {
			continue
		}
		expected := strings.TrimPrefix(asset.Digest, "sha256:")
		if len(expected) != 64 || !strings.HasPrefix(asset.URL, "https://github.com/cloudflare/cloudflared/releases/download/") {
			return "", errors.New("official release has no verifiable SHA256 digest")
		}
		if b, e := os.ReadFile(target); e == nil {
			sum := sha256.Sum256(b)
			if hex.EncodeToString(sum[:]) == expected {
				return target, nil
			}
		}
		req, _ = http.NewRequestWithContext(ctx, "GET", asset.URL, nil)
		r, e := client.Do(req)
		if e != nil {
			return "", e
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			return "", errors.New("connector download failed")
		}
		b, e := io.ReadAll(io.LimitReader(r.Body, 128<<20+1))
		if e != nil {
			return "", e
		}
		if len(b) > 128<<20 {
			return "", errors.New("connector too large")
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != expected {
			return "", errors.New("connector checksum mismatch")
		}
		if e = AtomicWrite(target, b); e != nil {
			return "", e
		}
		return target, nil
	}
	return "", errors.New("official Windows AMD64 connector asset missing")
}
func RunTunnel(ctx context.Context, dir string, output io.Writer) {
	status := func(state string) {
		_ = AtomicWrite(filepath.Join(dir, "tunnel-status.json"), []byte(fmt.Sprintf(`{"state":%q,"checkedAt":%q}`, state, time.Now().UTC().Format(time.RFC3339))))
	}
	status("downloading")
	exe, e := EnsureConnector(ctx, dir)
	if e != nil {
		status("connector-verification-failed")
		return
	}
	b, e := os.ReadFile(filepath.Join(dir, "secrets", "tunnel.dpapi"))
	if e != nil {
		status("missing-token")
		return
	}
	token, e := Unprotect(b)
	if e != nil {
		status("token-decryption-failed")
		return
	}
	for delay := time.Second; ctx.Err() == nil; {
		// Token is passed in the inherited environment, never command-line arguments.
		cmd := exec.CommandContext(ctx, exe, "tunnel", "--no-autoupdate", "run")
		cmd.Env = append(os.Environ(), "TUNNEL_TOKEN="+string(token))
		cmd.Stdout = output
		cmd.Stderr = output
		status("connecting")
		e = cmd.Run()
		if ctx.Err() != nil {
			break
		}
		status("reconnecting")
		select {
		case <-ctx.Done():
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
	status("stopped")
}
