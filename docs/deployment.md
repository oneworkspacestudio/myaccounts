# Deployment and recovery

## Files and permissions

The application produces `config.json`, `users.db`, `conf/app.conf`, `secrets/`, `files/`, `uploads/`, `tmp/`, `logs/`, `backups/`, `web/build/` and optional `tools/cloudflared.exe` inside the selected data folder. The frontend is embedded in the EXE and extracted at startup. Database records contain identity secrets and signing keys: never publish or commit this folder.

The launcher removes inherited ACLs from the data folder and grants the current Windows user, SYSTEM and Administrators access. Choose NTFS. Keep other users out of the executable and data directories. The product does not encrypt the complete SQLite database; use an encrypted disk for physical theft protection.

DPAPI tokens are Windows-machine-bound. The NTFS ACL permits the owner, SYSTEM and Administrators to read protected files, allowing the default LocalSystem service to read the tunnel credential after interactive setup. Any local user who obtains a blob can ask Windows to decrypt machine-protected data, so the ACL boundary is mandatory. Cross-machine backup restore decrypts the original token into the authenticated encrypted archive and protects it again on the destination machine. Actual service/reboot and cross-machine tests remain release gates.

## Windows service

Complete setup interactively first. From an elevated terminal:

```powershell
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data service install
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data service start
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data service stop
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data service uninstall
```

The setup wizard offers automatic Windows startup. Selecting it requests UAC, installs delayed automatic startup with restart-on-failure, stops the interactive launcher and starts the service. Leaving it unchecked needs no elevation. A configured installation is required; service startup never exposes a first-run wizard. The protected owner SID preserves the configuring user's access when SYSTEM starts the service. Check `logs/launcher.log` and `logs/identity.log`. Launcher and identity output rotate at 10 MiB with five backups and 30-day retention. Casdoor also maintains operational logs and database audit records. Actual service/reboot verification is still pending.

## Cloudflare

Use a scoped API token with Account / Cloudflare Tunnel / Edit and Zone / DNS / Edit and Read permissions for the chosen account and zone. The wizard also needs your account ID and zone ID. Enter an HTTPS subdomain issuer and explicitly check the resource-creation authorization. The API token is used in memory and discarded; only the operational tunnel token is protected with DPAPI.

The zone must be active and belong to the selected account. The exact hostname must have no existing DNS records. Provisioning creates a fresh dedicated remotely managed tunnel, a localhost-origin route with catch-all 404, and a proxied CNAME. A partial failure may leave a created tunnel or record; the error includes the tunnel ID. Inspect and remove only those newly created resources yourself. Existing resources are never reconfigured.

For an existing tunnel, configure its public hostname to `http://127.0.0.1:8788` (or your selected port), and provide its operational token. The launcher verifies the official connector download and starts it. The token is supplied via environment rather than command-line text. State appears in `tunnel-status.json`; connecting is not proof of successful public HTTPS.

Manual mode supports an independently configured tunnel or reverse proxy. Terminate TLS at your proxy, forward the original public Host header, keep the origin loopback-only, and expose only the configured origin. Verify the discovery issuer matches the configured public origin, then test a complete authorization flow. Do not expose a public installation until the security gates are satisfied.

Official reference: https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/get-started/create-remote-tunnel-api/

## Encrypted offline backups

Stop the application/service before backup. Put a strong passphrase (at least 16 characters) in a file protected by NTFS ACLs; passphrase files are never committed or included in backup. Age's standard scrypt recipient format encrypts and authenticates the complete archive.

For an interactive instance, run `MyAccounts-windows-amd64.exe -data F:\MyAccounts\data stop`. The protected process control credential requests graceful engine shutdown and waits for the launcher to release the data-directory lock. For a Windows service, use `service stop`.

```powershell
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data backup F:\MyAccounts\backup.age F:\private\passphrase.txt
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\restored restore F:\MyAccounts\backup.age F:\private\passphrase.txt
```

Restore refuses any existing destination. It authenticates the archive before extraction, rejects unsafe paths and symlinks, enforces a 512 MiB limit, stages into a protected folder, validates configuration and presence of the database, then renames the staged folder into place. Signing keys and stable user IDs remain in the database. SQLite quick_check validates database integrity before backup and restore. The tunnel token is re-protected for the destination Windows machine; no plaintext credential file remains after restore. A real cross-machine restore test is still required.

Inspect the restored folder and test locally before switching the service data path. Keep the old folder as rollback until identities, applications and issuer are verified. Backups omit logs, temporary sessions, extracted web assets and previous backups. Machine-bound schedule/migration/owner credentials are also omitted; configure a backup schedule again on the restored machine.

## Online and scheduled backups

Sign in as a global administrator and open `/operations`. The server console provides health, HTTPS verification, backup download, release checking and schedules (hourly, six-hourly, daily or weekly). Online backups use SQLite `VACUUM INTO` to include committed WAL transactions, then authenticate and encrypt the snapshot with age. A real scheduled firing and native restore passed on a disposable installation. The launcher must be running for the schedule to fire. Keep the passphrase separately; Windows protects a local copy for scheduled execution. Backups are retained until you remove them; monitor available disk space.

## Verification, token recovery and updates

```powershell
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data verify
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data tunnel download
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data tunnel token F:\private\new-tunnel-token.txt
.\MyAccounts-windows-amd64.exe update-check
```

Stop the application before replacing its operational tunnel token. Obtain a new token through authorized Cloudflare administration, save it in a protected temporary file, run the token command, then restart. Delete the temporary file according to your storage policy. The command protects the token locally and never prints it. `verify` checks discovery, exact issuer consistency and advertised endpoint origins over certificate-validated HTTPS (or localhost for local-only mode).

`update-check` reads the latest published GitHub release and never installs anything. A private repository may require a read-authorized token supplied as `MYACCOUNTS_GITHUB_TOKEN`; it is not persisted. No release or missing access produces a clear error. Release checking does not approve applying an update.

Stop the service, make and verify an encrypted backup, preserve the previous EXE, then replace only the EXE. The data path and issuer must remain unchanged. Before starting a changed executable, the launcher takes a protected encrypted upgrade snapshot. It commits the executable marker only after local health succeeds. A failed startup restores the previous database under the engine lock and quarantines changed DB/WAL files in protected backups; run the saved previous EXE after investigating the failure. Failed-migration identity recovery and a healthy live upgrade passed. This does not establish physical power-loss durability. Never downgrade a changed database without a verified snapshot. Automatic executable installation is not implemented.

## Email and signing keys

Every installation asks for its own domain, Cloudflare method and optional SMTP credentials at execution. The first-run email test supports certificate-validated implicit TLS on port 465; additional SMTP settings are available in Providers after login. An email address and root domain do not establish a working SMTP server. Keep signup and recovery disabled until a real test message arrives. Use the separate `myaccounts` organization for normal users; every built-in user is a global administrator.

For key rotation, create a new certificate, allow relying parties time to refresh JWKS, then select it for the application. Keep the old verification certificate until its issued tokens expire and clients have refreshed. Immediate activation can fail in clients that recently cached JWKS. The independent-client test verified the new signature with refreshed discovery, overlapping public keys, unchanged subject and old-token UserInfo. Consent removal prompts again on later authorization; it does not establish immediate invalidation of an already issued token or a relying party's session.
