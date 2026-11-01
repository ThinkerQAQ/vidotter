# Optional first-user MCP configuration; official tunnel-client is required.
param([string]$TunnelId)
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
if (Test-Path $profile) {
    $existing = Get-Content -Raw -Path $profile
    if ($existing -notmatch ([regex]::Escape($TunnelId))) {
        throw 'Tunnel ID differs from existing VidOtter profile. Use its original ID or update the profile explicitly.'
    }
}
if (-not (Test-Path $profile)) {
    $mcpCommand = '"' + $binary + '" --mcp-stdio'
    & $clientPath init --sample sample_mcp_stdio_local --profile-dir $private --profile vidotter --tunnel-id $TunnelId --mcp-command $mcpCommand --health-listen-addr '127.0.0.1:0'
    if ($LASTEXITCODE -ne 0) { throw 'Tunnel profile initialization failed' }
}
$secure = Get-Content -Path $keyFile -TotalCount 1 | ConvertTo-SecureString
$bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try { $env:CONTROL_PLANE_API_KEY = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr) }
finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
try {
    & $clientPath doctor --config $profile
    if ($LASTEXITCODE -ne 0) { throw 'Tunnel doctor failed' }
} finally {
    Remove-Item Env:CONTROL_PLANE_API_KEY -ErrorAction SilentlyContinue
}
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
    Start-Process -FilePath (Get-Command powershell.exe).Source -ArgumentList @('-NoProfile','-WindowStyle','Hidden','-File',('"' + $launcher + '"')) -WindowStyle Hidden
}
Write-Host 'VidOtter Tunnel configured. In ChatGPT: Plugins -> Add custom MCP server -> Tunnel.'
