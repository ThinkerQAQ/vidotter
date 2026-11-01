# VidOtter local first-run helper. No network authentication or implicit MCP access.
param([ValidateSet('extension','mcp')][string]$Mode = 'extension')
$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot

if ($Mode -eq 'extension') {
    $folder = Join-Path $root 'extension'
    if (!(Test-Path (Join-Path $folder 'manifest.json'))) {
        throw "VidOtter extension is missing: $folder"
    }
    # Show the precise folder to select. Do not rely on the user finding it.
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
            Start-Process 'https://github.com/ThinkerQAQ/vidotter/blob/main/docs/zh-CN/tutorial/first-run-on-new-machine.md'
        }
    }
    Write-Host "Activate Developer mode, choose Load unpacked and select: $folder"
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
