# MyAccounts repository instructions

Use the repository-history skill before substantive repository work and maintain the compact handoff after material work. Read `agents/history/` and reconcile it with Git state.

Preserve the supplied brief and upstream license notices. The Go module path is retained for Casdoor imports. MyAccounts must reuse upstream OIDC and cryptography rather than inventing a protocol implementation.

Build `web/` before compiling the embedded executable. Run identity and migration tests only against disposable protected directories. `MYACCOUNTS_TEST_DATA` can choose an NTFS test location independently of Go's build temporary directory. Never test against an existing live account database.

Keep `docs/acceptance.md` and `docs/release-acceptance.json` honest. Do not mark an unperformed clean-host, service/reboot, integration, cloud or release test passed. Publish v0.1.0 only when the stated gates pass. GitHub and Cloudflare account actions require the authorization specified in the original brief.
