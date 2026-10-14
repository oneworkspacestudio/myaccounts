# MyAccounts development preview

Portable Windows AMD64 identity service based on pinned Casdoor, with embedded UI/SQLite, original account branding, email-first sign-in, saved account identifiers, TOTP/recovery codes, passkeys, OIDC, a domain/Cloudflare/SMTP setup wizard, optional Windows service and encrypted backups.

This preview is for evaluation. It does not satisfy all v0.1.0 acceptance gates. Do not treat it as a production identity deployment.

Verify `SHA256SUMS.txt`, then double-click `MyAccounts-windows-amd64.exe`. Choose an NTFS data folder, create your administrator and enter your own domain, Cloudflare and email settings. No programming or database runtime is required by the executable. The optional official cloudflared connector downloads separately.

Verified development-host checks include native startup, password/profile/session flows, code/PKCE/signature/nonce/state and key rotation, TOTP/recovery replay, group MFA, encrypted online/offline/scheduled backups, upgrade rollback, real FileBrowser/Gitea callbacks and Node/Python examples. Browser tests exercise virtual-authenticator passkeys and explicit consent. A loopback SMTP fixture verifies signup, atomically consumed/expiring recovery and old-session revocation; it does not verify your SMTP service. The tagged build's Actions result is the authoritative CI evidence.

Open gates: independent clean Windows host without developer runtimes; service/reboot/recovery; real SMTP registration/verification/reset; cross-machine restore; full threat and third-party license review. Dependency scanning found zero reachable vulnerabilities but nonreachable advisories remain. A pinned Alibaba number dependency has no license file in its module or public repository; redistribution review remains unresolved. SQLite is ACL-protected, not fully encrypted at rest. Opt-in application discovery and a detailed server log viewer are incomplete.

Use a separate normal-user organization. Casdoor's built-in organization gives every user global-administrator privileges; never enable built-in signup. Keep password recovery administrators for connected applications until their SSO and recovery paths are verified.

Read `docs/acceptance.md`, `docs/deployment.md` and `docs/integrations.md` in the repository before deployment. Full v0.1.0 publication remains blocked by `docs/release-acceptance.json`.
