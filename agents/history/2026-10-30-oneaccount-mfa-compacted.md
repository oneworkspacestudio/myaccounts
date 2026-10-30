# OneAccount MFA and independent One Authenticator

User authorized implementation, push, preview release and managed installation.
Isolated branch codex/one-authenticator starts f631efc. Original checkout has
unrelated Linux work; preserve it. No GitHub Actions. No owner fixture data.

New object/controllers oneaccount_mfa layer uses existing SQLite and user IDs.
Seeds m1 AESGCM/account AAD, key separate secrets/oneaccount-mfa.dpapi; startup
migrates legacy plaintext and fails closed for missing key with active seeds.
Encrypted portable backup roundtrips the envelope key inside age and rewraps
DPAPI at restore. Ordinary user JSON omits seed/recovery fields.
Pending five-minute server seeds, verified activation CAS, ten128-bit recovery
codes with h1 account HMAC digests, normalized atomic single-use and OTP monotonic
counter, persistent10 attempts/5min buckets. Challenge random/expiring/user+app
bound/atomic. Session proof binds session and credential epoch. Central mint,
refresh, API and introspection enforce proof, trusted-device bypass removed for
TOTP. Group-required unenrolled user cannot mint JWT. Noninteractive user grants
fail closed for enrolled TOTP. Activation/regeneration expire prior tokens.
Disable transaction removes credential/session/proof and expires tokens before
Beego cache/session-cookie removal. Five-minute recent MFA context plus fresh
password enables security actions even with the last recovery code; codes remain
single-use. Regeneration CAS preserves original recent-MFA timestamp.

AUI-authored security UI has QR/manual setup, confirmation, ten codes download/
print, regeneration/removal/device/events. Build compiles AUI via pinned parser.
Read docs/preview-20261030.1.md for scope and limitations.

PASS final native build/test/vet + frozen frontend; focused encryption/JSON/vector
checks; native adversarial MFA/OIDC/PKCE, old session/token/refresh denial, password
grant denial, invalid/expired/concurrent replay, normalized recovery, regeneration,
rate limits across IP/challenge/flow, disable and encrypted backup/restore. Actual
browser settings provisioned independent encrypted PWA, generated offline code
with host stopped and enabled MFA, then authenticated removal. Test-only SQLite
expiry/window edits model time only in fresh synthetic fixtures. Whole combined
legacy smoke is not claimed. Physical phone/reboot/clean VM remain untested.

Publication pending: prospective preview-20261030.1 with minimum Workspace109
and capability oneaccount-mfa-v1. Paired manager enforces provider introspection
on cached portal sessions and proxied streams. Do not distribute to an old manager.
Canonical app I:/Projects/one-authenticator; task builds/evidence in sibling
one-authenticator-20261030 tooling. Only named C task scratch is disposable.
Owner C:/tmp/workspace must only be updated through backed supported path after
managed fresh/upgrade/recovery and public release/channel evidence.

Provider preview-20261030.1 public with exact executable eb941eaf... and source
926137b. Authenticator preview2 fixes manual-save camera cleanup; all frozen PWA
crypto/offline/QR/manual cases and SW1→2 encrypted-vault preservation pass. Actual
35-app managed fresh install and108→109 protected upgrade passed central session
revocation, OTP access to Drive/Docs/Photos, rollback/reapply and original-password
SSO. Suite109 signed publication is in progress; owner installation remains pending
until immutable asset/public-channel verification. No owner fixture or enrollment.
Recent-password server proof lasts five minutes (UI asks on each action); it is
not a one-use password nonce. Synthetic resume must retain the actual test owner
password: Suite Secrets.AdminPassword is a separate service credential.
Go runtime license supplement added; original published assets remain immutable.
