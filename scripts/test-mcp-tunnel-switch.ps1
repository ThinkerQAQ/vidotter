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
$oldMode = $env:VIDOTTER_MCP_TEST_MODE
$oldStartupDir = $env:VIDOTTER_MCP_TEST_STARTUP_DIR
$startup = Join-Path $work 'startup/VidOtter MCP Tunnel.lnk'
$env:VIDOTTER_MCP_TEST_MODE = '1'
$env:VIDOTTER_MCP_TEST_STARTUP_DIR = Split-Path -Parent $startup
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
    function Setup([string]$id, [bool]$replace=$false, [string]$expected='', [string]$inputKey='') {
        $cli = @('-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',$setup,'-TunnelId',$id)
        if ($replace) { $cli += @('-ReplaceExisting','-ExpectedTunnelId',$expected) }
        try {
            if ($inputKey) { $cli += '-KeyFromStdin'; $inputKey | & powershell.exe @cli 1>$null 2>$null }
            else { & powershell.exe @cli 1>$null 2>$null }
        }
        catch { return 1 } # Expected failure cases write to stderr.
        return $LASTEXITCODE
    }
    if ((Setup $original) -ne 0) { throw 'Initial setup failed' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($original)) { throw 'Initial ID not saved' }
    if (!(Test-Path (Join-Path $env:LOCALAPPDATA 'VidOtter/MCP/runtime-key.dpapi'))) { throw 'DPAPI key missing' }
    if ((Setup $other) -eq 0) { throw 'Silent ID replacement must be rejected' }
    if ((Setup $other $true $bad) -eq 0) { throw 'Stale original ID must be rejected' }
    if ((Setup $other $true $original) -ne 0) { throw 'Explicit replacement failed' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($other)) { throw 'Replacement ID not saved' }
    $env:CONTROL_PLANE_API_KEY = 'FAKE_ROTATED_RUNTIME_KEY_FOR_TEST_456'
    if ((Setup $other $true $other) -ne 0) { throw 'Same-ID key rotation failed' }
    $keyPath = Join-Path $env:LOCALAPPDATA 'VidOtter/MCP/runtime-key.dpapi'
    $saved = Get-Content $keyPath | ConvertTo-SecureString
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($saved)
    try {
        if ([Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) -cne $env:CONTROL_PLANE_API_KEY) { throw 'Rotated key not saved' }
    } finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
    $expectedKey = [IO.File]::ReadAllText($keyPath)
    $revisionPath = Join-Path $env:LOCALAPPDATA 'VidOtter/MCP/configuration-revision'
    $expectedRevision = Get-Content -Raw $revisionPath
    $env:CONTROL_PLANE_API_KEY = 'FAKE_FAILED_CANDIDATE_KEY_FOR_TEST_789'
    # Page submission uses stdin, overrides inherited keys and never opens a window.
    $inlineKey = 'FAKE_INLINE_KEY_FROM_PASSWORD_FORM_123'
    if ((Setup $other $true $other $inlineKey) -ne 0) { throw 'Stdin key update failed' }
    $saved=Get-Content $keyPath | ConvertTo-SecureString
    $ptr=[Runtime.InteropServices.Marshal]::SecureStringToBSTR($saved)
    try {
        if([Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) -cne $inlineKey){throw 'Wrong stdin key saved'}
    } finally {[Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr)}
    $expectedKey=[IO.File]::ReadAllText($keyPath)
    $expectedRevision=Get-Content -Raw $revisionPath
    $env:VIDOTTER_FAKE_BAD_ID = $bad
    if ((Setup $bad $true $other) -eq 0) { throw 'Invalid candidate doctor must fail' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($other)) { throw 'Doctor failure modified saved ID' }
    Remove-Item Env:VIDOTTER_FAKE_BAD_ID -ErrorAction SilentlyContinue
    $env:VIDOTTER_FAKE_FAIL_HEALTH = '1'
    if ((Setup $original $true $other) -eq 0) { throw 'Failed control-plane health must roll back' }
    if ((Get-Content -Raw $profile) -notmatch [regex]::Escape($other)) { throw 'Health failure did not restore original ID' }
    if ([IO.File]::ReadAllText($keyPath) -cne $expectedKey) { throw 'Failed update did not restore the original key' }
    if ((Get-Content -Raw $revisionPath) -cne $expectedRevision) { throw 'Failed update marked successful' }
    Write-Host 'Windows Tunnel switch smoke: initial setup, explicit switch, stale ID, doctor failure, rollback all passed.'
} finally {
    $env:LOCALAPPDATA = $oldLocal
    $env:PATH = $oldPath
    $env:CONTROL_PLANE_API_KEY = $oldKey
    $env:VIDOTTER_FAKE_BAD_ID = $oldBad
    $env:VIDOTTER_FAKE_FAIL_HEALTH = $oldHealth
    $env:VIDOTTER_MCP_HEALTH_ATTEMPTS = $oldAttempts
    $env:VIDOTTER_MCP_TEST_MODE = $oldMode
    $env:VIDOTTER_MCP_TEST_STARTUP_DIR = $oldStartupDir
    if (Test-Path $work) { Remove-Item $work -Recurse -Force -ErrorAction SilentlyContinue }
}

# Expected negative smoke cases leave LASTEXITCODE=1; report the passing test explicitly.
exit 0
