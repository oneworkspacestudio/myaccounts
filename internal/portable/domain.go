package portable

import (
	"errors"
	"strings"
)

// DomainIssuer turns the user's explicit domain choice into a canonical issuer.
func DomainIssuer(root, sub string) (string, error) {
	root = strings.ToLower(strings.TrimSpace(root))
	sub = strings.ToLower(strings.TrimSpace(sub))
	if !hostname.MatchString(root) || strings.ContainsAny(root, "/:@") {
		return "", errors.New("enter a root domain such as example.com")
	}
	if sub != "" {
		for _, label := range strings.Split(sub, ".") {
			if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return "", errors.New("enter a valid subdomain")
			}
			for _, r := range label {
				if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
					return "", errors.New("enter a valid subdomain")
				}
			}
		}
		root = sub + "." + root
	}
	if len(root) > 253 {
		return "", errors.New("domain is too long")
	}
	return "https://" + root, nil
}
