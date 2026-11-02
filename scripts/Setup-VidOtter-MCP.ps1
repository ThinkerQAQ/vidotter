# Optional first-user MCP configuration; official tunnel-client is required.
param([string]$TunnelId, [switch]$ReplaceExisting, [string]$ExpectedTunnelId)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$binary = Join-Path $root 'bin\vidotter-windows-amd64.exe'
if (-not (Test-Path $binary)) { throw "VidOtter binary missing: $binary" }
$client = Get-Command tunnel-client -ErrorAction SilentlyContinue
if (-not $client) {
    $candidate = Join-Path $env:USERPROFILE '.local\bin\tunnel-client.exe'
    if (Test-Path $candidate) { $client = Get-Item $candidate }
}
if (-not $client) { throw 'Install official openai/tunnel-client first: https://github.com/openai/tunnel-client' }
$clientPath = $client.Source
if (-not $clientPath) { $clientPath = $client.FullName }
if (-not $TunnelId) { $TunnelId = Read-Host 'VidOtter Tunnel ID' }
if ($TunnelId -notmatch '^tunnel_([a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$') { throw 'Invalid Tunnel ID' }
$private = Join-Path $env:LOCALAPPDATA 'VidOtter\MCP'
New-Item -ItemType Directory -Force -Path $private | Out-Null
$keyFile = Join-Path $private 'runtime-key.dpapi'
if (-not (Test-Path $keyFile)) {
    if ($env:CONTROL_PLANE_API_KEY) {
        $secure = ConvertTo-SecureString -String $env:CONTROL_PLANE_API_KEY -AsPlainText -Force
    } else {
        $secure = Read-Host 'Restricted runtime API key (Tunnels Read + Use)' -AsSecureString
    }
    if (-not $secure -or $secure.Length -lt 16) { throw 'Invalid runtime key' }
    $secure | ConvertFrom-SecureString | Set-Content -Encoding UTF8 -Path $keyFile
}
$profile = Join-Path $private 'vidotter.yaml'
$originalId = ''
if (Test-Path $profile) {
    $existing = Get-Content -Raw -Path $profile
    $idMatch = [regex]::Match($existing, '(?m)^\s*tunnel_id:\s*["'']?(tunnel_[a-z0-9_]+)')
    if (-not $idMatch.Success) { throw 'Existing VidOtter profile has no valid Tunnel ID; not overwriting it.' }
    $originalId = $idMatch.Groups[1].Value
    if ($originalId -notmatch '^tunnel_([a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$') {
        throw 'Existing Tunnel ID is invalid; not overwriting it.'
    }
}
$switching = $originalId -and $originalId -ne $TunnelId
if ($switching) {
    if (-not $ReplaceExisting -or $ExpectedTunnelId -ne $originalId) {
        throw 'Changing Tunnel ID requires explicit confirmation using the latest saved ID.'
    }
} elseif ($ReplaceExisting) {
    throw 'There is no different saved Tunnel ID to replace.'
}

function Invoke-TunnelDoctor([string]$Path) {
    $secure = Get-Content -Path $keyFile -TotalCount 1 | ConvertTo-SecureString
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try { $env:CONTROL_PLANE_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
    try {
        & $clientPath doctor --config $Path
        if ($LASTEXITCODE -ne 0) { throw 'Tunnel doctor failed' }
    } finally {
        Remove-Item Env:CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue
    }
}
function Stop-OwnTunnel {
    Get-CimInstance Win32_Process -Filter "name='tunnel-client.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.Contains($profile) } |
        ForEach-Object { Invoke-CimMethod -InputObject $_ -MethodName Terminate | Out-Null }
}
function Start-OwnTunnel([string]$RunnerPath) {
    if (Test-Path $RunnerPath) {
        Start-Process -FilePath (Get-Command powershell.exe).Source -ArgumentList @(
            '-NoProfile', '-WindowStyle', 'Hidden', '-File', ('"' + $RunnerPath + '"')
        ) -WindowStyle Hidden
    }
}
$staging = ''
$rollback = ''
$replaced = $false
$completed = $false
try {
    if ($switching) {
        # Build and validate a separate profile while the old Tunnel is still running.
        $staging = Join-Path $private ('pending-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Force -Path $staging | Out-Null
        $mcpCommand = '"' + $binary.Replace('\','/') + '" --mcp-stdio'
        & $clientPath init --sample sample_mcp_stdio_local --profile-dir $staging --profile vidotter --tunnel-id $TunnelId --mcp-command $mcpCommand --health-listen-addr '127.0.0.1:0'
        if ($LASTEXITCODE -ne 0) { throw 'Replacement Tunnel profile initialization failed' }
        $candidate = Join-Path $staging 'vidotter.yaml'
        if (-not (Test-Path $candidate)) { throw 'Replacement Tunnel profile was not created' }
        Invoke-TunnelDoctor $candidate
        # The backup stays private. Only this VidOtter profile's daemon is stopped.
        Stop-OwnTunnel
        $rollback = Join-Path $private ('rollback-' + [guid]::NewGuid().ToString('N') + '.yaml')
        [System.IO.File]::Replace($candidate, $profile, $rollback)
        $replaced = $true
    } elseif (-not (Test-Path $profile)) {
        $mcpCommand = '"' + $binary.Replace('\','/') + '" --mcp-stdio'
        & $clientPath init --sample sample_mcp_stdio_local --profile-dir $private --profile vidotter --tunnel-id $TunnelId --mcp-command $mcpCommand --health-listen-addr '127.0.0.1:0'
        if ($LASTEXITCODE -ne 0) { throw 'Tunnel profile initialization failed' }
    }
    Invoke-TunnelDoctor $profile
$launcher = Join-Path $private 'run-mcp.ps1'
# Private runner reads the DPAPI-encrypted key at runtime. The key is never in the command line.
$runner = @'
param()
$ErrorActionPreference = 'Stop'
$private = Join-Path $env:LOCALAPPDATA 'VidOtter\MCP'
$keyFile = Join-Path $private 'runtime-key.dpapi'
$secure = Get-Content -Path $keyFile -TotalCount 1 | ConvertTo-SecureString
$bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try { $env:CONTROL_PLANE_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
try { & '__CLIENT__' run --config (Join-Path $private 'vidotter.yaml') --health.url-file (Join-Path $private 'health.url') }
finally { Remove-Item Env:CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue }
'@
$runner = $runner.Replace('__CLIENT__', $clientPath.Replace("'", "''"))
Set-Content -Path $launcher -Value $runner -Encoding UTF8
$startup = [Environment]::GetFolderPath('Startup')
# Isolated smoke tests never touch the user's real Startup folder.
if ($env:VIDOTTER_MCP_TEST_MODE -eq '1' -and $env:VIDOTTER_MCP_TEST_STARTUP_DIR) {
    $startup = $env:VIDOTTER_MCP_TEST_STARTUP_DIR
    New-Item -ItemType Directory -Force -Path $startup | Out-Null
}
$shortcutPath = Join-Path $startup 'VidOtter MCP Tunnel.lnk'
$shell = New-Object -ComObject WScript.Shell
$link = $shell.CreateShortcut($shortcutPath)
$link.TargetPath = (Get-Command powershell.exe).Source
$link.Arguments = '-NoProfile -WindowStyle Hidden -File "' + $launcher + '"'
$link.WorkingDirectory = $private
$link.Save()
$already = Get-CimInstance Win32_Process -Filter "name='tunnel-client.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -and $_.CommandLine.Contains($profile) }
if (-not $already) {
    Start-OwnTunnel $launcher
}
if ($switching) {
    # Health must confirm a control-plane poll, not merely a saved profile.
    $healthFile = Join-Path $private 'health.url'
    $healthy = $false
    $maxAttempts = 35
    if ($env:VIDOTTER_MCP_HEALTH_ATTEMPTS -match '^[1-9][0-9]?$') {
        $maxAttempts = [int]$env:VIDOTTER_MCP_HEALTH_ATTEMPTS
    }
    for ($attempt = 0; $attempt -lt $maxAttempts; $attempt++) {
        Start-Sleep -Seconds 1
        & $clientPath health --url-file $healthFile --require-control-plane-poll --json 1>$null 2>$null
        if ($LASTEXITCODE -eq 0) { $healthy = $true; break }
    }
    if (-not $healthy) { throw 'New Tunnel did not become ready; restoring previous profile.' }
}
$completed = $true
Write-Host 'VidOtter Tunnel configured. In ChatGPT: Plugins -> Add custom MCP server -> Tunnel.'
} catch {
    if ($replaced -and (Test-Path $rollback)) {
        Stop-OwnTunnel
        Copy-Item -LiteralPath $rollback -Destination $profile -Force
        Start-OwnTunnel (Join-Path $private 'run-mcp.ps1')
        Write-Warning 'Previous VidOtter Tunnel profile was restored.'
    }
    throw
} finally {
    if ($staging -and (Test-Path $staging)) { Remove-Item -LiteralPath $staging -Recurse -Force }
    if ($completed -and $rollback -and (Test-Path $rollback)) { Remove-Item -LiteralPath $rollback -Force }
}
# A successful setup closes the configuration terminal normally.
exit 0
