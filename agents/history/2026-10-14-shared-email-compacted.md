# Shared email continuation

Latest steering: sending account/email delivery must be optional; normal admin
CRUD and direct password changes must work without it. Implemented and installed.
The latest update at the end supersedes earlier runtime/version/owner-setup notes.

## Requirements and boundary

User authorized implementing a shared email service and setup wizard. Normal
installations should provide only their administrator email for email setup,
then prove ownership with an emailed code. Service URL is built in; a distinct
installation credential is generated automatically. Keep Cloudflare tunnelling.
Only the shared service owner configures the sending account once. No sending
account credentials supplied in this turn. Optional provider-choice question is
pending; the owner wizard supports Gmail and custom implicit-TLS SMTP.

Authoritative source remains `H:/Projects/myaccounts`, Git head `fcccd2a`; new work
is uncommitted and unpublished. Earlier `preview-20261014.1` remains unchanged and
does not include shared email. Follow `2026-10-14-public-install-filebrowser-compacted.md`
for installed identity/FileBrowser paths and original release/incident context.

## Implemented source

- `internal/mailrelay`: HTTPS enrollment/confirmation, hashed independent tokens,
  fixed code-only templates, SQLite persistent quotas, bounded attempts/expiry,
  message-ID/payload deduplication, per-installation revocation and client with
  redirect refusal. No arbitrary subjects/HTML/from/links/bulk mail.
- `cmd/myaccounts-mail` + `internal/mailservice`: native shared service, protected
  local owner wizard, Gmail/custom TLS465 sending account, actual inbox-code
  confirmation before saving, optional explicitly authorized dedicated Cloudflare
  provisioning/existing tunnel/manual HTTPS. DPAPI configuration/NTFS ACLs,
  protected data picker, separate connector and loopback origin with public Host
  check. Public default `https://mail.m8e.co.uk`; not provisioned/activated yet.
- Installer has administrator/connection/email/finish steps; email mode is shared,
  later, or advanced custom provider. Credential stays in DPAPI, bootstrap omits
  email code, explicit nonsecret sharedEmail flag avoids enabling skipped email.
- Existing global-admin operations page can open a token-protected local
  reconnect wizard and apply recovery immediately; CLI `email-setup` needs restart.
  Reconnection saves replacement before attempting old-token revocation.
- Managed provider is `admin/myaccounts-email`, type `MyAccounts Relay`, and only
  that exact provider can invoke the protected installation credential. Attach to
  default built-in/myaccounts apps and expose recovery without enabling signup;
  leave other client apps/custom mail providers alone. Apply marker prevents
  undoing later admin choices each boot. Existing atomic expiring reset stays.
- Encrypted backup converts the relay credential inside the age archive and
  re-protects it on restore; omit transient setup URLs.
- Build/CI include both deployable EXEs, checksums, UI fixture and native relay
  fixture. Documentation `docs/shared-email.md`; existing release gates stay false.

## Verification

Frontend Vite production build and desktop/mobile wizard fixture passed; reviewed
screenshots in ignored `dist/setup-review`. Fixed stale validation text after
successful connection. Browser setup/operations scripts parse.

Latest focused native tests passed: mailrelay, mailservice (owner token/Origin/Host,
code proof, exact tested config and DPAPI), portable (wizard proof, storage,
credential rotation/revocation, backup/restore), cred, command compile. A missing
`time` import and a missing disposable F: test directory were corrected before
the passing run. Root package tests, vet and full executable build subsequently passed. Full native
regression passed OIDC, MFA, operations, groups, sessions, actual Chrome passkeys
and consent, schedules, TLS relay signup/reset/expiry/concurrency, and native
encrypted restore. Final exact-mailbox binding received a focused unit/UI/native
rerun. The first runtime assertion incorrectly required the built-in realm
disableSelfSignup flag to be true; corrected against the unchanged baseline
(enableSignUp=false; normal realm also disableSelfSignup=true). No registration
policy was opened.

## Final runtime and artifacts

Identity version `shared-email-20261014-dev`, installed and source artifact SHA256
`7baa18cbd08fcdb4838503f4e5ef87b3c7616828076d5d96b618b7eb6a220d7d`.
Active `F:/MyAccounts/install/bin/MyAccounts.exe`; same data
`F:/MyAccounts/install/data`; loopback18888 and public issuer
`https://login.m8e.co.uk`. Health/public discovery/one signing key verified after
final promotion. Previous verified tagged EXE retained as
`MyAccounts-before-shared-email-78957576b48b.exe`; intermediate local build archived
under ignored H: dist to avoid using more F: space. Original older previous EXE
also retained. Automatic encrypted upgrade snapshots remain in runtime backups.

Mail executable `dist/MyAccounts-Mail-windows-amd64.exe`, copied to
`F:/MyAccounts/install/bin/MyAccounts-Mail.exe`, SHA256
`379799e1e1053231d0c99706db0ae5fb59ca2cfaf3424a1f112546633b9e2fc1`.
Data `F:/MyAccounts/mail-service`, loopback19890; owner wizard process70904 at this
handoff, still no service.dpapi. Protected wizard URL only in
`secrets/owner-setup-url.dpapi`; newest browser tab was queued in Codex. Earlier
setup tab was refreshed after the mailbox-case correction. No sending credentials
supplied, no public shared gateway activated. User must complete owner setup,
then connect email in the existing identity installation. Ordinary installers
enter email and verification code only, never provider settings.

Both actual FileBrowser browser flows and saved identifier tests passed against
the first new build and were repeated successfully against the final exact
installed artifact. Local/remote callbacks, matching subject, normal permissions,
other-admin refusal, HttpOnly/Secure cookies and saved-identifier removal passed.
Private browser profiles/logs remain under the existing protected install private
folder; no personal email, password, cookies or remote preview URL in this file.
Latest native fixture roots are under F:/MyAccounts/tests. Disposable TLS relay
process58100 was stopped after passing; no certificate-store changes or TLS
verification bypass. Private test CA was supplied only to a fresh child and the
script restores its environment. No test mail left the loopback sink.

## Build findings and resume

Use Go `I:/Projects/tooling/go1.27.2/go/bin/go.exe`, completed module cache
`H:/Projects/tooling/myaccounts/go-modcache`, build cache sibling go-cache,
temp `I:/Projects/tooling/tmp`, and MYACCOUNTS_TEST_DATA `F:/MyAccounts/tests`.
For this large dependency tree on exFAT, GODEBUG=goindex=0,http2client=0 avoids
indexing whole unused SDK modules. Four build processes/4GiB memory were used.
Pinned dependencies/go.sum unchanged; -mod=readonly; final H: builds used GOPROXY=off.
A diagnostic I: module-cache attempt has an incomplete Alibaba SDK v1.63.107
extraction (the .partial marker prevents using it); don't mistake cleanup/unzip
for compiler inactivity or repeatedly restart it. Prefer the completed H: cache.
No project/build cache was intentionally placed on C:. D:/G: are cloud drives;
do not use them for caches/secrets. F: has limited free space; put binaries for
historical builds on H: and live protected data/executables on F:.

Source and CI include `integration/mailrelay-fixture`: test-only TLS mail sink,
private CA and verified DPAPI installation credential. Never deploy this sink.
CI runs real native signup/reset through the shared HTTPS protocol. Only the two
deployable executables are release assets; the fixture executable is excluded.

No new commit/push/tag/release or agent-driven Cloudflare mutation in this
continuation. Source on main is dirty by design; new feature is not in the old
published preview. Implementation, exact-byte browser verification, checksums and handoff are
complete. Only service-owner configuration/public mail activation needs user
input; report this plainly. Current identity launcher PID63768 at handoff.
Do not claim real SMTP authentication/inbox delivery, public gateway readiness,
mail service automatic startup or original full-production gates passed.


## Latest update: optional email and administration

User explicitly clarified that the sending account must be optional and asked for
an admin panel to add/remove/edit users and change passwords. Do not request SMTP
credentials or imply they are prerequisites. The pending provider question above
is superseded. Keep Cloudflare tunnelling independent of email.

Implemented:
- Identity installer defaults to `emailMode=none`, labelled Skip email for now.
- Owner wizard has an authenticated Skip email for now action. Clears pending
  proof, returns ErrSkipped, removes transient protected URL and exits native mail
  command with success. No mail config, relay DB or tunnel created on skip.
- `/operations` now titled Administration, with Manage users linking to
  `/organizations/myaccounts/users`, direct admin-profile link and optional-email
  explanation. Existing Casdoor user forms already implement CRUD/direct reset;
  reuse these. New-user default password is empty/plain rather than upstream123.
- Guide/README/acceptance/CI updated; no new commit/push/release.

Verification: mailservice/portable tests and command compile; TypeScript and Vite
production build; desktop/mobile wizard fixture including both no-email skips;
exact native mail command successful skip with no config/tunnel/relay. Fresh native
no-email fixture passed real Chrome admin-panel link, create form, edit profile,
password modal without old password/code, and delete confirmation. API regression
passed unverified-user login, direct reset, old-password/session refusal, ordinary
user admin/create/reset denial, deleted-user login refusal and built-in admin
removal refusal. Base native smoke passed bootstrap/health/discovery/duplicate
instance/clean stop/encrypted backup/restore/signer preservation. New test is
`integration/user-admin.mjs`, invoked by smoke `-OIDC -Checks user-admin`; optional
MYACCOUNTS_PLAYWRIGHT runs browser form checks. CI sets this for a separate no-email
native fixture; CI has not been triggered for this unpublished source.

Current installed identity version `optional-email-admin-20261014-dev`:
`dist/MyAccounts-optional-email-preview.exe` -> protected installed MyAccounts.exe,
SHA256 a7091ef7c5f279e678d6083a95dbcb92a880ea202932cf6e22e831b044bfb5cb.
Current mail artifact `dist/MyAccounts-Mail-optional-email.exe` -> installed
MyAccounts-Mail.exe, SHA256
f2b3f39175fc5f11b889a360da46764e5d8734545cc940001bbdf27cf5b48a3f.
Manifest `dist/optional-email-SHA256SUMS.txt`. Prior shared-email binaries retained
under ignored H: dist as MyAccounts-before-optional-email.exe and
MyAccounts-Mail-before-optional-email.exe; older tagged rollback remains on F:.

Identity launcher PID76808 at handoff, same F data,18888/public login.m8e.co.uk.
After promotion, existing administrator login and new Manage users/optional email
HTML passed. SharedEmail remains false. Public HTTPS health and signing keys
unchanged; no user's password changed. Latest exact-byte FileBrowser browser SSO
was on prior shared-email build; this UI/optional-mail change did not repeat that
external suite. Both integrations remain configured.

Owner wizard PID21560 at handoff,19890, same protected mail-service data;
service.dpapi still absent. Refreshed private setup URL opened through Codex after
restart. Old tabs have stale tokens; use newest protected owner-setup-url.dpapi.
User can skip or close; running mail wizard is optional. Mail not sending/public.
No credentials requested or supplied. Private fixtures remain under F tests.
F remaining free was about207MiB before final promotion; avoid redundant binaries
there, use H for sources/build/history artifacts. Public/full-production gates
remain false. No service installation/elevation/reboot performed.


## Authorized GitHub publication continuation

User explicitly requested pushing the implementation, releasing the executable,
and expanding README with pictures, custom FileBrowser GitHub link/integration,
and other-app integration. This authorizes commit/push/tag/prerelease publication.
Preparing `preview-20261014.2`; stable v0.1.0 remains blocked by existing gates.

README rewritten with download/asset roles, setup, optional email, account admin,
custom FileBrowser registration/YAML/source permissions, generic OIDC setting table,
Gitea and runnable Node/Python examples. Detailed `docs/filebrowser.md` added;
links verified to public fahimc/filebrowser, branch drive-reskin, release
 drive-office-v0.4.0. Companion FileBrowser origin is upstream; `fork` is fahimc.
Its local SSO helper commit remains unpublished; no FileBrowser source was pushed.
Do not link unpublished companion MYACCOUNTS.md as a GitHub page.

Eight screenshots under docs/screenshots use only synthetic accounts/documents.
`integration/docs-screenshots.mjs`, called by native smoke `-OIDC -Checks docs`,
creates disposable users and drives actual custom FileBrowser SSO for pictures.
Set MYACCOUNTS_FILEBROWSER_EXE to custom v0.4.0 exe to reproduce. Initial capture
selector matched avatar/name twice; scoped it to the user edit link. Another
capture changed a fixture admin's display name and tripped the existing restore
assertion; capture now restores that name before harness backup. Final complete
capture + native backup/restore smoke passed. This was fixture-only, no live data.
Reviewed all images: no clipping, real credentials or user documents. Demo UUIDs
and test-directory paths visible in images belong only to disposable fixtures.

Local release checks: mailrelay/mailservice/portable/cred/root/command tests passed;
go vet passed. Previous TypeScript/Vite and real Chrome no-email admin CRUD/reset
remain applicable; frontend unchanged in this publication turn. Documentation
local links resolved. Release notes and deployment email text updated. Final
GitHub CI/release confirmation must be recorded after publishing; do not infer it
from this preparation note. Live optional-email runtime remains unchanged.
