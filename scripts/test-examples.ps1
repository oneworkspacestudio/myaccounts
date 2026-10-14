param([string]$AdminVault='F:/MyAccounts/install/private/install-login.dpapi',[string]$Python='C:/Users/fahim/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe',[string]$PythonPackages='H:/Projects/tooling/myaccounts/python-rp',[string]$Playwright='I:/Projects/filebrowser-drive/frontend/node_modules/@playwright/test/index.mjs')
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Security.Cryptography.ProtectedData
$utf8=New-Object Text.UTF8Encoding($false)
$private=Split-Path $AdminVault -Parent
$credentials=$utf8.GetString([Security.Cryptography.ProtectedData]::Unprotect([IO.File]::ReadAllBytes($AdminVault),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine))|ConvertFrom-Json
$children=@();$temporary=@()
try{
 $env:MYACCOUNTS_ADMIN_API='http://localhost:18888';$env:MYACCOUNTS_ISSUER=$credentials.issuer;$env:MYACCOUNTS_ADMIN_EMAIL=$credentials.email;$env:MYACCOUNTS_ADMIN_PASSWORD=$credentials.password
 $env:MYACCOUNTS_REDIRECT_PATH='/callback';$env:OIDC_ISSUER=$credentials.issuer
 foreach($kind in @('node','python')){
  $port=if($kind -eq 'node'){18994}else{18995};$handoff=Join-Path $private ('example-'+$kind+'-'+[Guid]::NewGuid()+'.json');$temporary+=$handoff
  $env:FILEBROWSER_ORIGIN="http://localhost:$port";$env:FILEBROWSER_CLIENT_NAME='example-'+$kind+'-'+[Guid]::NewGuid().ToString('N').Substring(0,8);$env:MYACCOUNTS_CLIENT_OUTPUT=$handoff;$env:MYACCOUNTS_CLIENT_DISPLAY=$kind+' OIDC example'
  node (Join-Path $PSScriptRoot '../integration/register-filebrowser.mjs');if($LASTEXITCODE){throw 'Example client registration failed'}
  $client=Get-Content $handoff -Raw|ConvertFrom-Json;$env:OIDC_CLIENT_ID=$client.clientId;$env:OIDC_CLIENT_SECRET=$client.clientSecret;$env:RP_ORIGIN="http://localhost:$port"
  if($kind -eq 'node'){$program=(Get-Command node).Source;$script=Join-Path $PSScriptRoot '../examples/node/server.mjs'}else{$program=$Python;$script=Join-Path $PSScriptRoot '../examples/python/app.py';$env:PYTHONPATH=$PythonPackages}
  $process=Start-Process -FilePath $program -ArgumentList ('"'+$script+'"') -WindowStyle Hidden -RedirectStandardOutput (Join-Path $private ('example-'+$kind+'-stdout.log')) -RedirectStandardError (Join-Path $private ('example-'+$kind+'-stderr.log')) -PassThru;$children+=$process
  $ready=$false;for($n=0;$n -lt 60;$n++){if($process.HasExited){throw 'Example relying party exited; inspect protected log'};try{if((Invoke-WebRequest "http://localhost:$port").StatusCode -eq 200){$ready=$true;break}}catch{};Start-Sleep -Milliseconds 500};if(!$ready){throw 'Example startup timed out'}
 }
 $env:MYACCOUNTS_PLAYWRIGHT=$Playwright;$env:MYACCOUNTS_BROWSER_DATA=Join-Path $private ('example-browser-'+[Guid]::NewGuid())
 node (Join-Path $PSScriptRoot '../integration/example-browser.mjs');if($LASTEXITCODE){throw 'Example browser acceptance failed'}
}finally{
 foreach($child in $children){if(!$child.HasExited){Stop-Process -Id $child.Id -ErrorAction SilentlyContinue}}
 foreach($path in $temporary){Remove-Item -LiteralPath $path -ErrorAction SilentlyContinue}
 foreach($name in @('MYACCOUNTS_ADMIN_EMAIL','MYACCOUNTS_ADMIN_PASSWORD','OIDC_CLIENT_SECRET','OIDC_CLIENT_ID','OIDC_ISSUER','RP_ORIGIN','MYACCOUNTS_REDIRECT_PATH','MYACCOUNTS_CLIENT_DISPLAY','MYACCOUNTS_CLIENT_OUTPUT','FILEBROWSER_CLIENT_NAME','FILEBROWSER_ORIGIN','PYTHONPATH')){Remove-Item ('Env:'+ $name) -ErrorAction SilentlyContinue}
 $credentials=$null;$client=$null
}
