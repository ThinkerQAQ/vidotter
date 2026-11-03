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

# MCP settings are managed in VidOtter, using the native Go service manager.
Write-Host 'Open VidOtter > Settings > External components > ChatGPT MCP to save your connection and control the service.'
$guide = Join-Path $root 'README_ZH.md'
if (!(Test-Path $guide)) { throw 'VidOtter offline guide is missing.' }
Start-Process -FilePath $guide
