$ErrorActionPreference = "Stop"
$setup = Join-Path $env:RUNNER_TEMP ("installer-output/VidOtter-Setup-" + $env:RELEASE_TAG + "-windows-amd64.exe")
$sandbox = Join-Path $env:RUNNER_TEMP "vidotter-windows-qa"
$target = Join-Path $sandbox "install with spaces"
New-Item -ItemType Directory -Force -Path $sandbox | Out-Null
& $setup --root $target --accept-licenses --confirm-command install
if ($LASTEXITCODE -ne 0) { throw "Windows installer failed" }
if (!(Test-Path (Join-Path $target "bin/vidotter-windows-amd64.exe"))) { throw "Bridge not installed" }
$key = "HKCU:\Software\Google\Chrome\NativeMessagingHosts\com.vidotter.bridge"
if (!(Test-Path $key)) { throw "Chrome Native Messaging host missing" }
$data = Join-Path $target "Data"
New-Item -ItemType Directory -Force -Path (Join-Path $data "downloads") | Out-Null
Set-Content -Path (Join-Path $data "config.toml") -Value 'sentinel=true'
Set-Content -Path (Join-Path $data "downloads/keep.txt") -Value 'kept'
if ($env:VIDOTTER_TEST_UPDATE_REPO) {
    $repoUri = ([System.Uri]::new((Resolve-Path $env:VIDOTTER_TEST_UPDATE_REPO).Path + [System.IO.Path]::DirectorySeparatorChar)).AbsoluteUri
    & (Join-Path $target "VidOtter-Maintenance.exe") --at $repoUri check-updates
    if ($LASTEXITCODE -ne 0) { throw "Update check failed" }
    & (Join-Path $target "VidOtter-Maintenance.exe") --at $repoUri --confirm-command update
    if ($LASTEXITCODE -ne 0) { throw "Update failed" }
    if (-not ((Get-Content -Raw (Join-Path $target "components.xml")) -match "<Version>$($env:VIDOTTER_TEST_UPDATE_VERSION)</Version>")) { throw "Installed component version did not update" }
    if (!(Test-Path (Join-Path $data "config.toml"))) { throw "Config lost on update" }
    if (!(Test-Path $key)) { throw "MCP Native Messaging registration lost on update" }
}
& (Join-Path $target "VidOtter-Maintenance.exe") --confirm-command remove com.thinkerqaq.vidotter
if ($LASTEXITCODE -ne 0) { throw "Windows uninstall failed" }
if (!(Test-Path (Join-Path $data "config.toml"))) { throw "Config lost on uninstall" }
if (!(Test-Path (Join-Path $data "downloads/keep.txt"))) { throw "Downloads lost on uninstall" }
if (Test-Path $key) { throw "Native Messaging registry not removed" }
Write-Host "Windows installer smoke passed"
