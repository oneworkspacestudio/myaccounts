// Copyright 2023 The Casdoor Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package object

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/casdoor/casdoor/util"
)

type MfaProps struct {
	Enabled            bool     `json:"enabled"`
	IsPreferred        bool     `json:"isPreferred"`
	MfaType            string   `json:"mfaType" form:"mfaType"`
	Secret             string   `json:"secret,omitempty"`
	CountryCode        string   `json:"countryCode,omitempty"`
	URL                string   `json:"url,omitempty"`
	RecoveryCodes      []string `json:"recoveryCodes,omitempty"`
	MfaRememberInHours int      `json:"mfaRememberInHours"`
}

type MfaInterface interface {
	Initiate(userId string, issuer string) (*MfaProps, error)
	SetupVerify(passcode string, lang string) error
	Enable(user *User) error
	Verify(passcode string, lang string) error
}

const (
	EmailType  = "email"
	SmsType    = "sms"
	TotpType   = "app"
	RadiusType = "radius"
	PushType   = "push"
)

const (
	MfaSessionUserId = "MfaSessionUserId"
	NextMfa          = "NextMfa"
	RequiredMfa      = "RequiredMfa"
)

func GetMfaUtil(mfaType string, config *MfaProps) MfaInterface {
	switch mfaType {
	case SmsType:
		return NewSmsMfaUtil(config)
	case EmailType:
		return NewEmailMfaUtil(config)
	case TotpType:
		return NewTotpMfaUtil(config)
	case RadiusType:
		return NewRadiusMfaUtil(config)
	case PushType:
		return NewPushMfaUtil(config)
	}

	return nil
}

// mfaErrorKey counts the failed MFA passcodes and recovery codes of a user apart from the
// password failures, which a correct password resets before every new MFA attempt
const mfaErrorKey = "mfa"

func VerifyMfaWithLimit(user *User, verify func() error, lang string) error {
	if err := MfaAttempt(user); err != nil {
		return err
	}
	err := checkVerifyCodeErrorTimes(user, mfaErrorKey, lang)
	if err != nil {
		return err
	}

	err = verify()
	if err != nil {
		_ = MfaAudit(user, "verification_failed")
		if limitErr := recordVerifyCodeErrorInfo(user, mfaErrorKey, lang); limitErr != nil {
			return fmt.Errorf("%s, %s", err.Error(), limitErr.Error())
		}
		return err
	}

	resetVerifyCodeErrorTimes(user, mfaErrorKey)
	return nil
}

func MfaRecover(user *User, recoveryCode string) error {
	hit := false

	if len(user.RecoveryCodes) == 0 {
		return fmt.Errorf("do not have recovery codes")
	}

	for _, code := range user.RecoveryCodes {
		if subtle.ConstantTimeCompare([]byte(code), []byte(RecoveryDigest(user, recoveryCode))) == 1 {
			hit = true
			user.RecoveryCodes = util.DeleteVal(user.RecoveryCodes, code)
			break
		}
	}
	if !hit {
		return fmt.Errorf("recovery code not found")
	}
	if err := consumeRecoveryCode(user, recoveryCode); err != nil {
		return err
	}

	_, err := UpdateUser(user.GetId(), user, []string{"recovery_codes"}, false)
	if err != nil {
		return err
	}

	return MfaAudit(user, "recovery_code_used")
}

func GetAllMfaProps(user *User, masked bool) []*MfaProps {
	mfaProps := []*MfaProps{}

	for _, mfaType := range []string{SmsType, EmailType, TotpType, RadiusType, PushType} {
		mfaProps = append(mfaProps, user.GetMfaProps(mfaType, masked))
	}
	return mfaProps
}

func (user *User) GetMfaProps(mfaType string, masked bool) *MfaProps {
	mfaProps := &MfaProps{}

	if mfaType == SmsType {
		if !user.MfaPhoneEnabled {
			return &MfaProps{
				Enabled: false,
				MfaType: mfaType,
			}
		}

		mfaProps = &MfaProps{
			Enabled:     user.MfaPhoneEnabled,
			MfaType:     mfaType,
			CountryCode: user.CountryCode,
		}
		if masked {
			mfaProps.Secret = util.GetMaskedPhone(user.Phone)
		} else {
			mfaProps.Secret = user.Phone
		}
	} else if mfaType == EmailType {
		if !user.MfaEmailEnabled {
			return &MfaProps{
				Enabled: false,
				MfaType: mfaType,
			}
		}

		mfaProps = &MfaProps{
			Enabled: user.MfaEmailEnabled,
			MfaType: mfaType,
		}
		if masked {
			mfaProps.Secret = util.GetMaskedEmail(user.Email)
		} else {
			mfaProps.Secret = user.Email
		}
	} else if mfaType == TotpType {
		if user.TotpSecret == "" {
			return &MfaProps{
				Enabled: false,
				MfaType: mfaType,
			}
		}

		mfaProps = &MfaProps{
			Enabled: true,
			MfaType: mfaType,
		}
		if masked {
			mfaProps.Secret = ""
		} else {
			mfaProps.Secret, _ = DecryptMfaSecret(user.TotpSecret, user.Id)
		}
	} else if mfaType == RadiusType {
		if !user.MfaRadiusEnabled {
			return &MfaProps{
				Enabled: false,
				MfaType: mfaType,
			}
		}

		mfaProps = &MfaProps{
			Enabled: user.MfaRadiusEnabled,
			MfaType: mfaType,
		}
		if masked {
			mfaProps.Secret = util.GetMaskedEmail(user.MfaRadiusUsername)
		} else {
			mfaProps.Secret = user.MfaRadiusUsername
		}
		mfaProps.URL = user.MfaRadiusProvider
	} else if mfaType == PushType {
		if !user.MfaPushEnabled {
			return &MfaProps{
				Enabled: false,
				MfaType: mfaType,
			}
		}

		mfaProps = &MfaProps{
			Enabled: user.MfaPushEnabled,
			MfaType: mfaType,
		}
		if masked {
			mfaProps.Secret = util.GetMaskedEmail(user.MfaPushReceiver)
		} else {
			mfaProps.Secret = user.MfaPushReceiver
		}
		mfaProps.URL = user.MfaPushProvider
	}

	if user.PreferredMfaType == mfaType {
		mfaProps.IsPreferred = true
	}
	return mfaProps
}

func DisabledMultiFactorAuth(user *User) error {
	oldSecret := user.TotpSecret
	sessions, err := GetUserSessions(user.Owner, user.Name)
	if err != nil {
		return err
	}
	ids := []string{}
	for _, s := range sessions {
		ids = append(ids, s.SessionId...)
	}
	user.PreferredMfaType = ""
	user.RecoveryCodes = []string{}
	user.MfaPhoneEnabled = false
	user.MfaEmailEnabled = false
	user.TotpSecret = ""
	user.MfaRadiusEnabled = false
	user.MfaRadiusUsername = ""
	user.MfaRadiusProvider = ""
	user.MfaPushEnabled = false
	user.MfaPushReceiver = ""
	user.MfaPushProvider = ""
	tx := ormer.Engine.NewSession()
	defer tx.Close()
	if err = tx.Begin(); err != nil {
		return err
	}
	defer tx.Rollback()
	if n, e := tx.Where("owner=? AND name=? AND totp_secret=?", user.Owner, user.Name, oldSecret).Cols("preferred_mfa_type", "recovery_codes", "mfa_phone_enabled", "mfa_email_enabled", "totp_secret", "mfa_radius_enabled", "mfa_radius_username", "mfa_radius_provider", "mfa_push_enabled", "mfa_push_receiver", "mfa_push_provider").Update(user); e != nil {
		return e
	} else if n != 1 {
		return fmt.Errorf("credential changed; sign in again")
	}
	if _, err = tx.Where("organization=? AND "+quoteColumn("user")+"=?", user.Owner, user.Name).Cols("expires_in").Update(&Token{ExpiresIn: 0}); err != nil {
		return err
	}
	if _, err = tx.Where("owner=? AND name=?", user.Owner, user.Name).Delete(&Session{}); err != nil {
		return err
	}
	for _, table := range []string{"oneaccount_mfa_proof", "oneaccount_mfa_pending", "oneaccount_mfa_device"} {
		if _, err = tx.Exec("DELETE FROM "+table+" WHERE user_id=?", user.Id); err != nil {
			return err
		}
	}
	if _, err = tx.Exec("INSERT INTO oneaccount_mfa_audit(user_id,event,created) VALUES(?,?,?)", user.Id, "disabled", time.Now().Unix()); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	DeleteBeegoSession(ids)
	return nil
}

func SetPreferredMultiFactorAuth(user *User, mfaType string) error {
	user.PreferredMfaType = mfaType

	_, err := UpdateUser(user.GetId(), user, []string{"preferred_mfa_type"}, false)
	if err != nil {
		return err
	}
	return nil
}
