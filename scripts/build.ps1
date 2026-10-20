param([string]$Go='go',[string]$Version='0.1.0-dev',[string]$WebBuild='',[string]$YarnPath='')
$ErrorActionPreference='Stop'
$root=Split-Path $PSScriptRoot -Parent
Push-Location $root
try {
 $canonicalWeb=Join-Path $root 'web'
 if(!$WebBuild){$WebBuild=$canonicalWeb}
 $WebBuild=[IO.Path]::GetFullPath($WebBuild)
 if($WebBuild -cne [IO.Path]::GetFullPath($canonicalWeb)){
  # A named NTFS browser work directory avoids exFAT metadata/lease failures.
  # It must contain exactly the canonical inputs; never accept stale UI assets.
  foreach($file in Get-ChildItem -LiteralPath $canonicalWeb -File -Recurse){
   $relative=[IO.Path]::GetRelativePath($canonicalWeb,$file.FullName)
   if($relative -match '^(node_modules|build|build-temp)[\\/]'){continue}
   $copy=Join-Path $WebBuild $relative
   if(!(Test-Path -LiteralPath $copy) -or (Get-FileHash -LiteralPath $file.FullName).Hash -cne (Get-FileHash -LiteralPath $copy).Hash){throw ('Browser build input differs: '+$relative)}
  }
 }
 function Run-Yarn {
  if($YarnPath){& node $YarnPath @args}else{& npx --yes yarn@1.22.22 @args}
  if($LASTEXITCODE){throw 'Pinned Yarn command failed'}
 }
 Push-Location $WebBuild
 try {
  Run-Yarn install --frozen-lockfile --ignore-scripts
  Run-Yarn build
 } finally {Pop-Location}
 if($WebBuild -cne [IO.Path]::GetFullPath($canonicalWeb)){
  $output=Join-Path $canonicalWeb 'build'
  if([IO.Path]::GetFullPath($output) -cne ([IO.Path]::GetFullPath($canonicalWeb)+[IO.Path]::DirectorySeparatorChar+'build')){throw 'Invalid generated UI output path'}
  if(Test-Path -LiteralPath $output){
   $entries=@(Get-Item -LiteralPath $output)+@(Get-ChildItem -LiteralPath $output -Recurse -Force)
   if(@($entries|Where-Object{$_.Attributes -band [IO.FileAttributes]::ReparsePoint}).Count){throw 'Refuse to replace a linked UI output directory'}
   Remove-Item -LiteralPath $output -Recurse -Force
  }
  Copy-Item -LiteralPath (Join-Path $WebBuild 'build') -Destination $output -Recurse
 }
 $env:CGO_ENABLED='0'
 & $Go test ./internal/mailrelay ./internal/mailservice ./internal/portable ./cred . ./cmd/myaccounts-mail
 if($LASTEXITCODE){throw 'Tests failed'}
 & $Go test ./object -run '^TestAuthorizationCodeRedirectRequiresExactOriginalRequest$' -count=1
 if($LASTEXITCODE){throw 'Authorization-code binding tests failed'}
 & $Go vet ./internal/mailrelay ./internal/mailservice ./internal/portable . ./cmd/myaccounts-mail
 if($LASTEXITCODE){throw 'Static analysis failed'}
 New-Item -ItemType Directory dist -Force | Out-Null
 & $Go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=$Version" -o dist/MyAccounts-windows-amd64.exe .
 if($LASTEXITCODE){throw 'Native build failed'}
 & $Go build -buildvcs=false -trimpath -ldflags '-s -w' -o dist/MyAccounts-Mail-windows-amd64.exe ./cmd/myaccounts-mail
 if($LASTEXITCODE){throw 'Shared email service build failed'}
 $mailSum=(Get-FileHash dist/MyAccounts-Mail-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
 $sum=(Get-FileHash dist/MyAccounts-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
 [System.IO.File]::WriteAllText((Join-Path $root 'dist/SHA256SUMS.txt'),"$sum  MyAccounts-windows-amd64.exe`n$mailSum  MyAccounts-Mail-windows-amd64.exe`n",(New-Object System.Text.UTF8Encoding($false)))
 $source=& git -c ('safe.directory='+$root.Replace('\','/')) rev-parse HEAD
 $dirty=[bool](& git -c ('safe.directory='+$root.Replace('\','/')) status --porcelain)
 @{version=$Version;sourceCommit=$source;sourceDirty=$dirty;githubActionsUsed=$false;frontendBuiltBeforeNative=$true;executableSHA256=$sum;mailExecutableSHA256=$mailSum}|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $root 'dist/build-info.json') -Encoding utf8
 Write-Output 'Native executable built. Run scripts/smoke.ps1 against a fresh NTFS data directory.'
} finally {Pop-Location}
