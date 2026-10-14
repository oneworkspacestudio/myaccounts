package portable

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIssuerVerification(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		var issuer string
		s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := issuer
			if mismatch {
				got = "https://attacker.example"
			}
			fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q,"jwks_uri":%q}`, got, issuer+"/authorize", issuer+"/token", issuer+"/jwks")
		}))
		issuer = s.URL
		e := VerifyIssuer(context.Background(), issuer, s.Client())
		s.Close()
		if (e != nil) != mismatch {
			t.Fatalf("mismatch %v: %v", mismatch, e)
		}
	}
}
