# VidOtter-managed per-user Tunnel runtime controls.
# Explicitly scoped to this profile; never touches other tunnel-client users.
param([Parameter(Mandatory=$true)][ValidateSet('start','stop','restart')][string]$Action)
$ErrorActionPreference = 'Stop'
$private = Join-Path $env:LOCALAPPDATA 'VidOtter\MCP'
$profile = Join-Path $private 'vidotter.yaml'
$runner = Join-Path $private 'run-mcp.ps1'
if (!(Test-Path $profile) -or !(Test-Path $runner)) {
    throw 'VidOtter Tunnel is not configured yet.'
}

function Stop-VidOtterTunnel {
    $procs = Get-CimInstance Win32_Process -Filter "Name='tunnel-client.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.Contains($profile) }
    foreach ($proc in $procs) {
        Invoke-CimMethod -InputObject $proc -MethodName Terminate | Out-Null
    }
}
function Start-VidOtterTunnel {
    $running = @(Get-CimInstance Win32_Process -Filter "Name='tunnel-client.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.Contains($profile) })
    if ($running.Count -gt 0) { return }
    Start-Process -FilePath (Get-Command powershell.exe).Source -ArgumentList @(
        '-NoProfile','-WindowStyle','Hidden','-File',('"' + $runner + '"')
    ) -WindowStyle Hidden
}
if ($Action -in @('stop','restart')) { Stop-VidOtterTunnel }
if ($Action -in @('start','restart')) { Start-VidOtterTunnel }
Write-Host "VidOtter Tunnel action $Action completed; check readiness in VidOtter Settings."
