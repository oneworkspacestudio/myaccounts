# Connect the custom FileBrowser to MyAccounts

Companion repository: [fahimc/filebrowser](https://github.com/fahimc/filebrowser),
[Drive/Office branch](https://github.com/fahimc/filebrowser/tree/drive-reskin).
The tested executable is the
[drive-office-v0.4.0 preview](https://github.com/fahimc/filebrowser/releases/tag/drive-office-v0.4.0).
These settings target that v2-based custom build. Older upstream versions may use
different proxy-header options.

![FileBrowser sign-in](screenshots/filebrowser-login.png)

## What is shared

MyAccounts authenticates the person and issues an OIDC identity. FileBrowser
validates the response, links its local account by the stable `sub`, and creates
its own session. FileBrowser retains its own sources, permissions, file storage,
Office editing and version history. Authentication does not copy documents into
MyAccounts or grant every user access to every source.

MyAccounts is optional to FileBrowser: its password login can operate separately.
Keep a working FileBrowser password administrator while setting up SSO and recovery.
An existing password account is not automatically merged by matching its email;
plan and test account linkage and source permissions before changing login methods.

## 1. Choose the browser-facing addresses

Use a stable HTTPS MyAccounts issuer such as `https://login.example.com`. Its
metadata is `https://login.example.com/.well-known/openid-configuration`. Both the
browser and FileBrowser backend must be able to reach the issuer. Localhost HTTP
is for same-machine development; localhost on another device is that other device.

Register a different MyAccounts application/client for each FileBrowser instance:

| FileBrowser address | Exact registered redirect URI |
| --- | --- |
| `http://localhost:8810` (same-machine preview) | `http://localhost:8810/api/auth/oidc/callback` |
| `https://files.example.com` | `https://files.example.com/api/auth/oidc/callback` |
| `https://files.example.com/files` | `https://files.example.com/files/api/auth/oidc/callback` |

A temporary tunnel hostname change also changes its callback. Update that one
client's allowed redirect deliberately, or use a permanent hostname. Do not use
wildcards. The callback must match the scheme, host, optional port and base path.

## 2. Register FileBrowser in MyAccounts

1. Sign in to MyAccounts as the global administrator and open **Applications**.
2. Create an application, for example **FileBrowser local**. Set its organization
   to **myaccounts**, where ordinary accounts belong, rather than **built-in**.
3. Keep a valid MyAccounts signing certificate selected. Enable the
   `authorization_code` grant and add the exact redirect URI from the table.
4. Save and copy that application's **Client ID** and generated **Client secret**.
   Use the client ID shown in the registration, not its display name.
5. Create an ordinary test user in **Administration → Manage users**. Set their
   password directly; email delivery and email verification are not prerequisites.

Make independent registrations for local and remote instances. Client secrets
belong only in the relying party's protected server configuration.

## 3. Configure FileBrowser

Back up its YAML and settings database first. Stop the selected instance before
changing its configuration. Merge this into its existing configuration; do not
replace its `server.database`, `server.sources`, editor or history settings.

```yaml
frontend:
  oidcLoginButtonText: Sign in with MyAccounts
auth:
  methods:
    noauth: false
    password:
      enabled: true
      signup: false
    oidc:
      enabled: true
      issuerUrl: https://login.example.com
      clientId: YOUR_FILEBROWSER_CLIENT_ID
      scopes: openid profile email
      userIdentifier: sub
      disableVerifyTLS: false
      adminGroup: ""
      userGroups: []
      groupsClaim: groups
userDefaults:
  account:
    permissions:
      admin: false
      api: false
```

Supply `FILEBROWSER_OIDC_CLIENT_SECRET` in the FileBrowser process environment
from protected local storage, then start its executable with the selected config:

```powershell
# The process environment must already contain FILEBROWSER_OIDC_CLIENT_SECRET.
.\filebrowser.exe -c .\config.yaml
```

Do not put a MyAccounts account password in FileBrowser configuration. Do not
commit the client secret or pass it on the executable command line. A protected
Windows wrapper can read a DPAPI handoff and populate the child environment;
this is how the existing local/remote preview installation was configured.

For a loopback origin behind a trusted proxy or Cloudflare Tunnel, merge:

```yaml
http:
  listen: 127.0.0.1
  trustProxyHeaders: true
```

Preserve the actual port and baseURL. The proxy must forward the external scheme
and host correctly. FileBrowser derives its OIDC callback from the incoming
request; `http.externalUrl` does not replace correct forwarding. Restrict the
origin to the trusted proxy. Keep TLS verification enabled.

These configuration names and callback rules follow the
[official FileBrowser OIDC documentation](https://filebrowserquantum.com/en/docs/configuration/authentication/oidc/)
and the verified custom executable. Configuration layout is documented in the
[official config guide](https://filebrowserquantum.com/en/docs/getting-started/config/).

## 4. Grant file access and verify

Open FileBrowser and choose **Sign in with MyAccounts**. Sign in as the ordinary
test user. After the callback, FileBrowser should show a normal account.

New users receive only intended source access. For a source that every new user
should see, configure that source's `config.defaultEnabled: true`; for restricted
sources, leave it false and assign users explicitly. Never enable broad source
access just to hide a permissions problem. Grant administrator rights separately
only when intended; leave `adminGroup` empty unless its claim mapping is tested.

![Sample files after MyAccounts sign-in](screenshots/filebrowser-files.png)

Before relying on SSO, verify that the intended files are visible, another
administrator's profile is refused, and the separate recovery login still works.
A MyAccounts password reset revokes issuer sessions, but an already established
FileBrowser cookie follows FileBrowser's own session/logout policy.

## Troubleshooting

| Symptom | Check |
| --- | --- |
| No MyAccounts button | OIDC enabled, `frontend.oidcLoginButtonText`, correct config file, restarted instance |
| Redirect URI rejected | Exact browser-facing callback; forwarding of external HTTPS/host; base path and changed tunnel hostname |
| Token/discovery failure | Issuer exact match, backend network access, client ID/secret, signing key and trusted HTTPS certificate |
| Login works but no files | Source permissions and `defaultEnabled`, not the MyAccounts password |
| Unexpected separate local account | Stable `sub` mapping; review linkage to an existing password account |
| Works locally but fails remotely | Separate client registration, remote callback and proxy headers |

Existing local and remote previews passed real Chrome callbacks, matching subject,
normal-user permissions and protected session cookies. A disposable custom v0.4.0
instance also produced these screenshots through an actual MyAccounts browser
sign-in. All pictured accounts and files are synthetic. This does not establish
that every FileBrowser version or deployment topology has been tested.
