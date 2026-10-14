# Architecture and upstream review

MyAccounts is a Windows launcher around a pinned, locally maintained Casdoor engine. The Go module path remains `github.com/casdoor/casdoor` to preserve upstream internal imports; this is a derivative product, not an official Casdoor release.

## Source review

Reviewed Casdoor commit `2e1127cf56b1ecba1863163007af0e04a6f554c8` on 2026-10-14. The source is included, with Apache-2.0 notices retained. The SQLite driver was upgraded and pinned to modernc.org/sqlite v1.60.1 (requires Go 1.26), with related maintained dependencies resolved in go.mod. Relevant actual source: `go.mod`, `main.go` upstream (now `engine.go`), `object/ormer.go`, `object/init.go`, `routers/static_filter.go`, `object/token*.go`, `object/mfa*.go`, `cred/argon2id.go` and `web/src/pages/auth/`.

* Backend: Go/Beego; SQLite uses `modernc.org/sqlite`, so CGO and a database runtime are unnecessary. Upstream also links optional database and integration drivers; the resulting binary is larger than a minimal identity server.
* UI: React/TypeScript/Vite. Node/Yarn are build-time tools only. Built assets are embedded with `go:embed`, then refreshed into the protected data directory at startup so the established static filter can use them. One EXE is distributed; two processes of that same EXE isolate the launcher and identity engine.
* Accounts: Casdoor owns password validation, Argon2id hashing, profiles, MFA, WebAuthn, sessions, authorization policy, application registration, recovery and audit records. MyAccounts does not implement OAuth/OIDC or cryptographic primitives.
* OIDC: Casdoor implements discovery, authorization, token exchange, JWKS and UserInfo. Protocol compliance and independent-client interoperability are acceptance gates, not assumptions based on feature names.
* New installation defaults: MyAccounts branding, generated 4096-bit RSA signing certificate, 12-character password minimum, Argon2id, signup disabled, private profiles, no default LDAP/RADIUS listeners, localhost-only HTTP. The upstream empty-admin welcome behavior is bypassed by the launcher’s protected local bootstrap.

Casdoor can be packaged as one native executable with these adaptations. It has a substantially larger feature and dependency surface than the brief needs. A production release requires continued upstream review and vulnerability remediation.

Alternatives considered from their upstream documentation: [ZITADEL OIDC](https://github.com/zitadel/oidc) provides a maintained provider library but would require writing and validating the full account system and persistence adapter. [Dex](https://github.com/dexidp/dex) is an OIDC federation engine with static password support, rather than a complete self-service profile/recovery administration product. [Pocket ID](https://github.com/pocket-id/pocket-id) emphasizes passkeys and does not match mandatory email/password flows. Keeping Casdoor is the closest fit and avoids inventing protocol logic. Alternatives were compared at product/library level, not subjected to complete source audits.

## Process and storage boundaries

The launcher owns `launcher.lock`, setup, asset extraction, configuration, logs, service control and the connector. The engine owns `engine.lock` and SQLite. Duplicate starts refuse the data directory. Shutdown is requested over loopback with a fresh random process-only secret; the engine drains HTTP requests before exit. A ten-second deadline permits forced termination if shutdown fails. SQLite handles crash recovery; reboot and power-loss durability are unverified.

`config.json` contains no credentials. Windows DPAPI protects the first-admin bootstrap and tunnel token for the current Windows machine. The bootstrap is removed after the administrator’s database record is initialized. Signing keys, SMTP credentials and MFA secrets live in the identity database and rely on restrictive NTFS directory ACLs; the database is **not encrypted at rest**. BitLocker or an encrypted volume is recommended for physical theft protection. No claim of whole-database encryption is made.

Source and build caches may reside on exFAT. Live data must support ACLs. On this development machine the source is on H: (verified copy from I:) and runtime testing uses F: to avoid C:. By default the executable uses data beside itself; unsupported or unwritable storage triggers a native Windows folder picker in interactive mode. The nonsecret data-location hint is stored beside the EXE. `-data` or `MYACCOUNTS_DATA` selects another NTFS location. Portable data beside the executable is explicitly supported with `-data .\data` on NTFS.

The launcher’s local wizard verifies Host, Origin and an unguessable fragment-delivered setup token. It persists no API token. Cloudflare provisioning requires a separate explicit checkbox, active zone ownership, and an absent DNS record. Only fresh dedicated tunnels are created; failures report resource IDs for manual cleanup. No existing tunnel or DNS configuration is overwritten.

`cloudflared.exe` is a separate official Cloudflare connector, downloaded into the protected data `tools/` directory. Its SHA256 must match the official GitHub release asset digest; download fails closed when no digest is available. The operational token is inherited through the environment, never passed as a CLI argument. Connector retry uses bounded backoff. Local connector lifecycle status is not proof that public HTTPS works.

## Important unfinished verification

See `docs/acceptance.md` for the authoritative status of every brief requirement. A packaged engine does not establish that all upstream features work with our SQLite/Windows configuration. Public issuer consistency, secure cookie handling, independent-client code/PKCE flows, SMTP, integrations, clean Windows startup and reboot/service tests must be demonstrated before a v0.1.0 release.
