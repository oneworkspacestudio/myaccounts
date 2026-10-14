# Application integration

Create a separate application/client in the MyAccounts Applications console for every relying party. Choose your organization, signing certificate, standard `openid profile email` scopes and **exact** redirect URIs. Generate a fresh client secret, retain it only in the relying party's protected configuration, and enable authorization code flow. Use PKCE S256 when the client supports it. Never use wildcard redirects. Never give applications an account password.

The provider discovery document is `ISSUER/.well-known/openid-configuration`. Use its advertised endpoints; do not copy endpoint guesses into clients. Confirm its `issuer` equals the public issuer exactly. Casdoor exposes application registration, client secrets, consent settings, token fields and rotation controls; these controls still need independent-client acceptance testing in this package.

## FileBrowser Quantum

Verified configuration names against [official OIDC documentation](https://filebrowserquantum.com/en/docs/configuration/authentication/oidc/) and [configuration-file documentation](https://filebrowserquantum.com/en/docs/getting-started/config/) on 2026-10-14. A disposable native drive-office-v0.3.1 build passed the real callback/session/stable-sub/normal-user tests; see `integration/filebrowser.mjs`.

Register `https://files.example.com/api/auth/oidc/callback` for a root-hosted instance, or append that suffix after your configured base URL (for example `/files/api/auth/oidc/callback`). FileBrowser derives its callback from the browser-facing request URL rather than `http.externalUrl`.

```yaml
auth:
  methods:
    oidc:
      enabled: true
      clientId: "YOUR_MYACCOUNTS_CLIENT_ID"
      issuerUrl: "https://login.example.com"
      scopes: "openid email profile"
      userIdentifier: "sub"
      disableVerifyTLS: false
```

Supply `FILEBROWSER_OIDC_CLIENT_SECRET` through the environment. Behind a trusted reverse proxy, FileBrowser v2 uses `http.trustProxyHeaders: true`; restrict access to that proxy. Older v1 releases use a forwarded-header list, so check the version before editing. Grant new users only the intended sources using `server.sources[].config.defaultEnabled`, or assign them manually. Authentication does not grant file access automatically. Preserve the password administrator until SSO and recovery are confirmed.

Group authorization additionally requires a verified `groups` claim and corresponding `adminGroup`, `userGroups` or `groupsClaim` configuration. Do not grant administrator access based on an untested claim mapping. Back up configuration before any change. MyAccounts does not automatically modify FileBrowser configuration.

## Gitea

From [official Gitea authentication documentation](https://docs.gitea.com/next/administration/authentication/): Site Administration → Identity & Access → Authentication Sources → Add Authentication Source, type OAuth2, provider OpenID Connect. Enter the client ID, secret and MyAccounts discovery URL. Name the authentication source `myaccounts`, and register `https://git.example.com/user/oauth2/myaccounts/callback` (using Gitea's actual ROOT_URL). Keep Gitea's existing administrator and validate account linking before disabling other login methods. Official Gitea 28.1.0 passed a disposable native callback, automatic registration, web session, stable-sub linkage and non-admin test; see `integration/gitea.mjs`. Set `[oauth2_client] USERNAME = userid` for stable subject linkage and review registration/linking policy. Isolated tests must set `SSH_ROOT_PATH`, `SSH_CREATE_AUTHORIZED_KEYS_FILE = false` and `SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE = false`; `DISABLE_SSH` alone does not prevent key-file rewriting.

## Generic clients and custom applications

Use a maintained OIDC client for your language. For Node, use [openid-client](https://github.com/panva/openid-client); for Python, [Authlib](https://docs.authlib.org/en/latest/client/frameworks.html). Discover the provider, register a server-side callback, store state/nonce and the PKCE verifier in a protected server session, request authorization code flow, validate the returned state, exchange the code with PKCE and verify the ID token through the client library. Use stable `sub` for account linkage; do not key identities by editable email alone. Pin the library version and follow its current official API; The runnable Node regression harness is `integration/oidc.mjs`; a standalone Python example remains pending.

Account switching must begin a normal client authorization request with suitable prompt behavior. An app launcher cannot substitute identity by editing cookies. Applications without OIDC may use Casdoor's supported forward-auth/reverse-proxy integration after per-application testing; universal SSO is not promised. Automated application discovery and configuration generation are not implemented.
