param([Parameter(Mandatory=$true)][string]$Exe,[string]$Root,[switch]$OIDC,[string]$Checks="all")
$ErrorActionPreference='Stop'
$Exe=(Resolve-Path -LiteralPath $Exe).Path
if (!$Root) { $Root=Join-Path ([System.IO.Path]::GetTempPath()) ('myaccounts-smoke-'+[Guid]::NewGuid()) }
if (Test-Path -LiteralPath $Root) { throw 'Smoke test requires a fresh directory' }
New-Item -ItemType Directory -Path (Join-Path $Root 'secrets') -Force | Out-Null
$port=18787
$origin="http://localhost:$port"
$password=[Guid]::NewGuid().ToString()+[Guid]::NewGuid().ToString()
$config=@{version=1;issuer=$origin;port=$port;adminEmail='admin@example.com';adminName='Smoke Admin';tunnelMode='local'}
$utf8=New-Object System.Text.UTF8Encoding($false)
[System.IO.File]::WriteAllText((Join-Path $Root 'config.json'),($config|ConvertTo-Json),$utf8)
Add-Type -AssemblyName System.Security.Cryptography.ProtectedData
$bootstrap=[System.Text.Encoding]::UTF8.GetBytes((@{name='Smoke Admin';email='admin@example.com';password=$password}|ConvertTo-Json))
$protected=[System.Security.Cryptography.ProtectedData]::Protect($bootstrap,$null,[System.Security.Cryptography.DataProtectionScope]::LocalMachine)
[System.IO.File]::WriteAllBytes((Join-Path $Root 'secrets/bootstrap.dpapi'),$protected)
$restoreProcess=$null
$process=Start-Process -FilePath $Exe -ArgumentList @('-data',('"'+$Root+'"'),'-no-browser') -WindowStyle Hidden -PassThru
try {
 $ready=$false
 for ($n=0;$n -lt 120;$n++) { if ($process.HasExited) { throw 'Executable stopped; inspect smoke data logs' };try{$health=Invoke-RestMethod "$origin/healthz";if($health.status -eq 'ok'){$ready=$true;break}}catch{};Start-Sleep -Milliseconds 500 }
 if (!$ready) { throw 'Health timeout' }
 $discovery=Invoke-RestMethod "$origin/.well-known/openid-configuration"
 if ($discovery.issuer -ne $origin) { throw 'Discovery issuer mismatch' }
 $jwks=Invoke-RestMethod $discovery.jwks_uri
 if (!$jwks.keys -or $jwks.keys.Count -lt 1) { throw 'No signing key published' }
 if (Test-Path (Join-Path $Root 'secrets/bootstrap.dpapi')) { throw 'Bootstrap credential was not removed' }
 $login=@{type='login';signinMethod='password';organization='built-in';username='admin@example.com';password=$password;application='app-built-in';autoSignin=$false}|ConvertTo-Json
 $result=Invoke-RestMethod "$origin/api/login" -Method Post -Body $login -ContentType 'application/json' -Headers @{Origin=$origin} -SessionVariable browserSession
 if ($result.status -ne 'ok') { throw ('Administrator login failed: '+$result.msg) }
 $sessionCookies=@($browserSession.Cookies.GetCookies([Uri]$origin)|Where-Object Name -like '*session*')
 if(!$sessionCookies -or @($sessionCookies|Where-Object {!$_.HttpOnly}).Count){throw 'Session cookie is not HttpOnly'}
 $account=Invoke-RestMethod "$origin/api/get-account" -WebSession $browserSession
 if ($account.status -ne 'ok' -or $account.data.email -ne 'admin@example.com') { throw 'Session account mismatch' }
 $account.data.displayName='Updated Smoke Admin'
 $profile=Invoke-RestMethod "$origin/api/update-user?id=built-in/admin&columns=displayName" -Method Post -Body ($account.data|ConvertTo-Json -Depth 64) -ContentType 'application/json' -Headers @{Origin=$origin} -WebSession $browserSession
 if($profile.status -ne 'ok'){throw 'Profile edit failed'}
 $account=Invoke-RestMethod "$origin/api/get-account" -WebSession $browserSession
 if($account.data.displayName -ne 'Updated Smoke Admin'){throw 'Profile edit did not persist'}
 $invalid=@{type='login';signinMethod='password';organization='built-in';username='admin@example.com';password='wrong-test-password';application='app-built-in'}|ConvertTo-Json
 $result=Invoke-RestMethod "$origin/api/login" -Method Post -Body $invalid -ContentType 'application/json' -Headers @{Origin=$origin}
 if ($result.status -eq 'ok') { throw 'Invalid credentials accepted' }
 $duplicate=Start-Process -FilePath $Exe -ArgumentList @('-data',('"'+$Root+'"'),'-no-browser') -WindowStyle Hidden -PassThru -Wait
 if ($duplicate.ExitCode -eq 0) { throw 'Duplicate instance accepted' }
 if (!(Test-Path (Join-Path $Root 'users.db'))) { throw 'No SQLite persistence' }
 if ($OIDC) {
  $env:MYACCOUNTS_TEST_ISSUER=$origin
  $env:MYACCOUNTS_TEST_PASSWORD=$password
  $env:MYACCOUNTS_FIXTURE=$Root
  $env:MYACCOUNTS_EXE=$Exe
  try {
   if($Checks -eq 'all' -or $Checks -eq 'oidc'){node (Join-Path $PSScriptRoot '../integration/oidc.mjs');if($LASTEXITCODE){throw 'Independent OIDC test failed'}}
   if($Checks -eq 'all' -or $Checks -eq 'mfa'){node (Join-Path $PSScriptRoot '../integration/mfa.mjs');if($LASTEXITCODE){throw 'Independent MFA test failed'}}
   if($Checks -eq 'all' -or $Checks -eq 'operations'){node (Join-Path $PSScriptRoot '../integration/operations.mjs');if($LASTEXITCODE){throw 'Operations test failed'}}
   if($Checks -eq 'all' -or $Checks -eq 'group-mfa'){node (Join-Path $PSScriptRoot '../integration/group-mfa.mjs');if($LASTEXITCODE){throw 'Group MFA test failed'}}
   if($Checks -eq 'all' -or $Checks -eq 'sessions'){node (Join-Path $PSScriptRoot '../integration/sessions.mjs');if($LASTEXITCODE){throw 'Session revocation test failed'}}
   # Pace the combined fixture through the real 30-attempt login window.
   # Individual suites do not need this pause; no production bypass is added.
   if($Checks -eq 'all' -and $env:MYACCOUNTS_PLAYWRIGHT){Start-Sleep -Seconds 60}
   if($env:MYACCOUNTS_PLAYWRIGHT -and ($Checks -eq 'all' -or $Checks -eq 'passkeys')){node (Join-Path $PSScriptRoot '../integration/passkeys.mjs');if($LASTEXITCODE){throw 'Passkey browser test failed'}}
   if($env:MYACCOUNTS_PLAYWRIGHT -and ($Checks -eq 'all' -or $Checks -eq 'consent')){node (Join-Path $PSScriptRoot '../integration/consent-browser.mjs');if($LASTEXITCODE){throw 'Consent browser test failed'}}
   if($Checks -eq 'all' -or $Checks -eq 'scheduled-backup'){node (Join-Path $PSScriptRoot '../integration/scheduled-backup.mjs');if($LASTEXITCODE){throw 'Scheduled backup test failed'}}
   if($Checks -eq 'all' -or $Checks -eq 'email-recovery'){node (Join-Path $PSScriptRoot '../integration/email-recovery.mjs');if($LASTEXITCODE){throw 'Email recovery fixture test failed'}}
   if($env:MYACCOUNTS_FILEBROWSER_EXE){$env:MYACCOUNTS_FILEBROWSER_DATA=Join-Path $Root 'filebrowser-integration';node (Join-Path $PSScriptRoot '../integration/filebrowser.mjs');if($LASTEXITCODE){throw 'FileBrowser integration failed'}}
   if($env:MYACCOUNTS_GITEA_EXE){$env:MYACCOUNTS_GITEA_DATA=Join-Path $Root 'gitea-integration';node (Join-Path $PSScriptRoot '../integration/gitea.mjs');if($LASTEXITCODE){throw 'Gitea integration failed'}}
  } finally {Remove-Item Env:MYACCOUNTS_TEST_PASSWORD;Remove-Item Env:MYACCOUNTS_TEST_ISSUER;Remove-Item Env:MYACCOUNTS_FIXTURE;Remove-Item Env:MYACCOUNTS_EXE}
 }
 foreach($logFile in Get-ChildItem (Join-Path $Root 'logs') -Filter '*.log'){if((Get-Content -LiteralPath $logFile.FullName -Raw).Contains($password)){throw 'Plaintext password leaked into logs'}}
 # Integration tests may add signing keys; compare the completed installation.
 $jwks=Invoke-RestMethod $discovery.jwks_uri
 & $Exe -data $Root stop
 if($LASTEXITCODE){throw 'Clean shutdown failed'}
 $process.WaitForExit(10000)|Out-Null
 if(!$process.HasExited -or $process.ExitCode -ne 0){throw 'Launcher did not exit cleanly'}
 $phrasePath=Join-Path (Split-Path $Root -Parent) ('myaccounts-phrase-'+[Guid]::NewGuid()+'.txt')
 [System.IO.File]::WriteAllText($phrasePath,([Guid]::NewGuid().ToString()+[Guid]::NewGuid().ToString()),$utf8)
 $archive=$Root+'.age';$restored=$Root+'-restored'
 try {
  & $Exe -data $Root backup $archive $phrasePath
  if($LASTEXITCODE){throw 'Native encrypted backup failed'}
  & $Exe -data $restored restore $archive $phrasePath
  if($LASTEXITCODE){throw 'Native encrypted restore failed'}
  if((Get-FileHash (Join-Path $Root 'users.db')).Hash -ne (Get-FileHash (Join-Path $restored 'users.db')).Hash){throw 'Restored database differs'}
  $restoreProcess=Start-Process -FilePath $Exe -ArgumentList @('-data',('"'+$restored+'"'),'-no-browser') -WindowStyle Hidden -PassThru
  $restoreReady=$false
  for($n=0;$n -lt 120;$n++){if($restoreProcess.HasExited){throw 'Restored installation stopped'};try{if((Invoke-RestMethod "$origin/healthz").status -eq 'ok'){$restoreReady=$true;break}}catch{};Start-Sleep -Milliseconds 500}
  if(!$restoreReady){throw 'Restored startup timed out'}
  $restoredLogin=Invoke-RestMethod "$origin/api/login" -Method Post -Body $login -ContentType 'application/json' -Headers @{Origin=$origin} -SessionVariable restoredSession
  if($restoredLogin.status -ne 'ok'){throw 'Restored identity cannot log in'}
  $restoredAccount=Invoke-RestMethod "$origin/api/get-account" -WebSession $restoredSession
  if($restoredAccount.data.id -ne $account.data.id -or $restoredAccount.data.displayName -ne 'Updated Smoke Admin'){throw 'Restored identity changed'}
  $restoredKeys=Invoke-RestMethod $discovery.jwks_uri
  if(($restoredKeys|ConvertTo-Json -Depth 64 -Compress) -ne ($jwks|ConvertTo-Json -Depth 64 -Compress)){throw 'Restored signing keys changed'}
  & $Exe -data $restored stop
  if($LASTEXITCODE){throw 'Restored shutdown failed'}
  $restoreProcess.WaitForExit(10000)|Out-Null
  if(!$restoreProcess.HasExited -or $restoreProcess.ExitCode -ne 0){throw 'Restored launcher did not stop cleanly'}

 } finally {Remove-Item -LiteralPath $phrasePath -ErrorAction SilentlyContinue}
 Write-Output 'PASS: native startup, admin bootstrap, email/password login, profile, session, failed credentials, discovery/JWKS, duplicate-instance refusal, clean stop, encrypted SQLite backup/restore, restored identity login and signing-key preservation'
} finally {
 if($restoreProcess -and !$restoreProcess.HasExited){& $Exe -data $restored stop | Out-Null; $restoreProcess.WaitForExit(10000)|Out-Null; if(!$restoreProcess.HasExited){Stop-Process -Id $restoreProcess.Id -Force -ErrorAction SilentlyContinue}}
 # The engine shutdown token is process-only. The smoke harness stops its own child processes.
 if(!$process.HasExited){& $Exe -data $Root stop | Out-Null}
 $children=Get-CimInstance Win32_Process | Where-Object ParentProcessId -eq $process.Id
 foreach($child in $children){Stop-Process -Id $child.ProcessId -Force -ErrorAction SilentlyContinue}
 if (!$process.HasExited) { Stop-Process -Id $process.Id -Force -ErrorAction SilentlyContinue }
}
