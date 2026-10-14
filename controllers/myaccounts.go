package controllers

import (
	"context"
	_ "embed"
	"encoding/json"
	"github.com/casdoor/casdoor/object"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/casdoor/casdoor/internal/portable"
)

//go:embed myaccounts.html
var operationsHTML []byte
var backupFileName = regexp.MustCompile(`^myaccounts-[0-9TZ]+-[a-f0-9]{8}\.age$`)

func (c *ApiController) MyAccountsConsole() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	c.Ctx.Output.Header("Content-Type", "text/html; charset=utf-8")
	c.Ctx.Output.Header("Cache-Control", "no-store")
	_ = c.Ctx.Output.Body(operationsHTML)
}

func (c *ApiController) requireMyAccountsAdmin() bool {
	if !c.IsGlobalAdmin() {
		c.Ctx.Output.SetStatus(http.StatusForbidden)
		c.ResponseError("Administrator access required")
		return false
	}
	return true
}
func (c *RootController) MyAccountsOperations() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	c.Ctx.Output.Header("Content-Type", "text/html; charset=utf-8")
	c.Ctx.Output.Header("Cache-Control", "no-store")
	_ = c.Ctx.Output.Body(operationsHTML)
}
func (c *ApiController) MyAccountsStatus() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	config, e := portable.Load(".")
	if e != nil {
		c.ResponseError("Configuration unavailable")
		return
	}
	dir, _ := filepath.Abs(".")
	items := []map[string]any{}
	entries, _ := os.ReadDir("backups")
	for _, entry := range entries {
		if !entry.IsDir() && backupFileName.MatchString(entry.Name()) {
			info, e := entry.Info()
			if e == nil {
				items = append(items, map[string]any{"name": entry.Name(), "bytes": info.Size(), "created": info.ModTime()})
			}
		}
	}
	_, credentialError := os.Stat("secrets/backup-passphrase.dpapi")
	_, emailError := portable.LoadSharedEmail(".")
	c.ResponseOk(map[string]any{"issuer": config.Issuer, "dataPath": dir, "tunnelMode": config.TunnelMode, "databaseHealthy": portable.CheckDatabase("users.db") == nil, "backupHours": config.BackupHours, "scheduleCredentialReady": credentialError == nil, "backupState": portable.BackupStatus("."), "backups": items, "sharedEmailConnected": config.SharedEmail && emailError == nil})
}
func (c *ApiController) MyAccountsEmailSetup() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		defer cancel()
		e := portable.SetupEmail(ctx, ".", func(link string) { opened <- link })
		if e == nil {
			e = object.ConfigureMyAccountsEmail()
		}
		done <- e
	}()
	select {
	case link := <-opened:
		c.ResponseOk(map[string]string{"url": link})
	case e := <-done:
		if e != nil {
			c.ResponseError(e.Error())
		} else {
			c.ResponseOk("Email connected")
		}
	case <-time.After(10 * time.Second):
		cancel()
		c.ResponseError("Email setup could not start")
	}
}
func (c *ApiController) MyAccountsVerify() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	config, e := portable.Load(".")
	if e != nil {
		c.ResponseError("Configuration unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if e = portable.VerifyIssuer(ctx, config.Issuer, nil); e != nil {
		c.ResponseError(e.Error())
		return
	}
	c.ResponseOk("HTTPS and exact issuer/discovery verified")
}
func (c *ApiController) MyAccountsBackup() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	var p struct {
		Passphrase string `json:"passphrase"`
	}
	if len(c.Ctx.Input.RequestBody) > 4096 || json.Unmarshal(c.Ctx.Input.RequestBody, &p) != nil {
		c.ResponseError("Invalid backup request")
		return
	}
	name := portable.BackupName()
	if e := portable.OnlineBackup(".", filepath.Join("backups", name), p.Passphrase); e != nil {
		c.ResponseError(e.Error())
		return
	}
	if e := portable.RecordBackup(".", name); e != nil {
		c.ResponseError("Backup saved; schedule state needs checking")
		return
	}
	c.ResponseOk(name)
}
func (c *ApiController) MyAccountsSchedule() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	var p struct {
		Hours      int    `json:"hours"`
		Passphrase string `json:"passphrase"`
	}
	if len(c.Ctx.Input.RequestBody) > 4096 || json.Unmarshal(c.Ctx.Input.RequestBody, &p) != nil {
		c.ResponseError("Invalid schedule request")
		return
	}
	if e := portable.SaveBackupSchedule(".", p.Hours, p.Passphrase); e != nil {
		c.ResponseError(e.Error())
		return
	}
	c.ResponseOk()
}
func (c *ApiController) MyAccountsDownload() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	name := c.GetString("name")
	if !backupFileName.MatchString(name) {
		c.Ctx.Output.SetStatus(400)
		c.ResponseError("Invalid backup name")
		return
	}
	c.Ctx.Output.Header("Cache-Control", "no-store")
	c.Ctx.Output.Download(filepath.Join("backups", name))
}
func (c *ApiController) MyAccountsUpdate() {
	if !c.requireMyAccountsAdmin() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, e := portable.CheckUpdate(ctx, os.Getenv("MYACCOUNTS_GITHUB_TOKEN"))
	if e != nil {
		c.ResponseError(e.Error())
		return
	}
	c.ResponseOk(result)
}
