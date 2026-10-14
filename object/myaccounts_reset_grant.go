package object

import (
	"errors"
	"time"

	"github.com/casdoor/casdoor/conf"
	"github.com/casdoor/casdoor/internal/portable"
)

// Password recovery reuses the engine's verification record and session. The
// database binds a short-lived, single-use permission to that exact user.
func IssuePasswordResetGrant(dest, code, userID string) (string, error) {
	record, e := getVerificationRecord(dest)
	if e != nil {
		return "", e
	}
	now := time.Now().Unix()
	if record == nil || record.IsUsed || record.Code != code || record.Time+int64(conf.GetVerificationCodeTimeout())*60 < now {
		return "", errors.New("verification code expired or already used")
	}
	session := ormer.Engine.NewSession()
	defer session.Close()
	if e = session.Begin(); e != nil {
		return "", e
	}
	defer session.Rollback()
	result, e := session.Exec("UPDATE verification_record SET is_used=1 WHERE owner=? AND name=? AND code=? AND is_used=0", record.Owner, record.Name, code)
	if e != nil {
		return "", e
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return "", errors.New("verification code expired or already used")
	}
	token := portable.RandomToken()
	if _, e = session.Exec("INSERT INTO myaccounts_reset_grant(grant_id,user_id,expires,used) VALUES(?,?,?,0)", factorKey("reset", token), userID, record.Time+int64(conf.GetVerificationCodeTimeout())*60); e != nil {
		return "", e
	}
	if e = session.Commit(); e != nil {
		return "", e
	}
	// Keep only live grants. Cleanup failure cannot grant access.
	_, _ = ormer.Engine.Exec("DELETE FROM myaccounts_reset_grant WHERE expires < ?", now)
	return token, nil
}

func ConsumePasswordResetGrant(token, userID string) error {
	if token == "" {
		return errors.New("password recovery expired; request a new code")
	}
	result, e := ormer.Engine.Exec("UPDATE myaccounts_reset_grant SET used=1 WHERE grant_id=? AND user_id=? AND expires>=? AND used=0", factorKey("reset", token), userID, time.Now().Unix())
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil || n != 1 {
		return errors.New("password recovery expired or already used; request a new code")
	}
	return nil
}

func RevokePasswordSessions(user *User) error {
	return terminateUserAccess(user)
}
