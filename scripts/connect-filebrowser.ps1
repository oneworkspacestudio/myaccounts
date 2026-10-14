param(
 [Parameter(Mandatory=$true)][string]$AdminVault,
 [Parameter(Mandatory=$true)][string]$AdminAPI,
 [Parameter(Mandatory=$true)][string]$Issuer,
 [Parameter(Mandatory=$true)][string]$FileBrowserExe,
 [Parameter(Mandatory=$true)][string]$Config,
 [Parameter(Mandatory=$true)][string]$Origin,
 [Parameter(Mandatory=$true)][string]$ClientName,
 [Parameter(Mandatory=$true)][int]$Port,
 [Parameter(Mandatory=$true)][string]$DriveRepo,
 [switch]$CreateRecoveryAdministrator
)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Security.Cryptography.ProtectedData
$utf8=New-Object Text.UTF8Encoding($false)
$private=Split-Path (Resolve-Path -LiteralPath $AdminVault).Path -Parent
$vault=$utf8.GetString([Security.Cryptography.ProtectedData]::Unprotect([IO.File]::ReadAllBytes($AdminVault),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine))|ConvertFrom-Json
$handoff=Join-Path $private ($ClientName+'-'+[Guid]::NewGuid()+'.json')
$protectedClient=Join-Path $private ($ClientName+'.dpapi')
if(Test-Path $protectedClient){throw 'This client already has a protected handoff; refuse replacement'}
$FileBrowserExe=(Resolve-Path -LiteralPath $FileBrowserExe).Path
$Config=(Resolve-Path -LiteralPath $Config).Path
$listener=Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction Stop
if(@($listener).Count -ne 1){throw 'Expected one preview listener'}
$owned=Get-CimInstance Win32_Process -Filter ('ProcessId='+$listener.OwningProcess)
if($owned.ExecutablePath -ne $FileBrowserExe){throw 'Listener is not the selected FileBrowser executable'}
try{
 $env:MYACCOUNTS_ADMIN_API=$AdminAPI;$env:MYACCOUNTS_ISSUER=$Issuer
 $env:MYACCOUNTS_ADMIN_EMAIL=$vault.email;$env:MYACCOUNTS_ADMIN_PASSWORD=$vault.password
 $env:FILEBROWSER_ORIGIN=$Origin;$env:FILEBROWSER_CLIENT_NAME=$ClientName;$env:MYACCOUNTS_CLIENT_OUTPUT=$handoff
 node (Join-Path $PSScriptRoot '../integration/register-filebrowser.mjs')
 if($LASTEXITCODE){throw 'Client registration failed'}
 $clientText=Get-Content -LiteralPath $handoff -Raw
 $protected=[Security.Cryptography.ProtectedData]::Protect($utf8.GetBytes($clientText),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine)
 [IO.File]::WriteAllBytes($protectedClient,$protected)
 $client=$clientText|ConvertFrom-Json
 [IO.File]::WriteAllText($handoff,(@{issuer=$client.issuer;clientId=$client.clientId;callback=$client.callback}|ConvertTo-Json),$utf8)
 $backup=Join-Path $private ($ClientName+'-before-sso')
 $patchArgs=@((Join-Path $DriveRepo 'scripts/configure-myaccounts.py'),'--config',$Config,'--client',$handoff,'--backup',$backup,'--port',$Port)
 if($CreateRecoveryAdministrator){$patchArgs+=@('--recovery-user',$vault.filebrowserRecoveryUsername)}
 python @patchArgs
 if($LASTEXITCODE){throw 'Configuration validation/backup failed'}
 Stop-Process -Id $owned.ProcessId
 if($CreateRecoveryAdministrator){
  Push-Location (Split-Path $Config -Parent)
  try{$vault.filebrowserRecoveryPassword | & $FileBrowserExe -c $Config user set $vault.filebrowserRecoveryUsername --password --admin; if($LASTEXITCODE){throw 'Recovery administrator setup failed'}}finally{Pop-Location}
 }
 $shell=(Get-Process -Id $PID).Path
 $runner=Join-Path $DriveRepo 'scripts/run-myaccounts-filebrowser.ps1'
 $arguments=@('-NoProfile','-File',('"'+$runner+'"'),'-Exe',('"'+$FileBrowserExe+'"'),'-Config',('"'+$Config+'"'),'-ProtectedClient',('"'+$protectedClient+'"'))
 $process=Start-Process -FilePath $shell -ArgumentList $arguments -WorkingDirectory (Split-Path $Config -Parent) -WindowStyle Hidden -RedirectStandardOutput (Join-Path $private ($ClientName+'-server.log')) -RedirectStandardError (Join-Path $private ($ClientName+'-error.log')) -PassThru
 for($n=0;$n -lt 120;$n++){if($process.HasExited){throw 'Connected FileBrowser stopped; inspect protected installation log'};try{$r=Invoke-WebRequest "http://localhost:$Port/login" -MaximumRedirection 0 -ErrorAction Stop;if($r.StatusCode -eq 200){Write-Output "FileBrowser connected on port $Port. Settings/config backup: $backup";return}}catch{};Start-Sleep -Milliseconds 500}
 throw 'FileBrowser startup timeout'
}finally{
 Remove-Item -LiteralPath $handoff -ErrorAction SilentlyContinue
 foreach($name in @('MYACCOUNTS_ADMIN_API','MYACCOUNTS_ISSUER','MYACCOUNTS_ADMIN_EMAIL','MYACCOUNTS_ADMIN_PASSWORD','FILEBROWSER_ORIGIN','FILEBROWSER_CLIENT_NAME','MYACCOUNTS_CLIENT_OUTPUT')){Remove-Item ('Env:'+ $name) -ErrorAction SilentlyContinue}
 $vault=$null;$client=$null;$clientText=$null
}
