package controllers

import (
	"context"
	"errors"
	"time"

	"github.com/casdoor/casdoor/object"
	"github.com/casdoor/casdoor/util"
)

// This authenticated response contains only a generic retry window, no account,
// credential or internal error detail. Preserve ordinary error masking otherwise.
func (c *ApiController) oneAccountMfaError(err error) {
	if errors.Is(err, object.ErrMfaRateLimited) {
		c.ResponseJsonData(&Response{Status: "error", Code: "mfa_rate_limited", Msg: "Too many verification attempts. Wait up to five minutes before trying again."})
		return
	}
	c.ResponseError(err.Error())
}

func (c *ApiController) oneAccountMfaUser() *object.User {
	c.Ctx.Output.Header("Cache-Control", "no-store")
	user := c.getCurrentUser()
	target := util.GetId(c.Ctx.Request.Form.Get("owner"), c.Ctx.Request.Form.Get("name"))
	if user == nil || user.GetId() != target {
		c.ResponseError("Sign in as this account to manage its authenticator")
		return nil
	}
	return user
}
func (c *ApiController) oneAccountMfaRecent(user *object.User) bool {
	id, _ := c.GetSession("oneAccountPasswordProof").(string)
	stamp, _ := c.GetSession("oneAccountPasswordTime").(int64)
	if id != user.Id || stamp > time.Now().Unix() || time.Now().Unix()-stamp > 300 {
		c.ResponseError("Verify your current password again before changing two-factor authentication")
		return false
	}
	return true
}
func (c *ApiController) oneAccountMfaBegin() {
	user := c.oneAccountMfaUser()
	if user == nil {
		return
	}
	session := c.Ctx.Input.CruSession.SessionID(context.Background())
	switch c.Ctx.Request.Form.Get("action") {
	case "status":
		status, err := object.MfaStatus(user)
		if err != nil {
			c.ResponseError("Security status unavailable")
			return
		}
		c.ResponseOk(status)
		return
	case "cancel":
		if err := object.CancelMfaEnrolment(user, session); err != nil {
			c.ResponseError("Cancellation failed")
			return
		}
		c.ResponseOk()
		return
	case "regenerate":
		if !c.oneAccountMfaDestructiveProof() {
			return
		}
		codes, err := object.RegenerateMfaRecovery(user, session)
		if err != nil {
			c.ResponseError("Recovery codes could not be regenerated")
			return
		}
		c.ResponseOk(map[string]interface{}{"recoveryCodes": codes})
		return
	}
	if !c.oneAccountMfaRecent(user) {
		return
	}
	props, err := object.BeginMfaEnrolment(user, c.Ctx.Input.CruSession.SessionID(context.Background()), c.Ctx.Request.Form.Get("deviceLabel"))
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	c.ResponseOk(props)
}
func (c *ApiController) oneAccountMfaVerify() {
	user := c.oneAccountMfaUser()
	if user == nil || !c.oneAccountMfaRecent(user) {
		return
	}
	err := object.VerifyMfaEnrolment(user, c.Ctx.Input.CruSession.SessionID(context.Background()), c.Ctx.Request.Form.Get("passcode"))
	if err != nil {
		c.oneAccountMfaError(err)
		return
	}
	c.ResponseOk()
}
func (c *ApiController) oneAccountMfaEnable() {
	user := c.oneAccountMfaUser()
	if user == nil || !c.oneAccountMfaRecent(user) {
		return
	}
	session := c.Ctx.Input.CruSession.SessionID(context.Background())
	codes, err := object.ConfirmMfaEnrolment(user, session)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	if err = object.GrantMfaSession(user, session); err != nil {
		c.ResponseError("Enabled; sign in again using your authenticator")
		return
	}
	c.DelSession("oneAccountPasswordProof")
	c.DelSession("oneAccountPasswordTime")
	c.ResponseOk(map[string]interface{}{"recoveryCodes": codes, "enabled": true})
}
func (c *ApiController) oneAccountMfaDestructiveProof() bool {
	user := c.oneAccountMfaUser()
	if user == nil || !c.oneAccountMfaRecent(user) {
		return false
	}
	if user.TotpSecret != "" {
		session := c.Ctx.Input.CruSession.SessionID(context.Background())
		var err error
		if c.Ctx.Request.Form.Get("passcode") == "" && c.Ctx.Request.Form.Get("recoveryCode") == "" {
			err = object.UseRecentMfaStepup(user, session)
		} else {
			err = object.VerifyMfaWithLimit(user, func() error {
				if recovery := c.Ctx.Request.Form.Get("recoveryCode"); recovery != "" {
					return object.MfaRecover(user, recovery)
				}
				return object.VerifyTotpOnce(user, c.Ctx.Request.Form.Get("passcode"), c.GetAcceptLanguage())
			}, c.GetAcceptLanguage())
		}
		if err != nil {
			c.ResponseError("Current authenticator or recovery proof required")
			return false
		}
	}
	c.DelSession("oneAccountPasswordProof")
	c.DelSession("oneAccountPasswordTime")
	return true
}
