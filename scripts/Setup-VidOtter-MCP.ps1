# Optional ChatGPT Tunnel setup. Credentials are entered only on this computer.
param([string]$TunnelId, [switch]$ReplaceExisting, [string]$ExpectedTunnelId, [switch]$KeyFromStdin, [switch]$SaveOnly, [switch]$ReuseSavedKey)
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$binary = Join-Path $root 'bin\vidotter-windows-amd64.exe'
if (!(Test-Path $binary)) { throw 'Installed VidOtter binary is missing.' }
$client = Get-Command tunnel-client -ErrorAction SilentlyContinue
if (!$client) { $client = Get-Item (Join-Path $env:USERPROFILE '.local\bin\tunnel-client.exe') -ErrorAction SilentlyContinue }
if (!$client) { throw 'Install the official OpenAI Tunnel Client first.' }
$clientPath = $client.Source
if (!$clientPath) { $clientPath = $client.FullName }
Write-Host 'Tunnel ID: https://platform.openai.com/settings/organization/tunnels'
Write-Host 'Runtime API Key: https://platform.openai.com/api-keys'
Write-Host 'Select the intended organization and create a restricted key with Tunnels Read + Use.'
if (!$TunnelId) { $TunnelId = Read-Host 'VidOtter Tunnel ID' }
if ($TunnelId -notmatch '^tunnel_([a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$') { throw 'Invalid Tunnel ID' }
$private = Join-Path $env:LOCALAPPDATA 'VidOtter\MCP'
New-Item -ItemType Directory -Force -Path $private | Out-Null
$profile = Join-Path $private 'vidotter.yaml'
$keyFile = Join-Path $private 'runtime-key.dpapi'
$launcher = Join-Path $private 'run-mcp.ps1'
$originalId = ''
if (Test-Path $profile) {
    $match = [regex]::Match((Get-Content -Raw $profile), '(?m)^\s*tunnel_id:\s*["'']?(tunnel_[a-z0-9_]+)')
    if (!$match.Success) { throw 'Existing Tunnel profile is invalid; refusing to overwrite it.' }
    $originalId = $match.Groups[1].Value
    if (!$ReplaceExisting -or $ExpectedTunnelId -ne $originalId) {
        throw 'Changing connection credentials requires confirmation using the latest saved Tunnel ID.'
    }
} elseif ($ReplaceExisting) { throw 'No existing connection to replace.' }
# Never silently import another application's inherited API key.
if ($KeyFromStdin) {
    $value = [Console]::In.ReadLine()
    if ([string]::IsNullOrWhiteSpace($value) -and $ReuseSavedKey -and (Test-Path $keyFile)) {
        $secure = Get-Content -Path $keyFile -TotalCount 1 | ConvertTo-SecureString
    } else {
        if ([string]::IsNullOrWhiteSpace($value)) { throw 'Runtime API Key is required.' }
        $secure = ConvertTo-SecureString -String $value -AsPlainText -Force
    }
    $value = $null
} elseif ($env:VIDOTTER_MCP_TEST_MODE -eq '1' -and $env:CONTROL_PLANE_API_KEY) {
    $secure = ConvertTo-SecureString -String $env:CONTROL_PLANE_API_KEY -AsPlainText -Force
} else {
    Write-Host 'Enter the Runtime API Key for this connection. The saved key is not reused automatically.'
    $secure = Read-Host 'Runtime API Key (hidden input)' -AsSecureString
}
if (!$secure -or $secure.Length -lt 16) { throw 'Runtime API Key is required.' }
function Invoke-TunnelClient {
    $clientArgs = @($args)
    if ($KeyFromStdin) {
        $quoted = @($clientArgs | ForEach-Object { '"' + $_.Replace('"','\"') + '"' })
        $process = Start-Process -FilePath $clientPath -ArgumentList $quoted -WindowStyle Hidden -Wait -PassThru
        $global:LASTEXITCODE = $process.ExitCode
    } else {
        & $clientPath @clientArgs
        $global:LASTEXITCODE = $LASTEXITCODE
    }
}
function Stop-OwnTunnel {
    Get-CimInstance Win32_Process -Filter "Name='tunnel-client.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.Contains($profile) } |
        ForEach-Object { Invoke-CimMethod -InputObject $_ -MethodName Terminate -ErrorAction SilentlyContinue | Out-Null }
}
function Start-OwnTunnel {
    if (Test-Path $launcher) {
        Start-Process -FilePath (Get-Command powershell.exe).Source -ArgumentList @(
            '-NoProfile','-WindowStyle','Hidden','-File',('"'+$launcher+'"')
        ) -WindowStyle Hidden
    }
}
$staging = Join-Path $private ('pending-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $staging | Out-Null
$hadProfile = Test-Path $profile
$hadKey = Test-Path $keyFile
$committing = $false
$completed = $false
try {
    if ($hadProfile) { Copy-Item $profile (Join-Path $staging 'old.yaml') }
    if ($hadKey) { Copy-Item $keyFile (Join-Path $staging 'old.dpapi') }
    $candidateKey = Join-Path $staging 'runtime-key.dpapi'
    $secure | ConvertFrom-SecureString | Set-Content -Encoding UTF8 -Path $candidateKey
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    try { $env:CONTROL_PLANE_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr); $secure.Dispose() }
    $mcpCommand = '"' + $binary.Replace('\','/') + '" --mcp-stdio'
    Invoke-TunnelClient init --sample sample_mcp_stdio_local --profile-dir $staging --profile vidotter --tunnel-id $TunnelId --mcp-command $mcpCommand --health-listen-addr '127.0.0.1:0'
    if ($LASTEXITCODE -ne 0) { throw 'Tunnel profile initialization failed.' }
    $candidate = Join-Path $staging 'vidotter.yaml'
    Invoke-TunnelClient doctor --config $candidate
    if ($LASTEXITCODE -ne 0) { throw 'Candidate Tunnel configuration check failed.' }
    # Commit ID and key together; retain both backups until live polling succeeds.
    if (!$SaveOnly) { Stop-OwnTunnel }
    $committing = $true
    Copy-Item $candidate $profile -Force
    Copy-Item $candidateKey $keyFile -Force
$runner = @'
param()
$ErrorActionPreference = 'Stop'
$private = Join-Path $env:LOCALAPPDATA 'VidOtter\MCP'
$keyFile = Join-Path $private 'runtime-key.dpapi'
$secure = Get-Content -Path $keyFile -TotalCount 1 | ConvertTo-SecureString
$bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try { $env:CONTROL_PLANE_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
$profile = Join-Path $private 'vidotter.yaml'
$existing = @(Get-CimInstance Win32_Process -Filter "Name='tunnel-client.exe'" -ErrorAction SilentlyContinue |
    Where-Object { $_.CommandLine -and $_.CommandLine.Contains($profile) })
if ($existing.Count -gt 0) { exit 0 }
try { & '__CLIENT__' run --config (Join-Path $private 'vidotter.yaml') --health.url-file (Join-Path $private 'health.url') }
finally { Remove-Item Env:CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue }
exit $LASTEXITCODE
'@
$runner = $runner.Replace('__CLIENT__', $clientPath.Replace("'", "''"))
Set-Content -Path $launcher -Value $runner -Encoding UTF8

    if ($SaveOnly) {
        Set-Content -Path (Join-Path $private 'configuration-revision') -Value ([guid]::NewGuid().ToString('N')) -Encoding ASCII
        Set-Content -Path (Join-Path $private 'restart-required') -Value 'true' -Encoding ASCII
        $completed=$true
        Write-Host 'MCP configuration saved. Service state and login tasks unchanged.'
        exit 0
    }
    Remove-Item (Join-Path $private 'health.url') -ErrorAction SilentlyContinue
    Start-OwnTunnel
    $healthy=$false
    $attempts=35
    if ($env:VIDOTTER_MCP_HEALTH_ATTEMPTS -match '^[1-9][0-9]?$') { $attempts=[int]$env:VIDOTTER_MCP_HEALTH_ATTEMPTS }
    for ($attempt=0;$attempt -lt $attempts;$attempt++) {
        Start-Sleep -Seconds 1
        Invoke-TunnelClient health --url-file (Join-Path $private 'health.url') --require-control-plane-poll --json 1>$null 2>$null
        if ($LASTEXITCODE -eq 0) { $healthy=$true;break }
    }
    if (!$healthy) { throw 'New connection failed its live polling check.' }
    Set-Content -Path (Join-Path $private 'configuration-revision') -Value ([guid]::NewGuid().ToString('N')) -Encoding ASCII
    Remove-Item (Join-Path $private 'restart-required') -ErrorAction SilentlyContinue
    $completed=$true
    Write-Host 'Tunnel ID and Runtime API Key saved and verified. Continue in ChatGPT Plugins.'
} catch {
    if ($committing) {
        if (!$SaveOnly) { Stop-OwnTunnel }
        if ($hadProfile) { Copy-Item (Join-Path $staging 'old.yaml') $profile -Force }
        else { Remove-Item $profile -ErrorAction SilentlyContinue }
        if ($hadKey) { Copy-Item (Join-Path $staging 'old.dpapi') $keyFile -Force }
        else { Remove-Item $keyFile -ErrorAction SilentlyContinue }
        if (!$SaveOnly -and $hadProfile -and $hadKey) { Start-OwnTunnel }
        Write-Warning 'The previous Tunnel ID and Runtime API Key were restored.'
    }
    throw
} finally {
    Remove-Item Env:CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue
    Remove-Item $staging -Recurse -Force
}
exit 0
