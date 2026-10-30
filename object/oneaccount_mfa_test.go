package object

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestOneAccountCredentialEncryptionAndBinding(t *testing.T) {
	previous := mfaKey
	mfaKey = bytes.Repeat([]byte{0x31}, 32)
	defer func() { mfaKey = previous }()
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	ciphertext, err := EncryptMfaSecret(secret, "synthetic-a")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ciphertext, secret) {
		t.Fatal("seed stored in plaintext")
	}
	plain, err := DecryptMfaSecret(ciphertext, "synthetic-a")
	if err != nil || plain != secret {
		t.Fatal("round trip failed")
	}
	if _, err = DecryptMfaSecret(ciphertext, "synthetic-b"); err == nil {
		t.Fatal("cross-account ciphertext accepted")
	}
	if _, err = DecryptMfaSecret(ciphertext[:len(ciphertext)-4]+"AAAA", "synthetic-a"); err == nil {
		t.Fatal("tampering accepted")
	}
	user := &User{Id: "synthetic-a", TotpSecret: ciphertext}
	raw, hashes, err := NewRecoveryCodes(user)
	if err != nil || len(raw) != 10 || len(hashes) != 10 {
		t.Fatal("ten recovery codes required")
	}
	seen := map[string]bool{}
	for i, code := range raw {
		if seen[code] || len(strings.ReplaceAll(code, "-", "")) != 32 || hashes[i] != RecoveryDigest(user, code) || hashes[i] == code {
			t.Fatal("invalid recovery generation")
		}
		seen[code] = true
	}
	user.RecoveryCodes = hashes
	b, _ := json.Marshal(user)
	if strings.Contains(string(b), ciphertext) || strings.Contains(string(b), hashes[0]) {
		t.Fatal("ordinary account API exposes credentials")
	}
}
func TestOneAccountRFC6238SHA1(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	for _, v := range []struct {
		seconds int64
		code    string
	}{{59, "94287082"}, {1111111109, "07081804"}, {1111111111, "14050471"}, {1234567890, "89005924"}, {2000000000, "69279037"}, {20000000000, "65353130"}} {
		got, err := totp.GenerateCodeCustom(secret, time.Unix(v.seconds, 0), totp.ValidateOpts{Period: 30, Digits: otp.DigitsEight, Algorithm: otp.AlgorithmSHA1})
		if err != nil || got != v.code {
			t.Fatalf("RFC vector %d: %s %v", v.seconds, got, err)
		}
	}
}
