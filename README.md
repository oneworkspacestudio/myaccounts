# MyAccounts

Portable, self-hosted identity and OpenID Connect for Windows, powered by Casdoor. Original branding, embedded web UI, embedded SQLite, and one native application executable. No Docker, Node, Python, Java or database server is required to run the packaged application.

**Development build. The complete release acceptance gates have not yet passed. Do not treat this as a production identity deployment.** See [acceptance status](docs/acceptance.md) for verified and incomplete work. The supplied brief is preserved verbatim in [MyAccounts-Codex-Project-Plan.md](MyAccounts-Codex-Project-Plan.md).

## Run

1. Obtain the built `MyAccounts-windows-amd64.exe` and verify its SHA256.
2. Put it in a writable folder. Double-click to open the local setup wizard, or choose an NTFS data location:

   ```powershell
   .\MyAccounts-windows-amd64.exe -data F:\MyAccounts\data
   ```

3. Create your administrator with your own password. Choose local-only mode initially, or configure a public HTTPS issuer and tunnel.
4. Sign in using the administrator email. Manage users, organizations, groups, roles, applications, security and providers using the Casdoor-backed dashboard.
5. Configure an SMTP provider and use its test-email action before enabling account signup or recovery. The initial application has signup disabled.

Local default: `http://localhost:8788`. Data defaults to `data/` beside the executable; `-data` or `MYACCOUNTS_DATA` overrides it. If that location is unwritable or lacks Windows permissions, interactive mode opens a native folder picker. Choose an NTFS location; a dedicated `MyAccounts/data` folder is created beneath it and its location is remembered beside the EXE. Flags precede subcommands. `-no-browser` is intended for an already configured installation. The first-run wizard is bound exclusively to IPv4 loopback.

To avoid C:, keep the executable on any suitable drive and pass an NTFS data path elsewhere. exFAT and FAT32 do not enforce the access controls required for live identities. For an explicitly portable NTFS installation, use `-data .\data`. Cloudflare's optional connector is a separate downloaded binary, stored in the protected data folder under `tools/`.

## Build

Pinned development toolchain: Go 1.27.2, Node 26.8.2, Yarn 1.22.22. Versions are build-time requirements, not application dependencies. Go dependencies are pinned in `go.mod`/`go.sum`; frontend dependencies are locked in `web/yarn.lock`.

```powershell
cd web
npx --yes yarn@1.22.22 install --frozen-lockfile --ignore-scripts
npx --yes yarn@1.22.22 build
cd ..
$env:CGO_ENABLED='0'
go test ./internal/portable ./cred .
go vet ./internal/portable .
go build -trimpath -ldflags '-s -w -X main.version=dev' -o dist/MyAccounts-windows-amd64.exe .
Get-FileHash dist/MyAccounts-windows-amd64.exe -Algorithm SHA256
```

Ignoring dependency install scripts prevents downloading Cypress; no runtime code requires those scripts. The frontend must be built before compiling the executable. The upstream database tests are not safe to run wholesale against a live installation and are not represented as passing by these commands.

## Operations

See [deployment and recovery](docs/deployment.md), [architecture](docs/architecture.md), [integrations](docs/integrations.md), [security policy](SECURITY.md) and [contributing](CONTRIBUTING.md).

Licensed under Apache-2.0. Original upstream notices are retained. This is an independent derivative of Casdoor and is not affiliated with Google.
