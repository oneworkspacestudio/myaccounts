# Acceptance status

This is the implementation and evidence ledger for the supplied brief. **v0.1.0 publication is blocked** until every gate in `release-acceptance.json` has real evidence. Upstream functionality is distinguished from functionality actually tested in this package. Do not infer completed tests from existing screens, source, or a successful compilation.

## Implemented locally

* Casdoor source retained at the reviewed commit with license notices and original protocol implementation. SQLite updated to pinned v1.60.1, with its resolved dependency upgrades.
* Embedded frontend, original MyAccounts logo and branding, responsive local setup wizard with light/dark adaptation and keyboard-accessible native controls.
* First administrator name/email/password/confirmation, local-only startup, HTTPS issuer validation, and explicit Cloudflare resource-creation authorization.
* Local/manual/existing-token/API Cloudflare modes; token validation, active-zone ownership, DNS conflict refusal, remotely managed tunnel ingress, token retrieval and verified official connector download with retry.
* Optional SMTP settings and explicit test-email action in the wizard (implicit TLS on port 465). An Email provider is initialized; signup remains disabled and provider attachment/verification policy is completed in the dashboard.
* Native Windows data-folder selection without defaulting to C:, persistent-ACL filesystem enforcement, restrictive ACLs, machine-bound DPAPI bootstrap and operational credentials, duplicate launcher/engine locks, orderly HTTP shutdown, rotating logs and Windows service commands.
* Age-encrypted offline backup/restore, authenticated archive extraction, unsafe-path/symlink/duplicate refusal, size limits, SQLite quick_check and Windows credential re-protection. Existing restore targets and backup files are refused.
* Public/local issuer discovery verification command, security headers, browser mutation Origin checks, opaque-session HttpOnly/SameSite/Secure normalization, and a global login throttle.
* Build/test/security/SBOM/checksum GitHub workflow with an explicit acceptance gate before publishing a tagged pre-release.
* README, architecture, deployment/recovery, integration recipes, contribution guidance, security policy, license/notice and original brief.

## Test evidence

Executed on the developer Windows host with Go 1.27.2; temporary protected data on F: and source/build artifacts on H:. This is not an independent clean Windows VM test.

* `go test ./internal/portable ./cred .`: passed, including real Windows explicit-ACL removal and real SQLite encrypted backup round-trip and byte preservation, wrong-passphrase refusal, encrypted-archive tampering refusal, existing-file refusal, issuer validation, DPAPI credential round-trip, first-run Host/Origin/setup-token boundaries, token persistence, mocked Cloudflare success/DNS collision/rejected credentials/authorization, issuer discovery mismatch detection and SMTP input refusal.
* `go vet ./internal/portable .`: passed.
* Frontend production build: passed with Vite 5.4.21, 3,511 modules. The upstream build reports a translation-chunk warning; no frontend build failure.
* Setup desktop/mobile PNGs captured and desktop and mobile visual layouts reviewed. Screenshots are in `docs/screenshots/`.
* Source transfer: all 1,308 original source files matched between I: and H: at transfer time. E: copy failed checksums and was abandoned. Subsequent implementation proceeds only in H:.

* Native AMD64 build (`CGO_ENABLED=0`): passed. `scripts/smoke.ps1 -OIDC` passed startup, protected bootstrap removal, email/password login, profile edit, HttpOnly cookie, failed-password refusal, discovery/JWKS, competing-instance refusal, orderly shutdown, encrypted backup/restore with byte-identical SQLite, restored identity login and signing-key preservation.
* `integration/oidc.mjs` with independent openid-client 6.8.8: passed discovery, code + PKCE, JWKS signature validation, nonce/state, UserInfo, invalid redirect/verifier and code replay refusal, and client-secret rotation. The provider credential API drives authorization; full browser consent and signing-key rotation are still pending.
* `integration/mfa.mjs` with independent otpauth 9.5.2: passed enrollment, password/MFA challenge, bad-code refusal, valid TOTP, repeated-step refusal and two concurrent requests for one recovery code with exactly one success. Replay markers persist in SQLite.
* Real FileBrowser native build drive-office-v0.3.1 (commit 72fc7547d0b099864f2957e26e745cc1ff6facbc, Quantum beta2.1.2 derivative): passed real OIDC login redirect/callback, token verification, authenticated session, stable-sub linkage and normal-user permissions in disposable configuration.
* Official Gitea 28.1.0 Windows AMD64 (SHA256 68495c963bab1bfcbbe5b48916c7f362a49aca931583575e09763a02ddf63e2d): passed real OIDC redirect/callback, automatic account registration, signed-in settings page, stable subject and administrator-access refusal in disposable configuration. Browser sessions do not authenticate this version's API; the harness checks the web session instead.
* govulncheck 1.9.0: initial eight reachable advisories remediated through pinned x/net, OpenTelemetry, AWS S3 and Redis upgrades. Repeat scan exit 0: zero reachable vulnerabilities, but four imported-package and three module advisories remain nonreachable in the scan. This is not a blanket vulnerability-free finding.
* CycloneDX gomod 1.9.0: generated `dist/SBOM.cdx.json`, format 1.6, 303 components. License detection warnings for darabonba-number and aliyun-oss require manual review. No complete third-party license clearance claimed.

## Introduced host-side incident

The initial isolated Gitea migration wrote an empty `%USERPROFILE%\.ssh\authorized_keys` on 2026-10-14. The prior contents were not captured and no automatic backup was found; possible loss of pre-existing keys cannot be ruled out. The user has been informed and asked about a backup. The file has been left for inspection. This account is in Windows Administrators, whose SSH configuration uses a separate ProgramData administrator key file; that does not establish what the per-user file previously contained. Subsequent fixtures explicitly disable authorized-key/principal-file generation, isolate HOME/USERPROFILE and Git configuration, and assert that the per-user key-file fingerprint remains unchanged. Do not conceal this introduced issue or fabricate replacement keys.

## Remaining requirements and limitations

| Brief area | Current status / remaining verification |
| --- | --- |
| Private GitHub repository, commits/push, CI, release | Repository authorization requested; no remote or release claimed. Workflow cannot be described as verified until run. |
| Email/password account system, profiles and administration | Executable-level email/password, session and profile tests passed; full registration and administration acceptance pending. |
| Registration, email verification, reset/recovery | Upstream flows retained; initial signup disabled. Real SMTP delivery and expiring/single-use recovery tests pending. |
| TOTP, recovery codes, passkeys, group MFA mandates | TOTP enrollment/challenge/replay and concurrent single-use recovery tests passed. Passkeys and group policies pending. |
| Email-entry → password-entry and remembered-account UX | Established Casdoor UI retained with original branding; exact requested interaction parity and all chooser/switch/revoke flows need review. |
| OIDC, redirects, PKCE, state/nonce, signed tokens, rotation | Independent OIDC regression and real relying-party callbacks passed as listed above. Full browser consent/revocation and signing-key rotation pending. |
| App launcher, roles/groups, per-app visibility | Upstream application administration and access model retained; user-facing launcher/visibility acceptance pending. |
| FileBrowser Quantum / Gitea | Real native FileBrowser and Gitea OIDC flows passed in disposable configuration. Node independent-client harness present; standalone Python example pending. |
| Opt-in application discovery / automatic config changes | Not implemented. No unknown software configuration is edited. |
| First-run domain root/subdomain controls, prerequisite display | HTTPS issuer input supports chosen hostname. Dedicated root/subdomain fields and full prerequisite review/status UI remain incomplete. |
| Cloudflare actual resources, DNS, connector and public login | Code and mocked API tests implemented. No credentials/test zone supplied; no real cloud resources created. Connector live download, status/rotation and public HTTPS checks pending. |
| Windows service / boot / reboot / shutdown recovery | Service command implementation present. Actual service installation/reboot and connector recovery tests pending. |
| Secret encryption | Launcher credentials protected by machine DPAPI plus mandatory ACLs. SQLite secrets/signing keys rely on ACLs; full database encryption is not implemented. |
| Backup, restore and upgrade | Native encrypted backup/restore preserves SQLite bytes, identity login and signing keys. Upgrade and cross-machine restore pending; scheduling and automatic migration rollback not implemented. |
| Updates, health/logs/backups administration panels | Local health/logs and CLI operations present; release-check and token-recovery CLI implemented; full MyAccounts-specific administration panels remain incomplete. |
| Clean Windows EXE, SBOM, vulnerabilities, static analysis | Build, focused tests/vet, reachable-dependency audit and SBOM generation passed locally. Clean host, remaining advisories and manual license review remain gates. |

The project must remain a development build while these gaps exist. A successful local smoke test cannot justify publishing a production identity release or claiming the full brief is complete.
