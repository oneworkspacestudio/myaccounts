package portable

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCloudflareProvisioning(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(fmt.Sprint(collision), func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Error("missing authentication")
				}
				w.Header().Set("Content-Type", "application/json")
				result := `{}`
				switch {
				case r.URL.Path == "/user/tokens/verify":
					result = `{"status":"active"}`
				case strings.HasPrefix(r.URL.Path, "/zones/") && !strings.Contains(r.URL.Path, "dns_records"):
					result = `{"name":"example.com","status":"active","account":{"id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`
				case strings.Contains(r.URL.Path, "dns_records") && r.Method == "GET":
					result = `[]`
					if collision {
						result = `[{"id":"existing"}]`
					}
				case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "cfd_tunnel"):
					writes++
					result = `{"id":"new-tunnel"}`
				case strings.HasSuffix(r.URL.Path, "/token"):
					result = `"operational-token"`
				default:
					if r.Method == "POST" || r.Method == "PUT" {
						writes++
					}
				}
				fmt.Fprintf(w, `{"success":true,"result":%s}`, result)
			}))
			defer server.Close()
			cf := Cloudflare{BaseURL: server.URL, Token: "test-token", Client: server.Client()}
			p := ProvisionRequest{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "login.example.com", 8787, true}
			result, e := cf.Provision(context.Background(), p)
			if collision {
				if e == nil || writes != 0 {
					t.Fatalf("DNS collision wrote resources: %d %v", writes, e)
				}
			} else {
				if e != nil || result.Token != "operational-token" || writes != 3 {
					t.Fatalf("provision: %+v %d %v", result, writes, e)
				}
			}
		})
	}
}
func TestCloudflareRequiresAuthorization(t *testing.T) {
	_, e := (Cloudflare{}).Provision(context.Background(), ProvisionRequest{})
	if e == nil {
		t.Fatal("unauthorized provisioning accepted")
	}
}
func TestCloudflareRejectsCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(403); fmt.Fprint(w, `{"success":false}`) }))
	defer server.Close()
	_, e := (Cloudflare{BaseURL: server.URL, Client: server.Client()}).Provision(context.Background(), ProvisionRequest{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "login.example.com", 8787, true})
	if e == nil {
		t.Fatal("rejected token accepted")
	}
}
