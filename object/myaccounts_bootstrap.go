package object

import (
	"encoding/json"
	"errors"
	"github.com/casdoor/casdoor/internal/portable"
	"github.com/casdoor/casdoor/util"
	"os"
)

type bootstrap = portable.SetupRequest

func CloseMyAccounts() error {
	if ormer == nil || ormer.Engine == nil {
		return nil
	}
	rows, e := ormer.Engine.QueryString("PRAGMA wal_checkpoint(TRUNCATE)")
	if e != nil {
		return e
	}
	if len(rows) != 1 || rows[0]["busy"] != "0" {
		return errors.New("SQLite checkpoint could not complete")
	}
	return ormer.Engine.Close()
}

// Called before InitDb, only for a locally authorized first-run bootstrap.
func PrepareMyAccounts() error {
	b, e := os.ReadFile("secrets/bootstrap.dpapi")
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	plain, e := portable.Unprotect(b)
	if e != nil {
		return e
	}
	var p bootstrap
	if e = json.Unmarshal(plain, &p); e != nil {
		return e
	}
	if len(p.Password) < 12 {
		return errors.New("invalid administrator bootstrap")
	}
	return os.Setenv("initAdminPassword", p.Password)
}
func FinishMyAccounts() error {
	b, e := os.ReadFile("secrets/bootstrap.dpapi")
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	plain, e := portable.Unprotect(b)
	if e != nil {
		return e
	}
	var p bootstrap
	if e = json.Unmarshal(plain, &p); e != nil {
		return e
	}
	user, e := GetUser("built-in/admin")
	if e != nil {
		return e
	}
	if user == nil || user.Password == "" {
		return errors.New("administrator was not initialized")
	}
	user.Email = p.Email
	user.DisplayName = p.Name
	user.Phone = ""
	user.Affiliation = ""
	user.UpdatedTime = util.GetCurrentTime()
	if _, e = UpdateUser(user.GetId(), user, []string{"email", "display_name", "phone", "affiliation", "updated_time"}, true); e != nil {
		return e
	}

	if p.SMTPHost != "" {
		provider := &Provider{Owner: "admin", Name: "myaccounts-smtp", CreatedTime: util.GetCurrentTime(), DisplayName: "MyAccounts Email", Category: "Email", Type: "SMTP", Host: p.SMTPHost, Port: 465, SslMode: "Enable", ClientId: p.SMTPUsername, ClientSecret: p.SMTPPassword, ClientId2: p.SMTPFrom, ClientSecret2: "MyAccounts", Title: "MyAccounts verification", Content: "Your verification code is %s"}
		existing, e := GetProvider(provider.GetId())
		if e != nil {
			return e
		}
		if existing == nil {
			if _, e = AddProvider(provider); e != nil {
				return e
			}
		}
	}
	os.Unsetenv("initAdminPassword")
	return os.Remove("secrets/bootstrap.dpapi")
}
