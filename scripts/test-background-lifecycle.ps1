# CI-only Windows Scheduled Task lifecycle. Uses a temporary installation and fake MCP config.
# Never run against an actual installed VidOtter or user tasks.
$ErrorActionPreference = 'Stop'
$taskNames = @('VidOtter-bridge', 'VidOtter-tunnel')
foreach ($name in $taskNames) {
    if (Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue) {
        throw "Isolated lifecycle test requires an empty Scheduled Task namespace: $name"
    }
}
$work = Join-Path $env:TEMP ('vidotter-task-test-' + [guid]::NewGuid().ToString('N'))
$oldLocal = $env:LOCALAPPDATA
$local = Join-Path $work 'Local'
$first = Join-Path $work 'First'
$second = Join-Path $work 'Second'
$scriptName = 'Manage-VidOtter-Background.ps1'
function RunManager([string]$root, [string]$action, [string]$target = 'bridge') {
    $script = Join-Path $root ('scripts\' + $scriptName)
    $out = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $script -Action $action -Target $target
    if ($LASTEXITCODE -ne 0) { throw "Background manager failed: $action $target" }
    return $out | ConvertFrom-Json
}
try {
    foreach ($dir in @((Join-Path $first 'scripts'), (Join-Path $second 'scripts'), (Join-Path $local 'VidOtter\MCP'))) {
        New-Item -ItemType Directory -Force -Path $dir | Out-Null
    }
    $profileSourceRoot = $first # May contain an 8.3 Windows alias (RUNNER~1).
    $first = (Get-Item -LiteralPath $first).FullName
    $second = (Get-Item -LiteralPath $second).FullName
    foreach ($dir in @($first, $second)) {
        Copy-Item (Join-Path $PSScriptRoot $scriptName) (Join-Path $dir ('scripts\' + $scriptName))
        Copy-Item (Join-Path $PSScriptRoot 'Run-VidOtter-Bridge.ps1') (Join-Path $dir 'scripts\Run-VidOtter-Bridge.ps1')
    }
    $env:LOCALAPPDATA = $local
    $private = Join-Path $local 'VidOtter\MCP'
    $mcpBinary = (Join-Path $profileSourceRoot 'bin\vidotter-windows-amd64.exe').Replace('\','/')
    Set-Content -Path (Join-Path $private 'vidotter.yaml') -Value ('mcp_command: "' + $mcpBinary + '" --mcp-stdio') -Encoding Ascii
    Set-Content -Path (Join-Path $private 'run-mcp.ps1') -Value '# CI fake runtime; never executed' -Encoding Ascii

    $status = RunManager $first 'status'
    if ($status.bridgeEnabled -or $status.tunnelEnabled) { throw 'Fresh install must have auto-start off' }
    $status = RunManager $first 'enable' 'bridge'
    $status = RunManager $first 'enable' 'tunnel'
    if (!$status.bridgeEnabled -or !$status.tunnelEnabled) { throw 'Enabling both tasks failed' }

    $beforeOwner = [string](Get-ScheduledTask -TaskName 'VidOtter-bridge').Actions[0].WorkingDirectory
    $other = Join-Path $second ('scripts\' + $scriptName)
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue' # Expected refusal from the second install.
    try {
        $errorOutput = & powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File $other -Action enable -Target bridge 2>&1
        $badExit = $LASTEXITCODE
    } finally { $ErrorActionPreference = $previousPreference }
    if ($badExit -eq 0) { throw 'Second installation silently stole the first install task' }
    $task = Get-ScheduledTask -TaskName 'VidOtter-bridge'
    $actualOwner = [string]$task.Actions[0].WorkingDirectory
    if (![string]::Equals($actualOwner.TrimEnd('\'), $beforeOwner.TrimEnd('\'), [StringComparison]::OrdinalIgnoreCase)) {
        throw "Second install modified the original task: before '$beforeOwner' after '$actualOwner' (refusal exit $badExit)"
    }

    $status = RunManager $first 'uninstall'
    foreach ($name in $taskNames) {
        if (Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue) { throw "Uninstall left task $name behind" }
    }
    $settings = Join-Path $private 'background-preferences.json'
    if (!(Test-Path $settings)) { throw 'Update preferences were not preserved' }

    $status = RunManager $first 'restore'
    if (!$status.bridgeEnabled -or !$status.tunnelEnabled) { throw 'In-place upgrade did not restore both switches' }

    $status = RunManager $first 'disable' 'tunnel'
    if ($status.tunnelEnabled) { throw 'Disabling Tunnel did not unregister the task' }
    $status = RunManager $first 'uninstall'
    $status = RunManager $first 'restore'
    if (!$status.bridgeEnabled -or $status.tunnelEnabled) { throw 'Restored settings ignored the disabled Tunnel state' }
    Write-Host 'Windows background lifecycle: duplicate-install rejection, uninstall cleanup, restore and disabled preference passed.'
} finally {
    try { $null = RunManager $first 'uninstall' } catch { Write-Warning $_.Exception.Message }
    $env:LOCALAPPDATA = $oldLocal
    Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
}
