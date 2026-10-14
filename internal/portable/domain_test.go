package portable

import "testing"

func TestDomainIssuer(t *testing.T) {
	for _, c := range []struct{ root, sub, want string }{{"M8E.CO.UK", "login", "https://login.m8e.co.uk"}, {"example.com", "", "https://example.com"}, {"example.com", "accounts.eu", "https://accounts.eu.example.com"}} {
		got, err := DomainIssuer(c.root, c.sub)
		if err != nil || got != c.want {
			t.Fatalf("domain: %q %v", got, err)
		}
	}
	for _, c := range [][2]string{{"https://example.com", "login"}, {"example.com:8080", "login"}, {"example.com", "-bad"}, {"example.com", "../bad"}, {"example.com", "bad..name"}, {"localhost", "login"}} {
		if _, err := DomainIssuer(c[0], c[1]); err == nil {
			t.Fatalf("invalid domain accepted: %v", c)
		}
	}
}
