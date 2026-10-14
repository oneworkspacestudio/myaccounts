# Security policy

This project is a development build and has not passed its production acceptance gates. Report suspected vulnerabilities privately to the repository owner using GitHub's private security reporting when available. Do not post databases, token values, passwords, key material or exploit captures containing personal data in public issues.

Keep live data on a protected NTFS directory, use HTTPS for public issuers, and maintain verified encrypted backups. Restrict access to the executable and data path. Keep Casdoor and its dependency graph current and audited. The SQLite database contains signing keys and other secrets; directory ACLs and optional volume encryption protect it, while Windows DPAPI protects launcher credentials. A Windows administrator can access process memory and protected resources.

Release must be blocked for unresolved critical/high reachable dependency vulnerabilities, failure of OIDC validation or CSRF/redirect/replay tests, accidental secret publication, missing clean Windows smoke evidence, data-loss migrations, or failed backup recovery. Private security reporting must be configured if a GitHub repository is created.
