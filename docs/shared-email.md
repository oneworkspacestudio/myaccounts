# Shared email and setup wizards

Email delivery and the sending account are optional. The installer defaults to
**Skip email for now**. Administrators can add, edit and remove users, and set
passwords directly without connecting any email service.

MyAccounts installations do not need SMTP details when using shared email.
Users enter their administrator email, choose **MyAccounts shared email**, receive
a six-digit connection code, and enter it in the setup wizard. The installation
generates its own random credential automatically. Cloudflare tunnelling for the
identity server continues independently.

## One-time service owner setup

`MyAccounts-Mail-windows-amd64.exe` is a separate native Windows executable for the
operator's shared server. It needs no Node, Python, Go installation or mail-server
runtime. Double-click it on a protected NTFS location, or select its data directory:

```powershell
.\MyAccounts-Mail-windows-amd64.exe -data F:\MyAccounts\mail-service -port 19890
```

Choose **Skip email for now** in the owner wizard to exit without saving a
sending account, creating a tunnel or starting the mail service. Run the mail
executable again later to configure it. MyAccounts itself remains usable.

If you choose to enable delivery, the local wizard asks for the sending account once. Gmail is a preset: enter the
sender email and a Google app password, not the account's normal password.
Another provider can use its encrypted SMTP submission endpoint on port 465.
The wizard sends a real code and requires it before saving configuration.

The connection step supports a new dedicated Cloudflare tunnel (with explicit
authorization and scoped token), an existing dedicated tunnel token, or manually
managed public HTTPS. The default shared address is `https://mail.m8e.co.uk`.
Changing the service operator/domain requires building installations with the
corresponding `internal/mailrelay.DefaultURL`; there is no relay URL or provider
password field in normal installation setup. The endpoint is not an existing
hosted service until the owner completes setup and verifies public access.

New Cloudflare provisioning refuses existing exact DNS records. It does not reuse
the identity tunnel or modify other services. Operational tunnel credentials and
SMTP configuration use machine DPAPI inside an owner/SYSTEM/Administrators-only
NTFS directory. The API provisioning token is discarded. The official connector
is verified using its release SHA256. The service listens only on loopback and
requires its configured public Host header after setup.

Keep the service running on a reliable host. The mail executable currently runs
interactively; automatic Windows service/reboot acceptance remains unverified.
Provider quotas, account eligibility, sending costs and inbox placement are the
operator's responsibility. Neither a successful SMTP DATA response nor a health
response proves delivery to an arbitrary recipient's inbox.

## Existing installations

On the computer running MyAccounts, sign in as the global administrator, open
**Administration** (`/operations`), and select **Connect email**. A private loopback wizard
uses the administrator address already saved for the installation. After a
successful code confirmation it applies the connection to the two default
MyAccounts applications. A CLI alternative is:

```powershell
.\MyAccounts.exe -data F:\MyAccounts\install\data email-setup
```

The CLI wizard asks for a restart after connecting; the dashboard wizard applies
it directly. Other OIDC client applications and custom email providers are not
overwritten. Administrators can attach the `admin/myaccounts-email` provider to
their own applications. Connecting email makes password recovery visible on the
default sign-in pages but does **not** enable account registration. Built-in
remains a global-administrator realm and must not allow public signup.

If the shared service is unavailable during first run, choose **Skip email for now**.
Accounts and ordinary sign-in remain usable. Later code delivery failures do not
create verification records or permit password resets without a valid code.
Custom SMTP remains an advanced alternative in setup.

## Managing accounts without email

Sign in as the global administrator and open **Administration** at `/operations`.
Choose **Manage users** to open `/organizations/myaccounts/users`.

- **Add** opens a new-user form. Enter the username, display name, email address
  and initial password, then save. No message is sent; an administrator-created
  account can sign in before its email is verified.
- Open a user to edit profile details and save the changes.
- Use **Modify password...** (or **Set password...**) on the user to choose and
  confirm a new password. The administrator does not need the user's old password
  or a mailbox code. A reset revokes existing MyAccounts sessions.
- Delete a user from the list to remove the account. The built-in administrator
  and your own account are protected from deletion in the list.

Use ordinary accounts in the MyAccounts organization for connected applications.
Keep the built-in realm for administrators. Without email, emailed recovery and
email verification are unavailable; direct administrator password resets work.

## Credential and delivery boundaries

- HTTPS only, certificate validation enabled, no credential-bearing redirects.
- Enrollment sends an email, issuer and hash of an installation-generated
  256-bit credential. A random challenge ID and six-digit emailed code are needed
  to activate it; codes expire after 10 minutes with at most five wrong attempts.
- Server credentials are hashed in SQLite. Installation credentials use DPAPI
  in `secrets/email-relay.dpapi`; they are not exposed in provider API responses.
- The relay accepts only a single recipient, a random message ID, a six-digit code and one of two
  purposes: verification or password reset. It builds fixed text emails itself.
  Arbitrary subjects, sender identities, HTML, links, invitations and bulk mail
  are rejected. Recovery continues through the existing atomic, expiring engine
  verification/reset grants; the relay cannot reset a password itself.
- Request retries for the same installation/message ID and matching payload are deduplicated
  for 24 hours after a confirmed delivery. SMTP failures are not retried blindly.
  A provider accepting a message just before a crash can still leave delivery
  uncertain; SMTP cannot provide universal exactly-once delivery.
- Persistent hourly limits: 300 total messages, 10 enrollments per source IP,
  three enrollments per administrator email, 20 messages per installation,
  40 messages per administrator email across installations, and five per recipient.
  Active capacity is 1,000 installations. Quotas are charged before delivery,
  survive restarts, and use atomic database transactions. The SMTP transport
  deadline is bounded. These limits are intentionally conservative for the preview.
- Cloudflare client IP headers are trusted only when tunnel mode is enabled and
  the peer is loopback. The origin must remain bound to loopback.
- Reconnection creates a fresh credential and attempts to revoke the prior one
  after the replacement is saved. A revoked/offline old endpoint does not prevent
  reconnecting. The operator can stop the mail service and revoke an ID with
  `MyAccounts-Mail-windows-amd64.exe -data DATA_DIRECTORY -revoke INSTALLATION_ID`.
- The relay sees recipient addresses and transient codes during sending. It
  stores administrator emails and hashed credentials in its protected database;
  message content and raw codes are not logged or retained. Do not enable proxy
  request-body logging. The shared operator is part of the email trust boundary.
- MyAccounts encrypted backups convert the DPAPI installation credential to a
  portable value **inside the encrypted archive**, then re-protect it on restore.
  Plaintext portable files and transient setup URLs are not left in restored data.
  A revoked credential requires reconnecting after restore.

## Verification

`go test ./internal/mailrelay ./internal/mailservice ./internal/portable ./cred .`
checks enrollment ownership/attempts/expiry, unauthorized and revoked delivery,
cross-installation isolation, retry deduplication, quotas across restart, strict
payloads, HTTPS/redirect boundaries, protected wizard storage and encrypted restore.
`node integration/setup-wizard.mjs` exercises desktop/mobile setup with a UI-only
fixture. `scripts/smoke.ps1 -SharedEmail -OIDC` adds a synthetic protected relay
credential to a disposable identity installation and checks default provider,
recovery and registration policy alongside the existing identity integrations.

The optional `integration/mailrelay-fixture` uses a private loopback TLS CA and
a protected message sink to exercise the complete native signup/reset flow through
the real relay HTTP protocol, without sending external email. It does not change
the Windows certificate store or disable certificate verification.

Local fixtures do not establish public mail-service availability or actual inbox
delivery. Record those only after the service owner completes the real wizard.
