param(
 [Parameter(Mandatory=$true)][string]$Exe,
 [Parameter(Mandatory=$true)][string]$Data,
 [Parameter(Mandatory=$true)][string]$Domain,
 [Parameter(Mandatory=$true)][string]$AdminEmail,
 [string]$AdminName='Administrator',
 [int]$Port=8788,
 [switch]$UseExistingCloudflareLogin
)
$ErrorActionPreference='Stop'
$Exe=(Resolve-Path -LiteralPath $Exe).Path
$Data=[IO.Path]::GetFullPath($Data)
if(Test-Path (Join-Path $Data 'config.json')){throw 'Installation is already configured; no changes made'}
$private=Join-Path (Split-Path $Data -Parent) 'private'
New-Item -ItemType Directory -Path $private,$Data -Force | Out-Null
$sid=[Security.Principal.WindowsIdentity]::GetCurrent().User.Value
& icacls.exe $private /inheritance:r /grant:r "*${sid}:(OI)(CI)F" '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' | Out-Null
if($LASTEXITCODE){throw 'Cannot protect installation credentials'}
Add-Type -AssemblyName System.Security.Cryptography.ProtectedData
$rng=[Security.Cryptography.RandomNumberGenerator]::Create()
function New-InstallPassword { $bytes=New-Object byte[] 48; $rng.GetBytes($bytes); [Convert]::ToBase64String($bytes) }
$password=New-InstallPassword
$recovery=New-InstallPassword
$utf8=New-Object Text.UTF8Encoding($false)
$credentials=@{issuer="https://$Domain";email=$AdminEmail;password=$password;filebrowserRecoveryUsername='myaccounts-recovery-admin';filebrowserRecoveryPassword=$recovery}
$blob=[Security.Cryptography.ProtectedData]::Protect($utf8.GetBytes(($credentials|ConvertTo-Json)),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine)
$vault=Join-Path $private 'install-login.dpapi'
if(Test-Path $vault){
 $credentials=$utf8.GetString([Security.Cryptography.ProtectedData]::Unprotect([IO.File]::ReadAllBytes($vault),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine))|ConvertFrom-Json
 if($credentials.issuer -ne "https://$Domain" -or $credentials.email -ne $AdminEmail){throw 'Credential handoff belongs to another installation'}
 $password=$credentials.password
}else{[IO.File]::WriteAllBytes($vault,$blob)}
[IO.File]::WriteAllText((Join-Path $Data 'setup-defaults.json'),(@{domain=$Domain}|ConvertTo-Json),$utf8)
$process=Start-Process -FilePath $Exe -ArgumentList @('-data',('"'+$Data+'"'),'-port',$Port) -WindowStyle Hidden -PassThru
$linkPath=Join-Path $Data 'secrets/setup-url.dpapi'
for($n=0;$n -lt 120;$n++){if(Test-Path $linkPath){break};if($process.HasExited){throw 'Setup process stopped; inspect local logs'};Start-Sleep -Milliseconds 500}
if(!(Test-Path $linkPath)){throw 'Setup link unavailable'}
$link=[Uri]$utf8.GetString([Security.Cryptography.ProtectedData]::Unprotect([IO.File]::ReadAllBytes($linkPath),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine))
$origin=$link.GetLeftPart([UriPartial]::Authority)
$labels=$Domain.Split('.');$sub=$labels[0];$root=($labels[1..($labels.Count-1)] -join '.')
$request=@{name=$AdminName;email=$AdminEmail;password=$password;confirm=$password;rootDomain=$root;subdomain=$sub;issuer="https://$Domain";mode='manual'}
if($UseExistingCloudflareLogin){
 # Read the existing official login without copying it or modifying its configuration.
 $pem=Get-Content -LiteralPath (Join-Path $env:USERPROFILE '.cloudflared/cert.pem') -Raw
 $payload=($pem -replace '-----BEGIN ARGO TUNNEL TOKEN-----','' -replace '-----END ARGO TUNNEL TOKEN-----','').Trim()
 $connection=$utf8.GetString([Convert]::FromBase64String($payload))|ConvertFrom-Json
 $request.mode='api';$request.apiToken=$connection.apiToken;$request.accountId=$connection.accountID;$request.zoneId=$connection.zoneID;$request.authorize='on'
}
try{
 $null=Invoke-RestMethod "$origin/setup" -Method Post -ContentType 'application/json' -Headers @{Origin=$origin;'X-Setup-Token'=$link.Fragment.Substring(1)} -Body ($request|ConvertTo-Json) -TimeoutSec 150
}catch{throw 'Installation setup failed. Review the wizard error; credentials and API tokens are not printed.'}
finally{$request.Clear();$connection=$null;$pem=$null;$payload=$null;$password=$null;$recovery=$null;$credentials=$null}
for($n=0;$n -lt 120;$n++){try{if((Invoke-RestMethod "$origin/healthz").status -eq 'ok'){Write-Output "Installation ready. Credential handoff: $vault";exit 0}}catch{};if($process.HasExited){throw 'Identity engine stopped; inspect installation logs'};Start-Sleep -Milliseconds 500}
throw 'Identity startup timed out'
