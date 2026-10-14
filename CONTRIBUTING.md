# Contributing

Read the original brief, architecture and acceptance matrix before changing the product. Preserve upstream copyright/license notices. Record local changes relative to the pinned Casdoor commit; do not merge upstream migrations without backup and no-data-loss checks.

Use pinned Go/Node/Yarn toolchains. Build the frontend before Go compilation, format Go changes with gofmt and run the portable security tests, relevant upstream tests, vet, dependency vulnerability audit, executable smoke tests and visual review. Database-dependent upstream tests must run in isolated disposable installations, never against user data.

Never commit live databases, credentials, sessions, signing keys or data folders. Add focused regression coverage when modifying security or persistence boundaries. Maintain `agents/history/` after material work. Publishing a repository, provisioning cloud resources and publishing a release require the authorization specified in the brief. A passing build is not a production release approval.
