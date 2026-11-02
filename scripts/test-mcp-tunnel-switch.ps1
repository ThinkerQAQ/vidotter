# Isolated Windows tunnel setup regression. Uses a fake client and fake runtime key.
# No actual Tunnel, network calls, or user's live profile are touched.
$ErrorActionPreference = 'Stop'
$work = Join-Path $env:TEMP ('vidotter-switch-' + [guid]::NewGuid().ToString('N'))
$oldLocal = $env:LOCALAPPDATA
$oldPath = $env:PATH
$oldKey = $env:CONTROL_PLANE_API_KEY
$oldBad = $env:VIDOTTER_FAKE_BAD_ID
$oldHealth = $env:VIDOTTER_FAKE_FAIL_HEALTH
$oldAttempts = $env:VIDOTTER_MCP_HEALTH_ATTEMPTS
$startup = Join-Path ([Environment]::GetFolderPath('Startup')) 'VidOtter MCP Tunnel.lnk'
$existingShortcut = Test-Path $startup
if ($existingShortcut) { throw 'Refusing to replace an existing user startup shortcut in smoke tests' }
try {
    foreach ($dir in @('app/scripts','app/bin','mock','local')) {
        New-Item -ItemType Directory -Force -Path (Join-Path $work $dir) | Out-Null
    }
    $setup = Join-Path $work 'app/scripts/Setup-VidOtter-MCP.ps1'
    Copy-Item (Join-Path $PSScriptRoot 'Setup-VidOtter-MCP.ps1') $setup
    Set-Content -Path (Join-Path $work 'app/bin/vidotter-windows-amd64.exe') -Value 'fake'
    $clientScript = Join-Path $work 'mock/tunnel-mock.ps1'
    @'
$allArgs = $args
$command = $allArgs[0]
function ValueAfter([string]$name) {
    $index = [Array]::IndexOf($allArgs, $name)
    if ($index -lt 0 -or $index + 1 -ge $allArgs.Count) { exit 8 }
    return $allArgs[$index + 1]
}
if ($command -eq 'init') {
    $dir = ValueAfter '--profile-dir'
    $id = ValueAfter '--tunnel-id'
    $name = ValueAfter '--profile'
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    Set-Content -Path (Join-Path $dir ($name + '.yaml')) -Value ('tunnel_id: ' + $id) -Encoding UTF8
    exit 0
}
if ($command -eq 'doctor') {
    $path = ValueAfter '--config'
    $data = Get-Content -Raw $path
    if (!$env:CONTROL_PLANE_API_KEY) { exit 7 }
    if ($env:VIDOTTER_FAKE_BAD_ID -and $data.Contains($env:VIDOTTER_FAKE_BAD_ID)) { exit 7 }
    exit 0
}
if ($command -eq 'health') {
    if ($env:VIDOTTER_FAKE_FAIL_HEALTH -eq '1') { exit 9 }
    exit 0
}
if ($command -eq 'run') { exit 0 }
exit 3
'@ | Set-Content -Encoding UTF8 -Path $clientScript
    @'
@echo off
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0tunnel-mock.ps1" %*
exit /b %ERRORLEVEL%
'@ | Set-Content -Encoding Ascii -Path (Join-Path $work 'mock/tunnel-client.cmd')
    $env:LOCALAPPDATA = Join-Path $work 'local'
    $env:PATH = (Join-Path $work 'mock') + ';' + $oldPath
    $env:CONTROL_PLANE_API_KEY = 'FAKE_RUNTIME_KEY_FOR_UNIT_TEST_123'
    $env:VIDOTTER_MCP_HEALTH_ATTEMPTS = '1'
    $original = 'tunnel_0123456789abcdef0123456789abcdef'
    $other = 'tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
    $bad = 'tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'
    $profile = Join-Path $env:LOCALAPPDATA 'VidOtter/MCP/vidotter.yaml'
    function Setup([string]$id, [bool]$replace=$false, [string]$expected='') {
        $cli = @('-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',$setup,'-TunnelId',$id)
        if ($replace) { $cli += @('-ReplaceExisting','-ExpectedTunnelId',$expected) }
        & powershell.exe @cli 1>$null 2>$null
        return $LASTEXITCODE
    }
    if ((Setup $original) -ne 0) { throw 'Initial setup failed' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($original)) { throw 'Initial ID not saved' }
    if (!(Test-Path (Join-Path $env:LOCALAPPDATA 'VidOtter/MCP/runtime-key.dpapi'))) { throw 'DPAPI key missing' }
    if ((Setup $other) -eq 0) { throw 'Silent ID replacement must be rejected' }
    if ((Setup $other $true $bad) -eq 0) { throw 'Stale original ID must be rejected' }
    if ((Setup $other $true $original) -ne 0) { throw 'Explicit replacement failed' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($other)) { throw 'Replacement ID not saved' }
    $env:VIDOTTER_FAKE_BAD_ID = $bad
    if ((Setup $bad $true $other) -eq 0) { throw 'Invalid candidate doctor must fail' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($other)) { throw 'Doctor failure modified saved ID' }
    Remove-Item Env:VIDOTTER_FAKE_BAD_ID -ErrorAction SilentlyContinue
    $env:VIDOTTER_FAKE_FAIL_HEALTH = '1'
    if ((Setup $original $true $other) -eq 0) { throw 'Failed control-plane health must roll back' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($other)) { throw 'Health failure did not restore original ID' }
    Write-Host 'Windows Tunnel switch smoke: initial setup, explicit switch, stale ID, doctor failure, rollback all passed.'
} finally {
    $env:LOCALAPPDATA = $oldLocal
    $env:PATH = $oldPath
    $env:CONTROL_PLANE_API_KEY = $oldKey
    $env:VIDOTTER_FAKE_BAD_ID = $oldBad
    $env:VIDOTTER_FAKE_FAIL_HEALTH = $oldHealth
    $env:VIDOTTER_MCP_HEALTH_ATTEMPTS = $oldAttempts
    if (-not $existingShortcut -and (Test-Path $startup)) { Remove-Item $startup -Force }
    if (Test-Path $work) { Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue }
}
