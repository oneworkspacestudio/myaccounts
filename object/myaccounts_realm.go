package object

import (
	"encoding/json"
	"errors"
	"github.com/casdoor/casdoor/internal/portable"
)

// Built-in is the engine's global-administrator realm. Ordinary identities must
// be created separately; do not enable built-in privilege consent for signup.
func EnsureMyAccountsRealm() error {
	org, e := GetOrganization("admin/myaccounts")
	if e != nil {
		return e
	}
	if org == nil {
		source, e := GetOrganization("admin/built-in")
		if e != nil {
			return e
		}
		if source == nil {
			return errors.New("administrator organization missing")
		}
		b, e := json.Marshal(source)
		if e != nil {
			return e
		}
		org = &Organization{}
		if e = json.Unmarshal(b, org); e != nil {
			return e
		}
		org.Owner = "admin"
		org.Name = "myaccounts"
		org.DisplayName = "MyAccounts"
		org.DefaultApplication = ""
		org.HasPrivilegeConsent = false
		if _, e = AddOrganization(org); e != nil {
			return e
		}
	}
	existing, e := GetApplication("admin/myaccounts")
	if e != nil {
		return e
	}
	if existing != nil {
		return nil
	}
	source, e := GetApplication("admin/app-built-in")
	if e != nil {
		return e
	}
	if source == nil {
		return errors.New("administrator application missing")
	}
	b, e := json.Marshal(source)
	if e != nil {
		return e
	}
	app := &Application{}
	if e = json.Unmarshal(b, app); e != nil {
		return e
	}
	app.Owner = "admin"
	app.Name = "myaccounts"
	app.DisplayName = "MyAccounts"
	app.Organization = "myaccounts"
	app.DefaultGroup = ""
	app.ClientId = "myaccounts"
	app.ClientSecret = portable.RandomToken()
	app.RedirectUris = []string{}
	app.Providers = []*ProviderItem{}
	app.EnableSignUp = false
	app.DisableSelfSignup = true
	app.EnableAutoSignin = false
	app.SigninMethods = []*SigninMethod{{Name: "Password"}, {Name: "WebAuthn"}}
	for _, item := range app.SigninItems {
		if item.Name == "Username" {
			item.Label = "Email or username"
		}
		if item.Name == "Forgot password?" {
			item.Visible = false
		}
	}
	_, e = AddApplication(app, "en")
	return e
}
