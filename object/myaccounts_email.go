package object

import (
	"github.com/casdoor/casdoor/internal/portable"
	"github.com/casdoor/casdoor/util"
	"os"
)

// Configure only the two owned default applications and only after the local
// wizard has verified email ownership. Existing third-party clients are untouched.
// Signup remains an administrator's explicit choice, especially in built-in.
func ConfigureMyAccountsEmail() error {
	config, e := portable.Load(".")
	if e != nil {
		return e
	}
	if !config.SharedEmail {
		return nil
	}
	credential, e := portable.LoadSharedEmail(".")
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	if b, e := os.ReadFile("email-applied.id"); e == nil && string(b) == credential.ID {
		return nil
	}
	p := &Provider{Owner: "admin", Name: "myaccounts-email", CreatedTime: util.GetCurrentTime(), DisplayName: "MyAccounts shared email", Category: "Email", Type: "MyAccounts Relay", Title: "MyAccounts verification", Content: "Your verification code is %s"}
	existing, e := GetProvider(p.GetId())
	if e != nil {
		return e
	}
	if existing == nil {
		if _, e = AddProvider(p); e != nil {
			return e
		}
	}
	for _, name := range []string{"admin/app-built-in", "admin/myaccounts"} {
		app, e := GetApplication(name)
		if e != nil {
			return e
		}
		if app == nil {
			continue
		}
		// Respect an explicitly configured alternative email provider.
		otherEmail := false
		attached := false
		for _, item := range app.Providers {
			if item.Name == p.Name && item.Owner == p.Owner {
				attached = true
				continue
			}
			provider, e := GetProvider(item.Owner + "/" + item.Name)
			if e != nil {
				return e
			}
			if provider != nil && provider.Category == "Email" {
				otherEmail = true
			}
		}
		if otherEmail {
			continue
		}
		if !attached {
			app.Providers = append(app.Providers, &ProviderItem{Owner: p.Owner, Name: p.Name, CanSignUp: true, CanSignIn: true, Rule: "All"})
		}
		for _, item := range app.SigninItems {
			if item.Name == "Forgot password?" {
				item.Visible = true
			}
		}
		if _, e = UpdateApplication(app.GetId(), app, true, "en", []string{"providers", "signin_items"}); e != nil {
			return e
		}
	}
	user, e := GetUser("built-in/admin")
	if e != nil {
		return e
	}
	if user != nil && user.Email == credential.Email {
		user.EmailVerified = true
		if _, e = UpdateUser(user.GetId(), user, []string{"email_verified"}, true); e != nil {
			return e
		}
	}
	// The marker contains only a random public installation ID, no credential.
	return portable.AtomicWrite("email-applied.id", []byte(credential.ID))
}
