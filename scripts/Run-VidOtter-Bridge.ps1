$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
& (Join-Path $root 'bin\vidotter-windows-amd64.exe') --bridge-service
exit $LASTEXITCODE
