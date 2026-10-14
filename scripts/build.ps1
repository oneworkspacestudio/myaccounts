param([string]$Go='go',[string]$Version='0.1.0-dev')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
Push-Location $root
try {
 Push-Location web
 try {
  npx --yes yarn@1.22.22 install --frozen-lockfile --ignore-scripts
  if($LASTEXITCODE){throw 'Frontend dependency installation failed'}
  npx --yes yarn@1.22.22 build
  if($LASTEXITCODE){throw 'Frontend build failed'}
 } finally {Pop-Location}
 $env:CGO_ENABLED='0'
 & $Go test ./internal/mailrelay ./internal/mailservice ./internal/portable ./cred . ./cmd/myaccounts-mail
 if($LASTEXITCODE){throw 'Tests failed'}
 & $Go vet ./internal/mailrelay ./internal/mailservice ./internal/portable . ./cmd/myaccounts-mail
 if($LASTEXITCODE){throw 'Static analysis failed'}
 New-Item -ItemType Directory dist -Force | Out-Null
 & $Go build -trimpath -ldflags "-s -w -X main.version=$Version" -o dist/MyAccounts-windows-amd64.exe .
 if($LASTEXITCODE){throw 'Native build failed'}
 & $Go build -trimpath -ldflags '-s -w' -o dist/MyAccounts-Mail-windows-amd64.exe ./cmd/myaccounts-mail
 if($LASTEXITCODE){throw 'Shared email service build failed'}
 $mailSum=(Get-FileHash dist/MyAccounts-Mail-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
 $sum=(Get-FileHash dist/MyAccounts-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
 [System.IO.File]::WriteAllText((Join-Path $root 'dist/SHA256SUMS.txt'),"$sum  MyAccounts-windows-amd64.exe`n$mailSum  MyAccounts-Mail-windows-amd64.exe`n",(New-Object System.Text.UTF8Encoding($false)))
 Write-Output 'Native executable built. Run scripts/smoke.ps1 against a fresh NTFS data directory.'
} finally {Pop-Location}
