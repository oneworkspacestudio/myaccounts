.DEFAULT_GOAL := help
.PHONY: help build test
help:
	@echo "MyAccounts: use scripts/build.ps1 for the Windows executable; release requires the acceptance gates."
build:
	pwsh -NoProfile -File scripts/build.ps1
test:
	go test ./internal/portable ./cred .
