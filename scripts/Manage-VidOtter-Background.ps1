# Per-user background registration. No administrator permissions or new runtime.
param([ValidateSet('status','enable','disable')][string]$Action='status',
      [ValidateSet('bridge','tunnel')][string]$Target='bridge')
$ErrorActionPreference='Stop'
$root=Split-Path -Parent $PSScriptRoot
$private=Join-Path $env:LOCALAPPDATA 'VidOtter\MCP'
$legacy=Join-Path ([Environment]::GetFolderPath('Startup')) 'VidOtter MCP Tunnel.lnk'
function TaskName($kind) { return "VidOtter-$kind" }
function IsEnabled($kind) {
    $task=Get-ScheduledTask -TaskName (TaskName $kind) -ErrorAction SilentlyContinue
    return [bool]($task -and $task.State -ne 'Disabled')
}
if ($Action -ne 'status') {
    $name=TaskName $Target
    if ($Action -eq 'disable') {
        # Unregistering does not stop this session's Bridge or disconnect Tunnel.
        if (Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue) {
            Unregister-ScheduledTask -TaskName $name -Confirm:$false
        }
    } else {
        if ($Target -eq 'bridge') {
            $program=(Get-Command powershell.exe).Source
            $runner=Join-Path $PSScriptRoot 'Run-VidOtter-Bridge.ps1'
            $arguments='-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "'+$runner+'"'
        } else {
            $runner=Join-Path $private 'run-mcp.ps1'
            if (!(Test-Path $runner) -or !(Test-Path (Join-Path $private 'vidotter.yaml'))) {
                throw 'Configure ChatGPT MCP first.'
            }
            $program=(Get-Command powershell.exe).Source
            $arguments='-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "'+$runner+'"'
        }
        $user=[Security.Principal.WindowsIdentity]::GetCurrent().Name
        $trigger=New-ScheduledTaskTrigger -AtLogOn -User $user
        $principal=New-ScheduledTaskPrincipal -UserId $user -LogonType Interactive -RunLevel Limited
        $settings=New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries
        $taskAction=New-ScheduledTaskAction -Execute $program -Argument $arguments -WorkingDirectory $root
        Register-ScheduledTask -TaskName $name -Action $taskAction -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
    }
    if ($Target -eq 'tunnel') { Remove-Item -LiteralPath $legacy -ErrorAction SilentlyContinue }
}
@{bridgeEnabled=(IsEnabled 'bridge'); tunnelEnabled=((IsEnabled 'tunnel') -or (Test-Path $legacy))} | ConvertTo-Json -Compress
