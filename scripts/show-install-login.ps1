param([Parameter(Mandatory=$true)][string]$Path)
$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Security.Cryptography.ProtectedData
$plain=[Security.Cryptography.ProtectedData]::Unprotect([IO.File]::ReadAllBytes((Resolve-Path -LiteralPath $Path).Path),$null,[Security.Cryptography.DataProtectionScope]::LocalMachine)
$login=[Text.Encoding]::UTF8.GetString($plain)|ConvertFrom-Json
Write-Host 'Installation credentials — change these passwords after first sign-in.'
$login|Format-List
