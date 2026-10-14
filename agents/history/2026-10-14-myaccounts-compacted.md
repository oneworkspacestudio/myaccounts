# MyAccounts implementation handoff — 2026-10-14

## Scope and current state

User requested a new project outside C:, implementation of the attached identity/SSO brief, and the brief in the repository. Authoritative checkout is `H:/Projects/myaccounts`; initial `I:/Projects/myaccounts` is an older copy. E: transfer failed source checksums and was abandoned. H: source transfer matched all 1,308 original files. H: is exFAT; live data must use a filesystem with persistent Windows ACLs. Disposable runtime tests use F: NTFS. No live identity deployment created.

Original brief is preserved verbatim at repository root. Development implementation is substantial but the complete brief is unfinished. `docs/acceptance.md` and `docs/release-acceptance.json` are the authoritative evidence ledger; release remains blocked. GitHub private repository creation/push authorization was requested but not received. Local implementation milestone committed; no remote, CI run, tag, push or release. Cloudflare credentials/test zone were not supplied; no cloud resources created.

## Decisions and implementation

Casdoor source pinned from commit `2e1127cf56b1ecba1863163007af0e04a6f554c8`, Apache-2.0 notices retained. Original Go module path retained for imports. One native Windows AMD64 EXE embeds Vite frontend, runs itself as the identity child, and uses modernc SQLite 1.60.1 without CGO. Go 1.27.2, Node 26.8.2, Yarn 1.22.22 build tools. Runtime requires none of these.

Launcher provides protected localhost wizard, Argon2id/bootstrap, original branding, native NTFS folder picker, persistent data selection, DPAPI machine credentials, full DACL replacement for existing data children, locks, loopback orderly shutdown, SQLite WAL/busy timeout/checkpoint, rotating logs, service commands, age encrypted offline backup/restore, issuer verification, update-check and Cloudflare provisioning/connector management. Default port 8788 avoids the unrelated existing 8787 service. Signing/SMTP/MFA secrets in SQLite rely on ACLs; database is not encrypted. No default LDAP/RADIUS listeners or default admin password. Signup starts disabled pending actual SMTP configuration/test.

Engine middleware normalizes configured opaque session cookie and checks browser mutation Origin/Host. Persistent SQLite TOTP counters and recovery-use markers reject replay/concurrent reuse, using upstream OTP libraries. Official connector downloads require GitHub asset SHA256; token uses environment, not CLI. Cloud API refuses existing DNS and creates only a fresh dedicated tunnel after explicit wizard authorization. Scheduled backup, automatic migration rollback, app discovery and requested custom administration/chooser UX remain incomplete.

## Verified evidence

Frontend production build passed (3,511 modules, translation chunk warning). Focused `go test ./internal/portable ./cred .` and `go vet ./internal/portable .` passed, including explicit Everyone ACL removal, real SQLite, DPAPI, archive adversarial cases, mocked Cloudflare and setup request boundaries. Root cookie tests pass.

Native `scripts/smoke.ps1 -OIDC` verifies bootstrap removal, password/session/profile, failed credentials, discovery/JWKS, duplicate refusal, clean stop, encrypted backup/restore byte equality, restored identity login and signing keys. Independent openid-client 6.8.8 verifies code/PKCE, JWKS signature, state/nonce, UserInfo, bad redirects/verifier, replay and secret rotation. Independent otpauth 9.5.2 verifies enrollment/challenge/TOTP replay and simultaneous recovery requests with exactly one success.

Real native FileBrowser drive-office-v0.3.1 (Quantum beta2.1.2 derivative, commit `72fc7547d0b099864f2957e26e745cc1ff6facbc`) and official Gitea 28.1.0 passed disposable OIDC callbacks, stable-sub linkage and normal-user permissions. Gitea API excludes browser session auth; check signed-in settings and forbidden admin page instead. Test code in integration/, executable paths supplied by environment. Existing FileBrowser checkout/configuration was not edited.

govulncheck 1.9.0 originally found eight reachable advisories; dependency patches reduced repeat scan to zero reachable, with four imported-package and three module advisories still nonreachable. This does not clear all vulnerabilities. CycloneDX 1.6 SBOM generated, 303 components; two library license-detection warnings require manual review. Native EXE, SBOM and checksums live in ignored dist/. Official connector download through native CLI passed digest verification; cloudflared 2026.10.0 runs. Actual tunnel remains untested. Setup desktop/mobile PNGs visually reviewed.

## Introduced host-side incident

First isolated Gitea migration wrote an empty `%USERPROFILE%/.ssh/authorized_keys` on 2026-10-14 because DISABLE_SSH alone does not disable key-file rewriting. No pre-state captured or automatic backup found: possible loss of existing keys cannot be ruled out. User was informed and asked for backup information; answer pending. Do not delete the file or fabricate replacement keys. Current account is in Administrators; SSHD configuration uses the separate ProgramData administrator key file for that group, but that does not prove the previous per-user contents. Subsequent fixtures disable authorized-key/principal generation, set SSH_ROOT_PATH inside fixture, isolate HOME/USERPROFILE and Git config, and compare per-user file fingerprint before/after. Record this as introduced by this task, not pre-existing.

## Resume

Use H: only. Go executable is `I:/Projects/tooling/go1.27.2/go/bin/go.exe`; caches/temp set beneath `H:/Projects/tooling/myaccounts`, `MYACCOUNTS_TEST_DATA` chooses F: NTFS. `scripts/build.ps1` builds frontend/tests/vet/native EXE. Smoke requires a fresh data directory and optional `MYACCOUNTS_FILEBROWSER_EXE` / `MYACCOUNTS_GITEA_EXE` fixture binaries. Never run upstream DB tests against real accounts.

Resolve SSH backup with user if available. Obtain explicitly pending GitHub authorization before remote writes. Finish clean Windows/service/reboot, real SMTP/reset/passkeys/group policy, public Cloudflare HTTPS, full browser consent/revocation/key rotation, upgrade/cross-machine restore, custom UI/admin panels, remaining advisories and license review before approving or publishing v0.1.0. Do not infer these from upstream screens or local smoke success.
