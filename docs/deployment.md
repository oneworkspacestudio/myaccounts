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

Installation uses kardianos/service and normal Windows elevation. The application does not silently request UAC elevation. A configured installation is required; service startup never exposes a first-run wizard. Check `logs/launcher.log` and `logs/identity.log`. Launcher and identity output rotate at 10 MiB with five backups and 30-day retention. Casdoor also maintains its own configured operational log and database audit records.

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

Inspect the restored folder and test locally before switching the service data path. Keep the old folder as rollback until identities, applications and issuer are verified. Backups omit logs, temporary sessions, extracted web assets and previous backups. Scheduled encrypted backups are not implemented yet.

## Verification, token recovery and updates

```powershell
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data verify
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data tunnel download
.\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data tunnel token F:\private\new-tunnel-token.txt
.\MyAccounts-windows-amd64.exe update-check
```

Stop the application before replacing its operational tunnel token. Obtain a new token through authorized Cloudflare administration, save it in a protected temporary file, run the token command, then restart. Delete the temporary file according to your storage policy. The command protects the token locally and never prints it. `verify` checks discovery, exact issuer consistency and advertised endpoint origins over certificate-validated HTTPS (or localhost for local-only mode).

`update-check` reads the latest published GitHub release and never installs anything. A private repository may require a read-authorized token supplied as `MYACCOUNTS_GITHUB_TOKEN`; it is not persisted. No release or missing access produces a clear error. Release checking does not approve applying an update.

Stop the service, make and verify an encrypted backup, preserve the previous EXE, then replace only the EXE. The data path and issuer must remain unchanged. Startup refreshes frontend files and the generated engine configuration. Upstream table synchronization performs schema updates; automatic migration rollback is not yet implemented. If a migration fails, restore into a fresh folder and run the previous EXE against it. Never downgrade a changed database without a verified snapshot. Use the explicit update-check command; automatic installation is not implemented.
