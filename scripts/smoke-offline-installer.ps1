$ErrorActionPreference = "Stop"
$setup = Join-Path $env:RUNNER_TEMP ("installer-output/VidOtter-Setup-" + $env:RELEASE_TAG + "-windows-amd64.exe")
$sandbox = Join-Path $env:RUNNER_TEMP "vidotter-windows-qa"
$target = Join-Path $sandbox "install with spaces"
New-Item -ItemType Directory -Force -Path $sandbox | Out-Null
& $setup --root $target --accept-licenses --confirm-command install
if ($LASTEXITCODE -ne 0) { throw "Windows installer failed" }
if (!(Test-Path (Join-Path $target "bin/vidotter-windows-amd64.exe"))) { throw "Bridge not installed" }
$offlineGuide = Join-Path $target "docs/extension-install.html"
if (!(Test-Path $offlineGuide)) { throw "Offline browser extension guide missing from installer" }
$guideText = Get-Content -Raw -Encoding UTF8 $offlineGuide
if ($guideText -notmatch "edge://extensions" -or $guideText -notmatch "chrome://extensions" -or $guideText -notmatch "manifest.json") {
    throw "Offline extension guide is missing required browser steps"
}
$manage = Join-Path $target "scripts/Manage-VidOtter-MCP.ps1"
if (!(Test-Path $manage)) { throw "Tunnel service manager missing from installed Windows release" }
$syntaxTokens = $null
$syntaxErrors = $null
[System.Management.Automation.Language.Parser]::ParseFile($manage, [ref]$syntaxTokens, [ref]$syntaxErrors) | Out-Null
if ($syntaxErrors.Count -gt 0) { throw "Tunnel service manager contains invalid PowerShell syntax: $syntaxErrors" }
$firstRun = Join-Path $target "scripts/VidOtter-FirstRun.ps1"
if (!(Test-Path $firstRun)) { throw "First-run helper missing from installation" }
$parseErrors = $null
[System.Management.Automation.Language.Parser]::ParseFile($firstRun, [ref]$null, [ref]$parseErrors) | Out-Null
if ($parseErrors.Count -gt 0) { throw "Windows first-run PowerShell script has syntax errors: $parseErrors" }
$helperText = Get-Content -Raw -Encoding UTF8 $firstRun
if ($helperText -notmatch 'Start-Process -FilePath \$guide' -or $helperText -notmatch 'edge://extensions/') {
    throw "First-run helper does not open offline guide and extension manager"
}

$startMenu = Join-Path ([Environment]::GetFolderPath('Programs')) 'VidOtter'
foreach ($shortcut in @('Install Browser Extension.lnk', 'Uninstall VidOtter.lnk')) {
    if (!(Test-Path (Join-Path $startMenu $shortcut))) {
        throw "Missing Start Menu setup shortcut: $shortcut"
    }
}

# Check uninstall shortcut opens the maintenance executable in this install.
$uninstallLink = Join-Path $startMenu 'Uninstall VidOtter.lnk'
$wsh = New-Object -ComObject WScript.Shell
$linkTarget = $wsh.CreateShortcut($uninstallLink).TargetPath
$maintenance = Join-Path $target 'VidOtter-Maintenance.exe'
if (![string]::Equals($linkTarget, $maintenance, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Unexpected uninstall shortcut target: $linkTarget"
}
$uninstallRoots = @(
    'HKCU:/Software/Microsoft/Windows/CurrentVersion/Uninstall',
    'HKLM:/Software/Microsoft/Windows/CurrentVersion/Uninstall',
    'HKLM:/Software/WOW6432Node/Microsoft/Windows/CurrentVersion/Uninstall'
)
$registered = @($uninstallRoots |
    ForEach-Object { Get-ChildItem -Path $_ -ErrorAction SilentlyContinue } |
    ForEach-Object { Get-ItemProperty -Path $_.PSPath -ErrorAction SilentlyContinue } |
    Where-Object { $_.DisplayName -like 'VidOtter*' })
Write-Host "Windows installed-app entries: $($registered.Count)"

python scripts/smoke-mcp-stdio.py (Join-Path $target "bin/vidotter-windows-amd64.exe")
if ($LASTEXITCODE -ne 0) { throw "Installed MCP failed" }
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
    python scripts/smoke-mcp-stdio.py (Join-Path $target "bin/vidotter-windows-amd64.exe")
    if ($LASTEXITCODE -ne 0) { throw "MCP failed after update" }
}
& (Join-Path $target "VidOtter-Maintenance.exe") --confirm-command remove com.thinkerqaq.vidotter
if ($LASTEXITCODE -ne 0) { throw "Windows uninstall failed" }
if (Test-Path $uninstallLink) { throw "Uninstall shortcut was not removed" }
if (!(Test-Path (Join-Path $data "config.toml"))) { throw "Config lost on uninstall" }
if (!(Test-Path (Join-Path $data "downloads/keep.txt"))) { throw "Downloads lost on uninstall" }
if (Test-Path $key) { throw "Native Messaging registry not removed" }
Write-Host "Windows installer smoke passed"

$legacy = Join-Path $sandbox "old unpacked VidOtter"
New-Item -ItemType Directory -Force -Path (Join-Path $legacy "Data") | Out-Null
Set-Content -Path (Join-Path $legacy "Data/config.toml") -Value 'migrate=safe'
& $setup --root $legacy --accept-licenses --confirm-command --auto-answer OverwriteTargetDirectory=Yes install
if ($LASTEXITCODE -ne 0) { throw "Windows legacy migration failed" }
if (!(Test-Path (Join-Path $legacy "components.xml"))) { throw "Legacy install not managed" }
if (-not ((Get-Content -Raw (Join-Path $legacy "Data/config.toml")) -match 'migrate=safe')) { throw "Legacy configuration overwritten" }
& (Join-Path $legacy "VidOtter-Maintenance.exe") --confirm-command remove com.thinkerqaq.vidotter
if ($LASTEXITCODE -ne 0) { throw "Legacy uninstall failed" }
if (!(Test-Path (Join-Path $legacy "Data/config.toml"))) { throw "Legacy config removed" }
Write-Host "Windows legacy adoption preserved Data"
