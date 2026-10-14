package portable

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"
)

type BackupState struct {
	LastSuccess time.Time `json:"lastSuccess"`
	File        string    `json:"file"`
}

func BackupName() string {
	return "myaccounts-" + time.Now().UTC().Format("20060102T150405Z") + "-" + RandomToken()[:8] + ".age"
}
func SaveBackupSchedule(dir string, hours int, password string) error {
	c, e := Load(dir)
	if e != nil {
		return e
	}
	c.BackupHours = hours
	if e = Validate(c); e != nil {
		return e
	}
	if hours > 0 {
		if len(password) < 16 {
			return errors.New("backup passphrase must contain at least 16 characters")
		}
		b, e := Protect([]byte(password))
		if e != nil {
			return e
		}
		if e = AtomicWrite(filepath.Join(dir, "secrets", "backup-passphrase.dpapi"), b); e != nil {
			return e
		}
	}
	if e = Save(dir, c); e != nil {
		return e
	}
	if hours == 0 {
		_ = os.Remove(filepath.Join(dir, "secrets", "backup-passphrase.dpapi"))
	}
	return nil
}
func BackupStatus(dir string) BackupState {
	var state BackupState
	b, e := os.ReadFile(filepath.Join(dir, "logs", "backup-state.json"))
	if e == nil {
		_ = json.Unmarshal(b, &state)
	}
	return state
}
func RecordBackup(dir, name string) error {
	b, e := json.Marshal(BackupState{LastSuccess: time.Now().UTC(), File: name})
	if e != nil {
		return e
	}
	return AtomicWrite(filepath.Join(dir, "logs", "backup-state.json"), b)
}
func RunScheduledBackups(ctx context.Context, dir string) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c, e := Load(dir)
			if e != nil || c.BackupHours == 0 {
				continue
			}
			state := BackupStatus(dir)
			if !state.LastSuccess.IsZero() && time.Since(state.LastSuccess) < time.Duration(c.BackupHours)*time.Hour {
				continue
			}
			protected, e := os.ReadFile(filepath.Join(dir, "secrets", "backup-passphrase.dpapi"))
			if e != nil {
				continue
			}
			password, e := Unprotect(protected)
			if e != nil {
				log.Print("Scheduled backup credential needs reconfiguration")
				continue
			}
			name := BackupName()
			if e = OnlineBackup(dir, filepath.Join(dir, "backups", name), string(password)); e != nil {
				log.Print("Scheduled backup failed; prior backups retained")
				continue
			}
			if e = RecordBackup(dir, name); e != nil {
				log.Print("Backup created; schedule state could not be saved")
			}
		}
	}
}
