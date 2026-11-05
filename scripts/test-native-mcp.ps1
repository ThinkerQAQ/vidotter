# Real isolated Windows HKCU Run login-startup and Go worker acceptance. No live credentials,
# global VidOtter tasks, browser, or OpenAI network access are used.
param([switch]$UseBuiltBinary)
$ErrorActionPreference='Stop'
$repo=Split-Path -Parent $PSScriptRoot
if(!$UseBuiltBinary){& go -C (Join-Path $repo 'tools/vidotterdev') run . build --os windows --arch amd64; if($LASTEXITCODE){throw 'Native build failed'}}
$work=Join-Path $repo (".build/native MCP's " + [guid]::NewGuid().ToString('N'))
$bin=Join-Path $work 'bin'
New-Item -ItemType Directory -Path $bin -Force | Out-Null
$executable=Join-Path $bin 'vidotter-windows-amd64.exe'
Copy-Item -LiteralPath (Join-Path $repo '.build/out/vidotter-windows-amd64.exe') -Destination $executable
& go -C (Join-Path $repo 'bridge') test -c -o (Join-Path $bin 'tunnel-client.exe') ./internal/tunnel
if($LASTEXITCODE){throw 'Synthetic Tunnel build failed'}
$oldLocal=$env:LOCALAPPDATA
$oldHome=$env:USERPROFILE
$env:LOCALAPPDATA=Join-Path $work 'local'
$env:USERPROFILE=Join-Path $work 'home'
$root=Join-Path $work 'Data/mcp'
$hash=[Security.Cryptography.SHA256]::Create().ComputeHash([Text.Encoding]::UTF8.GetBytes($root))
$taskName='VidOtter-MCP-'+([Convert]::ToHexString($hash).Substring(0,10).ToLowerInvariant())+'-Login'
$runKey='HKCU:\Software\Microsoft\Windows\CurrentVersion\Run'
function Invoke-Native([string[]]$Arguments){$text=& $executable @Arguments; if($LASTEXITCODE){throw "Native action failed: $Arguments"}; return $text}
function Save-Config($config){$config | ConvertTo-Json -Compress | & $executable --mcp-configure; if($LASTEXITCODE){throw 'Native Save failed'}}
function Read-State{(Invoke-Native -Arguments @('--mcp-service','status')) | ConvertFrom-Json}
function Wait-State([scriptblock]$Expected){$deadline=[DateTime]::UtcNow.AddSeconds(30);do{$state=Read-State;if(&$Expected $state){return $state};Start-Sleep -Milliseconds 200}while([DateTime]::UtcNow -lt $deadline);throw 'Native state did not converge'}
try {
 Write-Host "Saving synthetic connection"; Save-Config @{id='tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';key='SYNTHETIC_RUNTIME_KEY_FIRST_123456';autostart_enabled=$true;tunnel_auto_connect=$false}
 # The helper test executable uses an explicit test entrypoint, never a real Tunnel.
 $setupFile=Join-Path $root 'runtime/setup.json'
 $setup=Get-Content -LiteralPath $setupFile -Raw | ConvertFrom-Json
 $setup.tunnelArgs=@('-test.run=^TestTunnelChildHelper$','--')
 $setup | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $setupFile -Encoding utf8
 $registration=(Get-ItemProperty -Path $runKey -Name $taskName -ErrorAction Stop).$taskName
 if($registration -notlike ('*'+$executable+'*--mcp-worker')){throw 'Native HKCU Run startup path is not owned by synthetic test installation'}
 if((Read-State).managed_process_running){throw 'Save started the local worker'}
 Write-Host "Starting isolated native worker"; Start-Process -FilePath $executable -ArgumentList '--mcp-worker' -WorkingDirectory $work -WindowStyle Hidden
 $state=Wait-State {param($s) $s.managed_process_running -and !$s.tunnel_connecting}
 Invoke-Native -Arguments @('--mcp-service','connect') | Out-Null
 $state=Wait-State {param($s) $s.tunnel_ready}
 $revision=$state.revision
 Write-Host "Saving synthetic connection"; Save-Config @{id='tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb';key='SYNTHETIC_RUNTIME_KEY_SECOND_123456';autostart_enabled=$true;tunnel_auto_connect=$true}
 $state=Read-State
 if(!$state.restart_required -or !$state.managed_process_running -or $state.tunnel_ready -or $state.revision -eq $revision){throw 'Saved rotation incorrectly affected the active revision'}
 Invoke-Native -Arguments @('--mcp-service','disconnect') | Out-Null
 $state=Wait-State {param($s) $s.managed_process_running -and !$s.tunnel_connecting}
 Invoke-Native -Arguments @('--mcp-service','restart') | Out-Null
 $state=Wait-State {param($s) $s.tunnel_ready -and !$s.restart_required -and $s.id -eq 'tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'}
 Invoke-Native -Arguments @('--mcp-service','stop') | Out-Null
 $null=Wait-State {param($s) !$s.managed_process_running -and $s.autostart_enabled}

 Write-Host "Starting isolated native worker"; Start-Process -FilePath $executable -ArgumentList '--mcp-worker' -WorkingDirectory $work -WindowStyle Hidden
 $null=Wait-State {param($s) $s.tunnel_ready}
 Invoke-Native -Arguments @('--mcp-startup','disable') | Out-Null
 $state=Read-State
 if(!$state.managed_process_running -or $state.autostart_enabled){throw 'Disabling startup stopped the current run or was not applied'}
 $credentials=Get-Content -LiteralPath (Join-Path $root 'runtime/credentials.json') -Raw
 if($credentials.Contains('SYNTHETIC_RUNTIME_KEY') -or !$credentials.Contains('protectedKey')){throw 'Windows saved key is not DPAPI protected'}
 Write-Host 'PASS: real isolated login task -> standby -> connect -> rotation -> disconnect -> restart -> ready -> stop -> task relaunch -> disable startup while running; DPAPI protected.'
} finally {
 try{& $executable --mcp-service stop | Out-Null}catch{}
 $registered=(Get-ItemProperty -Path $runKey -Name $taskName -ErrorAction SilentlyContinue).$taskName
 if($registered){
   if($registered -notlike ('*'+$executable+'*--mcp-worker')){throw 'Refusing to clean up a Run entry owned by another installation'}
   Remove-ItemProperty -Path $runKey -Name $taskName
 }
 $env:LOCALAPPDATA=$oldLocal
 $env:USERPROFILE=$oldHome
}
