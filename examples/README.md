# Relying-party examples

These optional development examples use maintained OIDC clients. The MyAccounts EXE does not need Node or Python. Run the examples with separate client registrations and exact callbacks `http://localhost:18994/callback` (Node) or `http://localhost:18995/callback` (Python). Public deployments need HTTPS, durable protected server-side sessions, deployment supervision and application-specific authorization.

Supply `OIDC_ISSUER`, `OIDC_CLIENT_ID` and `OIDC_CLIENT_SECRET` in the process environment. `RP_ORIGIN` is the browser-facing origin. Python accepts `RP_SESSION_SECRET` for stable signed sessions; choose a strong protected value. Never put an account password in relying-party configuration. Use `sub` for account linkage.

Node: install the locked dependencies under `node/`, then `npm start`. Python: create a Python 3.12 environment, install `python/requirements.txt`, then run `python/app.py`. Each example exposes `/api/me` for smoke verification and a local sign-out. Provider session management remains in MyAccounts.

Authlib owns discovery, PKCE, state and ID-token/nonce validation ([official Flask OIDC documentation](https://docs.authlib.org/en/latest/oauth2/client/web/flask.html)). Node uses openid-client's discovery, code grant, UserInfo and signature checks ([official source/documentation](https://github.com/panva/openid-client)). Credentials and tokens are never included in page HTML or application logs.
