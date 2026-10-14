# MyAccounts preview-20261014.2

Windows AMD64 evaluation release with a standalone account-server executable,
embedded UI/SQLite, and an optional separate shared-email service executable.

## Included in this update

- Email delivery defaults to **Skip email for now**. No sending account or SMTP
  credentials are needed to administer accounts or use password sign-in.
- Administration at `/operations` links to user creation, profile editing,
  deletion and direct administrator password resets. New-user passwords are
  chosen by the administrator rather than filled with a shared default.
- Optional shared email has an installer ownership-code flow and a separate
  operator wizard for configuring its sending account and public connection.
  The owner wizard can also be skipped. Cloudflare tunnelling is independent.
- README screenshots and a detailed guide cover our custom
  [FileBrowser Drive/Office](https://github.com/fahimc/filebrowser/tree/drive-reskin),
  exact callbacks, permissions and recovery. Other-app instructions include
  generic OIDC settings, Gitea and runnable Node/Python relying-party examples.
- Existing OIDC/PKCE, TOTP/recovery, passkeys, group MFA, consent, key rotation,
  encrypted backup/restore and optional Windows startup support remain included.

## Install

Download `MyAccounts-windows-amd64.exe` and verify its SHA-256 using
`SHA256SUMS.txt`. Double-click it, choose a writable protected NTFS data directory,
create your own administrator and follow the setup wizard. No Go, Node, Python,
Docker or separate database runtime is needed. Cloudflare's connector is optional
and downloaded separately.

`MyAccounts-Mail-windows-amd64.exe` is only for the operator of a shared email
service. Ordinary installations do not need it. A public shared service is not
activated by downloading the main EXE; its operator must configure and verify it.
Email may remain disabled. See the README and `docs/shared-email.md`.

For an upgrade, take and verify an encrypted backup, stop MyAccounts, retain the
previous EXE and replace the executable while preserving the data path and issuer.
See `docs/deployment.md` before changing an existing installation.

## Verification and limits

Tagged GitHub Actions results are the authoritative build evidence. The workflow
runs native/UI checks, OIDC and recovery regressions, administration with email
disabled, dependency analysis, SBOM generation and uploaded checksum verification.
Local disposable browser tests cover custom FileBrowser SSO and sample screenshots.

This is a **prerelease for evaluation**, not v0.1.0 production approval. Remaining
gates include an independent clean Windows host, service/reboot/recovery,
real SMTP/public shared-service inbox delivery, cross-machine restore and full
threat/licensing review. No successful real SMTP authentication/inbox delivery is
claimed. Some nonreachable dependency advisories and a third-party redistribution
review remain open. SQLite relies on NTFS ACLs; full database encryption at rest,
a detailed log viewer and automatic app discovery are not implemented.

Create normal accounts in **myaccounts**, never enable public signup in the
**built-in** global-administrator realm, and retain connected apps' own recovery
access. Consult `docs/acceptance.md`, `docs/release-acceptance.json`, `LICENSE`,
`NOTICE` and the included SBOM. Full v0.1.0 publication remains gated.
