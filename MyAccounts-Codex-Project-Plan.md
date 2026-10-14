# MyAccounts — Codex Implementation Plan

## Mission
Build and publish **MyAccounts**, a portable self-hosted Google-Accounts-inspired identity and SSO platform. Target Windows 10/11 and Windows Server, with a **single native `MyAccounts.exe`** containing the web UI and identity service; no Docker, Node.js, Python, Java or separate database runtime. Users sign in via email/password, maintain a profile, optionally enable 2FA, and access integrated applications via OIDC. Include first-run domain setup and optional automated Cloudflare Tunnel provisioning.

**Codex execution instruction:** Do not stop at scaffolding. Review upstream components, implement the application, test it, create a new GitHub repository, commit/push and publish a verified GitHub Release. Do not claim any step succeeded unless verified. Obtain authorization for external account actions.

## 1. GitHub project
- Create a **new private** repository named `myaccounts` in my authenticated GitHub account; description: `Portable self-hosted Google-style identity and single sign-on platform`.
- Never overwrite an existing repository. Report name conflicts.
- Add README, architecture and deployment docs, appropriate licence (check upstream licence compatibility), contribution guidance, security policy, `.gitignore`, tests and GitHub Actions release workflow.
- Never commit secrets, live databases, credentials, session tokens or signing keys.
- Use milestone commits and push changes to `main`.

## 2. Architecture and existing authentication software
- First evaluate **Casdoor** (`https://github.com/casdoor/casdoor`) as the preferred established identity engine. Inspect the actual source, dependencies, licence, SQLite support, Windows AMD64 compatibility, MFA, email/password, OIDC and the feasibility of embedding its UI and backend in one binary.
- If Casdoor cannot be packaged securely and reliably as a standalone Windows executable, compare appropriate Go-native alternatives and document the choice in `docs/architecture.md`.
- **Do not write custom OAuth/OIDC protocol or cryptographic primitives.** Reuse tested upstream implementations and pin dependencies.
- Build as one Go executable where viable, with embedded static frontend assets and embedded SQLite persistence.
- The Cloudflare connector (`cloudflared.exe`) may be downloaded separately and automatically managed by the app. Make this distinction clear in documentation.

## 3. Single-executable and portable design
- Release main artifact: `MyAccounts-windows-amd64.exe`.
- Double-click starts service locally, opens the setup wizard on first run, and admin dashboard subsequently.
- Package web assets inside the executable; do not require an installed developer runtime.
- Keep durable data next to the executable (or an explicitly chosen writable data path). Support an unattended Windows service mode.
- Avoid two running instances competing for the same database; shut down cleanly and rotate logs.
- Example layout after first launch:

```text
MyAccounts/
├── MyAccounts.exe
├── data/
│   ├── users.db
│   ├── config.json
│   ├── secrets/
│   ├── uploads/
│   ├── logs/
│   └── backups/
└── tools/
    └── cloudflared.exe
```

- Restrict file permissions; encrypt sensitive secrets at rest where appropriate. Document machine-bound encryption tradeoffs and offer portable encrypted backup/restore.
- Safe database migrations, rollback/backups and updates must not erase identities or client registrations.

## 4. Google-inspired identity experience
Create a polished, original-branded, responsive account interface inspired by Google Accounts' interaction patterns, **not a deceptive replica or use of Google branding**.

Screens and flows:
- Email-entry → password-entry sign in; create account; verify email; password recovery/reset.
- Remembered-account chooser; add/switch/remove remembered accounts; sign out selected/all accounts.
- Optional TOTP authenticator 2FA; optional passkeys/WebAuthn; recovery codes; verification challenges.
- Home; personal info; password/security; devices and sessions; connected applications; privacy/data controls.
- Avatar, first/last name, display name, username if enabled, verified primary email, optional phone/DOB, language/locale/theme, stable account ID, groups/roles, created/last-sign-in dates.
- Minimal, accessible mobile-friendly UI with light/dark modes, keyboard support, safe error messaging and customizable original branding.

## 5. Security and account administration
- Use robust password hashing (e.g. Argon2id through maintained library), email verification and expiring single-use recovery tokens.
- Optional per-user TOTP 2FA and passkeys; allow administrators to mandate MFA for selected groups.
- Recovery codes, login throttling/rate limiting, secure cookies, session rotation/revocation and audit logs.
- Standard protections against CSRF, unsafe redirects, credential enumeration, token replay and session fixation.
- Correct PKCE/state/nonce handling; validate OAuth/OIDC redirect allowlists and tokens.
- Generate strong secrets/signing keys; avoid defaults, secret logging and accidental admin exposure.
- Admin console: users, groups, roles, apps, OIDC clients, sessions, audit/security events, domain, SMTP, Cloudflare status, backups, logs, health and updates.
- Email delivery: configurable SMTP and test-email action; clearly restrict email-dependent flows until email works.

## 6. OpenID Connect SSO
Provide a standards-compliant OIDC provider, preferably via the adopted identity engine:
- Discovery, authorization code + PKCE, token endpoint, UserInfo, JWKS, signed tokens and key rotation.
- Stable `sub` identifier; standard `openid profile email` scopes and configurable group claims.
- Client registration, per-app permitted redirect URLs, client-secret rotation, consent and revocation where supported.
- Never give integrated apps user passwords; use scopes/claims instead.
- App launcher with icons, names, URLs, roles/groups and configurable visibility.
- Account switching must follow each client's normal authorization flow; never silently substitute another identity.
- Test compatibility with an independently maintained OIDC client.

## 7. Self-hosted app integration and discovery
Initial integrations: **FileBrowser Quantum, Gitea, generic OIDC clients**, plus recipes for custom Node.js and Python apps.
- Verify current configuration parameters from each app's real documentation/source; do not guess.
- Generate integration steps/client registrations and validate redirect callbacks.
- Optional opt-in discovery: configured port checks and known application config paths. Only identify products using reliable signals.
- Back up config before user-authorized changes; never edit unknown software automatically.
- For apps without OIDC, document and optionally support an auth reverse proxy where compatible; do not claim universal automatic SSO.

## 8. First-run wizard
Bind initial configuration exclusively to localhost until admin setup completes. Make it inaccessible from public interfaces.

1. **Welcome:** check OS, writable data path and prerequisites.
2. **Administrator:** request name, email, password and confirmation; no factory default credentials.
3. **Domain:** ask for root domain (e.g. `example.com`) and subdomain (default `login`) to create `https://login.example.com`; optionally `accounts.example.com`. Validate public issuer consistency.
4. **Cloudflare method:** API token for automated setup; existing tunnel token; manual setup; or local-only mode.
5. **Email:** SMTP details and a test-email button. Restrict registration/recovery features if not configured.
6. **Provision:** write protected config, create DB, generate secrets and (if authorized) create tunnel/DNS.
7. **Verify:** local health, database, tunnel, public HTTPS, issuer URL, discovery endpoint and login; show useful error messages.

## 9. Cloudflare Tunnel automation
Implement with official Cloudflare APIs and a verified official `cloudflared` binary.
- Accept a **scoped API token** and validate accessible accounts/zones; never require a global API key.
- Confirm the selected domain is on the user's Cloudflare account (with correctly configured DNS).
- Ask for explicit authorization before creating/changing Cloudflare resources.
- Create or reuse dedicated `myaccounts` tunnel, configure public hostname and local-only origin, and create DNS routing without overwriting unrelated entries.
- Download/check official connector and start it. Offer automatic Windows service install when elevated permissions are approved.
- Prefer remotely managed tunnels; follow current official API contracts and authentication methods.
- Support pre-existing tunnel token and manual/no-Cloudflare modes.
- Detect DNS collisions, rejected credentials, missing zone ownership, disconnected connectors and HTTPS failures.
- Persist only necessary operational credentials securely; never log tokens or embed them in exposed command-line args where safer options exist.
- Provide status, reconnection, token rotation/recovery, and boot persistence. Expose only required application endpoints, not unrelated management services.

## 10. Windows startup/service
Implement `MyAccounts.exe service install|start|stop|uninstall` using a maintained Windows service library.
- Interactive mode: double-click, start local server and open dashboard/wizard.
- Service mode: automatic start without logged-in user, reliable shutdown and logs; handle tunnel connector lifecycle or coordinate with its own Windows service.
- Request elevation only where needed. Ensure setup does not require admin rights unless installing services.

## 11. Backup and updates
- Manual and optional scheduled encrypted backups of database, issuer signing keys, secrets, avatars and client registrations.
- Secure portable restore, integrity validation and migration checks; preserve stable identity identifiers and signing configuration.
- Document key portability and OS-protection constraints; ensure no data loss on version upgrade.
- GitHub release update checking and appropriate rollback strategy.

## 12. Testing and security gates
Automated and manually verified coverage for:
- Fresh Windows first run; first admin creation; email/password login; failed credentials; verification/reset.
- Profile edits, optional TOTP, recovery codes, remembered-account chooser, sessions/revocation.
- OIDC discovery/JWKS, authorization code + PKCE, token validation, invalid redirects, secret rotation.
- FileBrowser Quantum and Gitea integration (where integration environments available).
- Mocked Cloudflare API success/failure, DNS conflicts and tunnel reconnection; real Cloudflare end-to-end tests only with explicit credentials/test zone.
- Windows service startup after reboot; concurrent process prevention; SQLite backup/restore; migration and no-data-loss upgrade.
- Security regression tests, pinned dependency audit, static analysis and SBOM.
- Verify produced EXE on a **clean Windows machine without development runtimes**. Do not claim unperformed tests have passed.

## 13. GitHub Actions and release
On a version tag:
1. Checkout and install pinned build tooling.
2. Static analysis, unit tests and integration tests.
3. Build frontend, embed assets and compile Windows AMD64 executable.
4. Run relevant clean-environment smoke tests.
5. Generate `SHA256SUMS.txt`, SBOM and release notes.
6. Publish tagged GitHub Release containing `MyAccounts-windows-amd64.exe` and checksums.

Optionally provide a portable ZIP, but the standalone executable is mandatory. Publish `v0.1.0` only when the documented release acceptance gates pass, and label pre-production limitations clearly.

## 14. Milestones
- **A — Foundation:** New GitHub repo, architectural review, selected identity engine, basic build and CI.
- **B — Accounts:** Sign-in, registration, profiles, password reset, MFA and admin management.
- **C — SSO:** OIDC, client management, account chooser, app launcher.
- **D — Portable EXE:** Embedding, SQLite, migrations, data directories and Windows service.
- **E — Setup + Cloudflare:** Domain wizard, scoped authorization, tunnel/DNS automation and verification.
- **F — Integrations:** Real FileBrowser Quantum and Gitea configurations tested.
- **G — Hardening:** Security, backups, portability tests, docs, failures/recovery.
- **H — Release:** Commit/push, GitHub Actions release, executable/checksums and installation instructions.

## 15. Definition of done
- New repo created in my GitHub account; source committed/pushed.
- CI produces a verified downloadable native Windows EXE.
- Clean Windows host starts first-run setup without Docker or additional programming runtimes.
- Admin account and email/password user accounts work, including profiles and optional 2FA.
- Domain wizard can configure `https://login.<chosen-domain>` with authorized Cloudflare Tunnel provisioning, and verifies public HTTPS and OIDC issuer.
- OIDC application sign-in works, including verified FileBrowser Quantum integration.
- Users persist through restart/reboot and upgrade; Windows service/tunnel recovery works.
- Encrypted backup/restore and critical security regression tests pass.
- GitHub Release published with executable, checksum, release notes and known limitations.

## 16. Final Codex instructions
Implement iteratively and autonomously. Inspect current upstream source and official documentation first. Make small validated commits. Do not silently drop important features or invent verification results. Request interactive authorization when genuinely required for GitHub or Cloudflare. If a requirement is incompatible with a secure architecture, document it and adopt the closest safe alternative. Return the repository URL, release URL, final architecture, implemented vs incomplete features, actual test evidence, and installation/recovery steps.
