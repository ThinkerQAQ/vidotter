# VidOtter local first-run helper. No network authentication or implicit MCP access.
param([ValidateSet('extension','mcp')][string]$Mode = 'extension')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

if ($Mode -eq 'extension') {
    $folder = Join-Path $root 'extension'
    if (!(Test-Path (Join-Path $folder 'manifest.json'))) {
        throw "VidOtter extension is missing: $folder"
    }
    $guide = Join-Path $root 'README_ZH.md'
    if (!(Test-Path $guide)) {
        throw "VidOtter offline extension guide is missing: $guide"
    }
    # Open only the requested local README; do not open Explorer or browser settings.
    Start-Process -FilePath $guide
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
    Start-Process 'https://github.com/ThinkerQAQ/vidotter/blob/main/README_ZH.md'
}
