# VidOtter local first-run helper. No network authentication or implicit MCP access.
param([ValidateSet('extension','mcp')][string]$Mode = 'extension')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

if ($Mode -eq 'extension') {
    $folder = Join-Path $root 'extension'
    if (!(Test-Path (Join-Path $folder 'manifest.json'))) {
        throw "VidOtter extension is missing: $folder"
    }
    $guide = Join-Path $root 'docs/extension-install.html'
    if (!(Test-Path $guide)) {
        throw "VidOtter offline extension guide is missing: $guide"
    }
    # The offline guide always opens. No internet or browser store is required.
    # Show the precise folder to select, so users do not need to locate it.
    Start-Process -FilePath explorer.exe -ArgumentList ('"' + $folder + '"')
    $edgePaths = @(
        (Join-Path ${env:ProgramFiles(x86)} 'Microsoft\Edge\Application\msedge.exe'),
        (Join-Path $env:ProgramFiles 'Microsoft\Edge\Application\msedge.exe')
    )
    $edge = $edgePaths | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
    if ($edge) {
        Start-Process -FilePath $edge -ArgumentList 'edge://extensions/'
    } else {
        $chromePaths = @(
            (Join-Path $env:ProgramFiles 'Google\Chrome\Application\chrome.exe'),
            (Join-Path ${env:ProgramFiles(x86)} 'Google\Chrome\Application\chrome.exe')
        )
        $chrome = $chromePaths | Where-Object { $_ -and (Test-Path $_) } | Select-Object -First 1
        if ($chrome) {
            Start-Process -FilePath $chrome -ArgumentList 'chrome://extensions/'
        } else {
            Write-Warning "Edge or Chrome was not found. Open either browser and navigate to its extensions page."
        }
    }
    Start-Process -FilePath $guide
    Write-Host "Required browser activation: enable Developer mode, choose Load unpacked, select $folder"
    exit 0
}

# MCP is in the same installed Go binary. ChatGPT Tunnel is opt-in, and the
# official tunnel-client plus a user-owned Tunnel ID/key are still required.
$setup = Join-Path $PSScriptRoot 'Setup-VidOtter-MCP.ps1'
$client = Get-Command tunnel-client -ErrorAction SilentlyContinue
$localClient = Join-Path $env:USERPROFILE '.local\bin\tunnel-client.exe'
if (!(Test-Path $setup)) {
    throw "VidOtter MCP helper was not packaged: $setup"
}
if ($client -or (Test-Path $localClient)) {
    Write-Host 'Starting the optional VidOtter ChatGPT Tunnel setup.'
    & $setup
} else {
    Write-Host 'The MCP server is already installed. Cloud ChatGPT requires tunnel-client and your own Tunnel credentials.'
    Start-Process 'https://github.com/openai/tunnel-client'
    Start-Process 'https://github.com/ThinkerQAQ/vidotter/blob/main/docs/zh-CN/how-to/mcp.md'
}
