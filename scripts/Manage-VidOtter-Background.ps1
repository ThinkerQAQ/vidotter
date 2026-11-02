# One set of per-user Windows Scheduled Tasks for the single installed VidOtter.
# "uninstall" removes tasks/processes, but preserves credentials and preferences.
param([ValidateSet('status','enable','disable','uninstall','restore')][string]$Action='status',
      [ValidateSet('bridge','tunnel')][string]$Target='bridge')
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot)).TrimEnd('\')
$private=Join-Path $env:LOCALAPPDATA 'VidOtter\MCP'
$preferences=Join-Path $private 'background-preferences.json'
$legacy=Join-Path ([Environment]::GetFolderPath('Startup')) 'VidOtter MCP Tunnel.lnk'

function TaskName($kind) { return "VidOtter-$kind" }
function OwnsTask($task) {
    if (!$task -or !$task.Actions -or !$task.Actions[0].WorkingDirectory) { return $false }
    $path=[IO.Path]::GetFullPath($task.Actions[0].WorkingDirectory).TrimEnd('\')
    return [string]::Equals($root, $path, [StringComparison]::OrdinalIgnoreCase)
}
function GetOwnTask($kind) {
    $task=Get-ScheduledTask -TaskName (TaskName $kind) -ErrorAction SilentlyContinue
    if ($task -and !(OwnsTask $task)) {
        throw "Task $(TaskName $kind) belongs to another installation; refusing to replace or delete it."
    }
    return $task
}
function IsEnabled($kind) {
    $task=Get-ScheduledTask -TaskName (TaskName $kind) -ErrorAction SilentlyContinue
    return [bool]($task -and (OwnsTask $task) -and $task.State -ne 'Disabled')
}
function SavePreferences {
    New-Item -ItemType Directory -Force -Path $private | Out-Null
    # Only boolean preferences: never store the Tunnel key here.
    @{bridgeEnabled=(IsEnabled 'bridge'); tunnelEnabled=((IsEnabled 'tunnel') -or (Test-Path $legacy))} |
        ConvertTo-Json -Compress | Set-Content -Encoding UTF8 -Path $preferences
}
function RegisterOwnTask($kind) {
    $name=TaskName $kind
    $null=GetOwnTask $kind
    $program=(Get-Command powershell.exe).Source
    if ($kind -eq 'bridge') {
        $runner=Join-Path $PSScriptRoot 'Run-VidOtter-Bridge.ps1'
    } else {
        $runner=Join-Path $private 'run-mcp.ps1'
        if (!(Test-Path $runner) -or !(Test-Path (Join-Path $private 'vidotter.yaml'))) {
            throw 'Configure ChatGPT MCP first.'
        }
    }
    if (!(Test-Path $runner)) { throw "Missing VidOtter launcher: $runner" }
    $arguments='-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "'+$runner+'"'
    $user=[Security.Principal.WindowsIdentity]::GetCurrent().Name
    $trigger=New-ScheduledTaskTrigger -AtLogOn -User $user
    $principal=New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
    $settings=New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
    $taskAction=New-ScheduledTaskAction -Execute $program -Argument $arguments -WorkingDirectory $root
    Register-ScheduledTask -TaskName $name -Action $taskAction -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
}
function RemoveOwnTask($kind, [bool]$stop) {
    $name=TaskName $kind
    $task=GetOwnTask $kind
    if (!$task) { return }
    if ($stop) {
        Stop-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue
    }
    Unregister-ScheduledTask -TaskName $name -Confirm:$false
}
function StopOwnProcesses {
    $bridgeBinary=Join-Path $root 'bin\vidotter-windows-amd64.exe'
    $profile=Join-Path $private 'vidotter.yaml'
    Get-CimInstance Win32_Process -Filter "Name='vidotter-windows-amd64.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.ExecutablePath -and [string]::Equals($_.ExecutablePath,$bridgeBinary,[StringComparison]::OrdinalIgnoreCase) -and $_.CommandLine -like '*--bridge-service*' } |
        ForEach-Object { Invoke-CimMethod -InputObject $_ -MethodName Terminate -ErrorAction SilentlyContinue | Out-Null }
    Get-CimInstance Win32_Process -Filter "Name='tunnel-client.exe'" -ErrorAction SilentlyContinue |
        Where-Object { $_.CommandLine -and $_.CommandLine.Contains($profile) } |
        ForEach-Object { Invoke-CimMethod -InputObject $_ -MethodName Terminate -ErrorAction SilentlyContinue | Out-Null }
}
if ($Action -eq 'enable') {
    RegisterOwnTask $Target
    if ($Target -eq 'tunnel') { Remove-Item -LiteralPath $legacy -ErrorAction SilentlyContinue }
    SavePreferences
} elseif ($Action -eq 'disable') {
    RemoveOwnTask $Target $false
    if ($Target -eq 'tunnel') { Remove-Item -LiteralPath $legacy -ErrorAction SilentlyContinue }
    SavePreferences
} elseif ($Action -eq 'uninstall') {
    # Validate ownership of ALL tasks before changing anything.
    foreach ($kind in @('bridge','tunnel')) { $null=GetOwnTask $kind }
    SavePreferences
    foreach ($kind in @('bridge','tunnel')) { RemoveOwnTask $kind $true }
    StopOwnProcesses
    Remove-Item -LiteralPath $legacy -ErrorAction SilentlyContinue
} elseif ($Action -eq 'restore' -and (Test-Path $preferences)) {
    $saved=Get-Content -Raw -Path $preferences | ConvertFrom-Json
    foreach ($kind in @('bridge','tunnel')) { $null=GetOwnTask $kind }
    if ($saved.bridgeEnabled) { RegisterOwnTask 'bridge' }
    if ($saved.tunnelEnabled -and (Test-Path (Join-Path $private 'run-mcp.ps1'))) {
        RegisterOwnTask 'tunnel'
    }
}
@{bridgeEnabled=(IsEnabled 'bridge'); tunnelEnabled=((IsEnabled 'tunnel') -or (Test-Path $legacy))} | ConvertTo-Json -Compress
