# Chat native-client OIDC callback binding — active

## Scope and source

The October19 Chat parity brief requires public-client MyAccounts OIDC/PKCE,
validated callbacks, native clients and local releases. Current user authorizes
changed repositories, push/releases; no Actions. Use repository-history and
protected synthetic tests. Do not treat this prerequisite as a native app.

Fresh checkout on I from oneworkspacestudio/myaccounts, default mainf03c0ad
(source executable tagpreview-20261015.1 is25b6312). Branchcodex/chat-native-auth.
The old I:/Projects/myaccounts is an uncommitted older snapshot and is preserved;
H:/F: locations in older histories are historical, superseded by the current
user instruction to work on I and use named C NTFS scratch only when required.
No owner account database or live MyAccounts was used. No provider release yet.

## Actual finding

The actual public MyAccounts executable in the isolated Workspace42 fixture
passed browser S256 code exchange with NO client secret and NO copied browser
cookies; RS256/JWKS signature, issuer, audience and nonce checked. Used code replay
and wrong verifier were denied. A valid verifier with a changed token-request
redirect_uri was accepted: existing provider omission, not introduced by Chat5.
RFC6749 sections4.1.3/10.6 and RFC9700 require binding the actual original URI,
not merely checking the app's other registered callbacks.

Source already uses an atomic claimAuthorizationCode CAS; preserve that and test
concurrent exchange rather than duplicating it. GetOAuthCode already receives
redirectUri but Token did not retain it; token controller did not pass it onward.

## Current changes, not yet verified or committed

Token adds private JSON-omitted code_redirect_uri text column. Authorization
stores original URI; form/JSON token input threads it to code validation before
PKCE/atomic consumption. Exact comparison rejects missing/changed/legacy-unbound
codes. Existing access/refresh tokens and identities remain. Old pending codes
must restart authorization. Invalid-code/verifier errors no longer echo sensitive
code/challenge values. Pure exact-binding table tests added. Need actual SQLite
migration and full provider/browser/client regression; do not claim success yet.

Unit compile/test running via native-provider-unit.log on I task tooling.
Frontend must be built before native executable per AGENTS; sources copied to
C:/Projects/.cache/chat-parity-20261019/myaccounts-web; pinned Yarn build running.
Initial npx using I npm cache failed ECOMPROMISED in libnpmexec.touchLock (lease
update), not an audited package compromise. Retry isolates only npm exec cache
on named C/myaccounts-npm-exec; Yarn/downloads/output cache remains on I. Never
clear unknown global caches. Source and final artifacts on I.

## Test/release coordination

Root task tooling I:/Projects/tooling/chat-parity-20261019; detailed Chat handoff
in I:/Projects/workspace-chat/agents/history/2026-10-19-whatsapp-parity-compacted.md.
Chat5 and Workspace43 incomplete foundation candidates are now public. They do
not ship native clients or application E2EE; all broader mandatory work remains.
Actual public43 installer/bundle hashes were downloaded and verified; paired
upgrade and default-online fresh selected install are running in separate task
fixtures. Owner42 and remote health remain unchanged/healthy. Do not replace a
fixture's provider during its published-download acceptance. Build/test this
provider in another protected synthetic fixture after browser assets finish.

Native OIDC harness is a new uncommitted Chat script. It currently asserts the
missing callback check and fails against public provider, as expected. Earlier
callback timeouts were harness routing: Playwright route does not intercept a
redirect chain's subsequent callback. Listening to the actual callback request
also handles silent HTTP302 SSO; no bearer credential is logged. Code/token stay
in memory; only public client ID and endpoints in protected fixture config.

## Next

Finish Go unit + pinned web build; compile native preview and run real positive,
wrong/missing/alternate-registered redirect, wrong verifier, code replay/concurrent
claim, JSON/form input, legacy-code migration, existing sessions/refresh and
managed app callbacks. Reuse upstream OIDC/crypto and preserve license notices.
No v0.1.0 production claim; signing/clean-host/service/reboot and full native app
prerequisites remain unverified. Commit/publish only tested exact sources with
truthful candidate scope; eventual Workspace pin must reference released bytes.

## October20 actual development acceptance

Frozen UI install/typecheck/build passed on named NTFS copy; only767 locked Yarn
transport URLs changed to registry.npmjs.org, versions/integrities unchanged.
Root native tests and vet passed. Whole-object focused unit passed. Root compile
caught the internal CasWAF caller; it now supplies its constructed original callback.
Git VCS stamping initially failed on safe.directory; development retry used scoped
Git config. Build script now uses explicit source/hash receipt and buildvcs=false,
supports verified -WebBuild and pinned -YarnPath, runs binding tests in addition to
native regression/vet. No global security/tooling settings were changed.

Actual development executable SHAde9824b21011e1f28782bf3499d1bc629caa7a18b7468344e49e1931a99c2cc4
upgraded the existing primary synthetic public43 provider database. Real S256
public exchange, RS256/JWKS/issuer/audience/nonce, code replay, wrong verifier,
missing/changed/other-registered redirect, form+JSON and concurrent single claim
all passed. Legacy pending code rejected; prior issued access token/subject and
browser SSO retained. Chat state/actual UI document bytes/filename passed after.
First assertion incorrectly required synchronous redirect before provider JS SSO
finished; harness now awaits callback without entering another password. No owner
data used. Primary30960 is now MANUAL provider prototype (not public43 exactpair).

The external helper's spawned server inherited pipe closure; initial run waited
after helper exit but eventually completed within150s and every assertion passed.
Harness now waits spawn exit with ignored pipes, preserving bounded timeout.
Do not rerun upgrade mode on the already-upgraded fixture or overwrite baseline.
Normal negative-only mode passed again. Final clean source build, actual released
downloads/fresh+upgrade, publication and Workspace pin remain required. No provider
commit/tag/release yet. Chat5/Workspace43 are public incomplete foundation releases.
