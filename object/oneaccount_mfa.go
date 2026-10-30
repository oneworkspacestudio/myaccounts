package object

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/casdoor/casdoor/internal/portable"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// OneAccount MFA uses the existing user, session and SQLite store. The AES key
// is separate from SQLite and protected with the existing machine-DPAPI + ACL
// mechanism. Backups must include secrets; a database alone cannot decrypt MFA.
var mfaKey []byte
var mfaKeyMu sync.Mutex
var ErrMfaRateLimited = errors.New("too many verification attempts; wait five minutes")

func initOneAccountMfa() error {
	mfaKeyMu.Lock()
	defer mfaKeyMu.Unlock()
	const path = "secrets/oneaccount-mfa.dpapi"
	blob, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		count, countErr := ormer.Engine.Where("totp_secret LIKE 'm1:%'").Count(new(User))
		if countErr != nil {
			return countErr
		}
		if count > 0 {
			return errors.New("OneAccount MFA key missing; restore a verified encrypted account backup")
		}
		key := make([]byte, 32)
		if _, err = rand.Read(key); err != nil {
			return err
		}
		if blob, err = portable.Protect(key); err != nil {
			return err
		}
		if err = portable.AtomicWrite(path, blob); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	if mfaKey, err = portable.Unprotect(blob); err != nil {
		return err
	}
	if len(mfaKey) != 32 {
		return errors.New("invalid OneAccount MFA key")
	}
	for _, sql := range []string{
		`CREATE TABLE IF NOT EXISTS oneaccount_mfa_pending (session_id TEXT PRIMARY KEY,user_id TEXT NOT NULL,secret TEXT NOT NULL,expires INTEGER NOT NULL,verified INTEGER NOT NULL DEFAULT 0,label TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS oneaccount_mfa_proof (session_id TEXT NOT NULL,user_id TEXT NOT NULL,credential TEXT NOT NULL,expires INTEGER NOT NULL,verified INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(session_id,user_id))`,
		`CREATE TABLE IF NOT EXISTS oneaccount_mfa_challenge (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,expires INTEGER NOT NULL,used INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS oneaccount_mfa_limits (user_id TEXT PRIMARY KEY,window INTEGER NOT NULL,attempts INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS oneaccount_mfa_device (user_id TEXT PRIMARY KEY,label TEXT NOT NULL,enrolled INTEGER NOT NULL,last_used INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS oneaccount_mfa_audit (id INTEGER PRIMARY KEY AUTOINCREMENT,user_id TEXT NOT NULL,event TEXT NOT NULL,created INTEGER NOT NULL)`,
	} {
		if _, err = ormer.Engine.Exec(sql); err != nil {
			return err
		}
	}
	columns, err := ormer.Engine.QueryString("PRAGMA table_info(oneaccount_mfa_proof)")
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, c := range columns {
		present[c["name"]] = true
	}
	for name, definition := range map[string]string{"verified": "INTEGER NOT NULL DEFAULT 0"} {
		if !present[name] {
			if _, err = ormer.Engine.Exec("ALTER TABLE oneaccount_mfa_proof ADD COLUMN " + name + " " + definition); err != nil {
				return err
			}
		}
	}
	for _, sql := range []string{"CREATE INDEX IF NOT EXISTS oneaccount_mfa_pending_expiry ON oneaccount_mfa_pending(expires)", "CREATE INDEX IF NOT EXISTS oneaccount_mfa_challenge_expiry ON oneaccount_mfa_challenge(expires)", "CREATE INDEX IF NOT EXISTS oneaccount_mfa_proof_expiry ON oneaccount_mfa_proof(expires)"} {
		if _, err = ormer.Engine.Exec(sql); err != nil {
			return err
		}
	}
	// Existing MFA accounts stay enrolled, but old sessions require a new MFA proof.
	var users []*User
	if err = ormer.Engine.Where("totp_secret <> '' OR recovery_codes <> '[]'").Find(&users); err != nil {
		return err
	}
	for _, user := range users {
		changed := false
		if user.TotpSecret != "" && !strings.HasPrefix(user.TotpSecret, "m1:") {
			if user.TotpSecret, err = EncryptMfaSecret(user.TotpSecret, user.Id); err != nil {
				return err
			}
			changed = true
		}
		for i, code := range user.RecoveryCodes {
			if !strings.HasPrefix(code, "h1:") {
				user.RecoveryCodes[i] = RecoveryDigest(user, code)
				changed = true
			}
		}
		if changed {
			if _, err = updateUser(user.GetId(), user, []string{"totp_secret", "recovery_codes"}); err != nil {
				return err
			}
		}
	}
	return nil
}
func EncryptMfaSecret(secret string, binding string) (string, error) {
	block, err := aes.NewCipher(mfaKey)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return "m1:" + base64.RawURLEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(secret), []byte("OneAccount MFA v1:"+binding))), nil
}
func DecryptMfaSecret(secret string, binding string) (string, error) {
	if !strings.HasPrefix(secret, "m1:") {
		return "", errors.New("unencrypted MFA credential")
	}
	b, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(secret, "m1:"))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(mfaKey)
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(b) < aead.NonceSize() {
		return "", errors.New("damaged MFA credential")
	}
	plain, err := aead.Open(nil, b[:aead.NonceSize()], b[aead.NonceSize():], []byte("OneAccount MFA v1:"+binding))
	if err != nil {
		return "", errors.New("cannot decrypt MFA credential")
	}
	defer clear(plain)
	return string(plain), nil
}
func RecoveryDigest(user *User, code string) string {
	mac := hmac.New(sha256.New, mfaKey)
	mac.Write([]byte(user.Id + ":" + strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))))
	return "h1:" + hex.EncodeToString(mac.Sum(nil))
}
func NewRecoveryCodes(user *User) ([]string, []string, error) {
	raw := make([]string, 10)
	digests := make([]string, 10)
	for i := range raw {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return nil, nil, err
		}
		s := strings.ToUpper(hex.EncodeToString(b))
		raw[i] = s[:8] + "-" + s[8:16] + "-" + s[16:24] + "-" + s[24:]
		digests[i] = RecoveryDigest(user, raw[i])
	}
	return raw, digests, nil
}
func MfaAudit(user *User, event string) error {
	_, err := ormer.Engine.Exec("INSERT INTO oneaccount_mfa_audit(user_id,event,created) VALUES(?,?,?)", user.Id, event, time.Now().Unix())
	return err
}
func MfaAttempt(user *User) error {
	now := time.Now().Unix()
	window := now / 300
	_, err := ormer.Engine.Exec(`INSERT INTO oneaccount_mfa_limits(user_id,window,attempts) VALUES(?,?,1) ON CONFLICT(user_id) DO UPDATE SET attempts=CASE WHEN window=excluded.window THEN attempts+1 ELSE 1 END,window=excluded.window`, user.Id, window)
	if err != nil {
		return err
	}
	rows, err := ormer.Engine.QueryString("SELECT attempts FROM oneaccount_mfa_limits WHERE user_id=?", user.Id)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("MFA limits unavailable")
	}
	// Compare numerically through the DB so malformed state fails closed.
	rows, err = ormer.Engine.QueryString("SELECT user_id FROM oneaccount_mfa_limits WHERE user_id=? AND attempts<=10", user.Id)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return ErrMfaRateLimited
	}
	return nil
}
func MfaPasswordAttempt(user *User) error {
	copy := *user
	copy.Id = user.Id + ":password-stepup"
	return MfaAttempt(&copy)
}
func MfaSessionAuthorized(user *User, session string) bool {
	if user.TotpSecret == "" {
		return true
	}
	if session == "" {
		return false
	}
	rows, err := ormer.Engine.QueryString("SELECT user_id FROM oneaccount_mfa_proof WHERE session_id=? AND user_id=? AND credential=? AND expires>?", session, user.Id, factorKey("epoch", user.TotpSecret), time.Now().Unix())
	return err == nil && len(rows) == 1
}
func MfaStatus(user *User) (map[string]interface{}, error) {
	rows, err := ormer.Engine.QueryString("SELECT label,enrolled,last_used FROM oneaccount_mfa_device WHERE user_id=?", user.Id)
	if err != nil {
		return nil, err
	}
	remaining := 0
	for _, digest := range user.RecoveryCodes {
		used, e := ormer.Engine.QueryString("SELECT credential FROM myaccounts_factor_use WHERE user_id=? AND credential=?", user.Id, factorKey("recovery", digest))
		if e != nil {
			return nil, e
		}
		if len(used) == 0 {
			remaining++
		}
	}
	status := map[string]interface{}{"enabled": user.TotpSecret != "", "recoveryCodesRemaining": remaining, "devices": rows, "deviceLimit": 1}
	events, err := ormer.Engine.QueryString("SELECT event,created FROM oneaccount_mfa_audit WHERE user_id=? ORDER BY id DESC LIMIT 15", user.Id)
	if err != nil {
		return nil, err
	}
	status["securityNotifications"] = events
	return status, nil
}
func CancelMfaEnrolment(user *User, session string) error {
	_, err := ormer.Engine.Exec("DELETE FROM oneaccount_mfa_pending WHERE session_id=? AND user_id=?", session, user.Id)
	return err
}
func RegenerateMfaRecovery(user *User, session string) ([]string, error) {
	raw, digests, err := NewRecoveryCodes(user)
	if err != nil {
		return nil, err
	}
	old := user.TotpSecret
	plain, err := DecryptMfaSecret(old, user.Id)
	if err != nil {
		return nil, err
	}
	encrypted, err := EncryptMfaSecret(plain, user.Id)
	if err != nil {
		return nil, err
	}
	tx := ormer.Engine.NewSession()
	defer tx.Close()
	if err = tx.Begin(); err != nil {
		return nil, err
	}
	defer tx.Rollback()
	user.TotpSecret = encrypted
	user.RecoveryCodes = digests
	n, err := tx.Where("owner=? AND name=? AND totp_secret=?", user.Owner, user.Name, old).Cols("totp_secret", "recovery_codes").Update(user)
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, errors.New("credential changed; sign in again")
	}
	if _, err = tx.Where("organization=? AND "+quoteColumn("user")+"=?", user.Owner, user.Name).Cols("expires_in").Update(&Token{ExpiresIn: 0}); err != nil {
		return nil, err
	}
	result, err := tx.Exec("UPDATE oneaccount_mfa_proof SET credential=? WHERE session_id=? AND user_id=? AND credential=? AND expires>?", factorKey("epoch", encrypted), session, user.Id, factorKey("epoch", old), time.Now().Unix())
	if err != nil {
		return nil, err
	}
	n, err = result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, errors.New("verification expired; sign in again")
	}
	if _, err = tx.Exec("INSERT INTO oneaccount_mfa_audit(user_id,event,created) VALUES(?,?,?)", user.Id, "recovery_codes_regenerated", time.Now().Unix()); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return raw, nil
}
func GrantMfaSession(user *User, session string) error {
	_, _ = ormer.Engine.Exec("DELETE FROM oneaccount_mfa_proof WHERE expires<?", time.Now().Unix())
	_, err := ormer.Engine.Exec(`INSERT INTO oneaccount_mfa_proof(session_id,user_id,credential,expires,verified) VALUES(?,?,?,?,?) ON CONFLICT(session_id,user_id) DO UPDATE SET credential=excluded.credential,expires=excluded.expires,verified=excluded.verified`, session, user.Id, factorKey("epoch", user.TotpSecret), time.Now().Add(30*24*time.Hour).Unix(), time.Now().Unix())
	return err
}

// A completed MFA sign-in is accepted as equivalent factor evidence for five
// minutes, always paired with a fresh password check. Reusing that authenticated
// context is not reusing its OTP/recovery code; their atomic markers remain spent.
func UseRecentMfaStepup(user *User, session string) error {
	now := time.Now().Unix()
	rows, err := ormer.Engine.QueryString("SELECT user_id FROM oneaccount_mfa_proof WHERE session_id=? AND user_id=? AND credential=? AND expires>? AND verified>? AND verified<=?", session, user.Id, factorKey("epoch", user.TotpSecret), now, now-300, now)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("a current authenticator or recovery code is required")
	}
	return nil
}
func BeginMfaChallenge(user *User) (string, error) {
	_, _ = ormer.Engine.Exec("DELETE FROM oneaccount_mfa_challenge WHERE expires<?", time.Now().Unix())
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	_, err := ormer.Engine.Exec("INSERT INTO oneaccount_mfa_challenge(id,user_id,expires) VALUES(?,?,?)", id, user.Id, time.Now().Add(5*time.Minute).Unix())
	return id, err
}
func ClaimMfaChallenge(user *User, id string) error {
	r, err := ormer.Engine.Exec("UPDATE oneaccount_mfa_challenge SET used=1 WHERE id=? AND user_id=? AND expires>? AND used=0", id, user.Id, time.Now().Unix())
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("verification expired; sign in again")
	}
	_, _ = ormer.Engine.Exec("UPDATE oneaccount_mfa_device SET last_used=? WHERE user_id=?", time.Now().Unix(), user.Id)
	return nil
}
func MfaChallengeValid(user *User, id string) bool {
	rows, err := ormer.Engine.QueryString("SELECT id FROM oneaccount_mfa_challenge WHERE id=? AND user_id=? AND used=0 AND expires>?", id, user.Id, time.Now().Unix())
	return err == nil && len(rows) == 1
}
func BeginMfaEnrolment(user *User, session, label string) (*MfaProps, error) {
	_, _ = ormer.Engine.Exec("DELETE FROM oneaccount_mfa_pending WHERE expires<?", time.Now().Unix())
	if user.TotpSecret != "" {
		return nil, errors.New("disable the existing authenticator with password and current proof before replacing it")
	}
	if len(label) > 80 {
		return nil, errors.New("device label too long")
	}
	if label == "" {
		label = "One Authenticator"
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "OneAccount", AccountName: user.GetId(), Period: 30, SecretSize: 20, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		return nil, err
	}
	secret, err := EncryptMfaSecret(key.Secret(), user.Id)
	if err != nil {
		return nil, err
	}
	_, err = ormer.Engine.Exec(`INSERT INTO oneaccount_mfa_pending(session_id,user_id,secret,expires,verified,label) VALUES(?,?,?,?,0,?) ON CONFLICT(session_id) DO UPDATE SET user_id=excluded.user_id,secret=excluded.secret,expires=excluded.expires,verified=0,label=excluded.label`, session, user.Id, secret, time.Now().Add(5*time.Minute).Unix(), label)
	if err != nil {
		return nil, err
	}
	return &MfaProps{MfaType: TotpType, Secret: key.Secret(), URL: key.URL()}, nil
}
func VerifyMfaEnrolment(user *User, session, code string) error {
	rows, err := ormer.Engine.QueryString("SELECT secret FROM oneaccount_mfa_pending WHERE session_id=? AND user_id=? AND expires>?", session, user.Id, time.Now().Unix())
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("setup expired; start again")
	}
	secret, err := DecryptMfaSecret(rows[0]["secret"], user.Id)
	if err != nil {
		return err
	}
	if err = MfaAttempt(user); err != nil {
		return err
	}
	temp := *user
	temp.TotpSecret = rows[0]["secret"]
	if err = VerifyTotpOnce(&temp, code, ""); err != nil {
		return err
	}
	_ = secret // Decryption was validated before the atomic replay check.
	_, err = ormer.Engine.Exec("UPDATE oneaccount_mfa_pending SET verified=1 WHERE session_id=? AND user_id=? AND expires>?", session, user.Id, time.Now().Unix())
	return err
}
func ConfirmMfaEnrolment(user *User, session string) ([]string, error) {
	raw, digests, err := NewRecoveryCodes(user)
	if err != nil {
		return nil, err
	}
	tx := ormer.Engine.NewSession()
	defer tx.Close()
	if err = tx.Begin(); err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryString("SELECT secret,label FROM oneaccount_mfa_pending WHERE session_id=? AND user_id=? AND expires>? AND verified=1", session, user.Id, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, errors.New("verify the pending setup code first")
	}
	r, err := tx.Exec("DELETE FROM oneaccount_mfa_pending WHERE session_id=? AND user_id=? AND verified=1", session, user.Id)
	if err != nil {
		return nil, err
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return nil, errors.New("setup already consumed")
	}
	user.TotpSecret = rows[0]["secret"]
	user.RecoveryCodes = digests
	user.PreferredMfaType = TotpType
	n, err = tx.Where("owner=? AND name=? AND totp_secret=''", user.Owner, user.Name).Cols("totp_secret", "recovery_codes", "preferred_mfa_type").Update(user)
	if err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, errors.New("enrolment state changed")
	}
	if _, err = tx.Where("organization=? AND "+quoteColumn("user")+"=?", user.Owner, user.Name).Cols("expires_in").Update(&Token{ExpiresIn: 0}); err != nil {
		return nil, err
	}
	if _, err = tx.Exec("INSERT INTO oneaccount_mfa_device(user_id,label,enrolled) VALUES(?,?,?) ON CONFLICT(user_id) DO UPDATE SET label=excluded.label,enrolled=excluded.enrolled,last_used=0", user.Id, rows[0]["label"], time.Now().Unix()); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if err = MfaAudit(user, "enrolled"); err != nil {
		return nil, err
	}
	return raw, nil
}
