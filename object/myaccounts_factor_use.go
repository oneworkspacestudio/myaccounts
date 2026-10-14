package object

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"time"
)

// Credential consumption is application persistence, not a cryptographic implementation.
// All OTP generation/validation remains in pquerna/otp; SQLite performs atomic compare-and-set.
func InitFactorStore() error {
	_, e := ormer.Engine.Exec(`CREATE TABLE IF NOT EXISTS myaccounts_factor_use (user_id TEXT NOT NULL, credential TEXT NOT NULL, last_step INTEGER NOT NULL, PRIMARY KEY(user_id, credential))`)
	return e
}
func factorKey(kind, secret string) string {
	sum := sha256.Sum256([]byte(kind + ":" + secret))
	return fmt.Sprintf("%x", sum[:])
}
func VerifyTotpOnce(user *User, passcode, lang string) error {
	props := user.GetMfaProps(TotpType, false)
	mfa := NewTotpMfaUtil(props)
	if e := mfa.Verify(passcode, lang); e != nil {
		return e
	}
	current := time.Now().UTC().Unix() / MfaTotpPeriodInSeconds
	matched := int64(-1)
	for offset := int64(-1); offset <= 1; offset++ {
		step := current + offset
		code, e := totp.GenerateCodeCustom(user.TotpSecret, time.Unix(step*MfaTotpPeriodInSeconds, 0), totp.ValidateOpts{Period: MfaTotpPeriodInSeconds, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if e != nil {
			return e
		}
		if subtle.ConstantTimeCompare([]byte(code), []byte(passcode)) == 1 {
			matched = step
		}
	}
	if matched < 0 || user.Id == "" {
		return errors.New("invalid authenticator identity or code")
	}
	key := factorKey("totp", user.TotpSecret)
	if _, e := ormer.Engine.Exec("INSERT OR IGNORE INTO myaccounts_factor_use(user_id,credential,last_step) VALUES(?,?,0)", user.Id, key); e != nil {
		return e
	}
	result, e := ormer.Engine.Exec("UPDATE myaccounts_factor_use SET last_step=? WHERE user_id=? AND credential=? AND last_step<?", matched, user.Id, key, matched)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return errors.New("authenticator code has already been used; wait for the next code")
	}
	return nil
}
func consumeRecoveryCode(user *User, code string) error {
	if user.Id == "" {
		return errors.New("invalid recovery identity")
	}
	result, e := ormer.Engine.Exec("INSERT OR IGNORE INTO myaccounts_factor_use(user_id,credential,last_step) VALUES(?,?,1)", user.Id, factorKey("recovery", code))
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return errors.New("recovery code has already been used")
	}
	return nil
}
