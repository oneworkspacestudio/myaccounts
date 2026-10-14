package portable

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func VerifyIssuer(ctx context.Context, issuer string, client *http.Client) error {
	if !strings.HasPrefix(issuer, "https://") && !strings.HasPrefix(issuer, "http://localhost:") {
		return errors.New("issuer must be HTTPS or local development")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	target := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, e := http.NewRequestWithContext(ctx, "GET", target, nil)
	if e != nil {
		return e
	}
	r, e := client.Do(req)
	if e != nil {
		return errors.New("issuer HTTPS/discovery is not reachable")
	}
	defer r.Body.Close()
	if r.StatusCode != 200 || r.Request.URL.String() != target {
		return errors.New("discovery must return HTTP 200 without redirects")
	}
	var document struct{ Issuer, AuthorizationEndpoint, TokenEndpoint, JWKS string }
	var raw map[string]json.RawMessage
	if e = json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&raw); e != nil {
		return e
	}
	_ = json.Unmarshal(raw["issuer"], &document.Issuer)
	_ = json.Unmarshal(raw["authorization_endpoint"], &document.AuthorizationEndpoint)
	_ = json.Unmarshal(raw["token_endpoint"], &document.TokenEndpoint)
	_ = json.Unmarshal(raw["jwks_uri"], &document.JWKS)
	if document.Issuer != issuer {
		return errors.New("discovery issuer differs from the configured issuer")
	}
	for _, endpoint := range []string{document.AuthorizationEndpoint, document.TokenEndpoint, document.JWKS} {
		if !strings.HasPrefix(endpoint, strings.TrimRight(issuer, "/")+"/") {
			return errors.New("discovery endpoints must belong to the configured issuer")
		}
	}
	return nil
}
