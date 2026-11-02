#!/usr/bin/env bash
set -euo pipefail
action="${1:-}"
[[ "$action" =~ ^(start|stop|restart)$ ]] || { echo 'Use start|stop|restart' >&2; exit 2; }
os="$(uname -s)"
private="${XDG_CONFIG_HOME:-$HOME/.config}/vidotter/mcp"
profile="${XDG_CONFIG_HOME:-$HOME/.config}/tunnel-client/vidotter.yaml"
client="$(command -v tunnel-client || true)"
[[ -n "$client" ]] || client="$HOME/.local/bin/tunnel-client"
create_linux_unit() {
    local unit="$HOME/.config/systemd/user/vidotter-tunnel.service"
    [[ -f "$profile" && -s "$private/runtime.env" && -x "$client" ]] || {
        echo 'Save Tunnel ID and Runtime API Key before starting.' >&2; return 1;
    }
    mkdir -p "$(dirname "$unit")"
    cat > "$unit" <<EOF
[Unit]
Description=VidOtter MCP Tunnel
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
EnvironmentFile="$private/runtime.env"
ExecStart="$client" run --config "$profile" --health.url-file "$private/health.url"
Restart=on-failure
RestartSec=5
UMask=0077
[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
}
create_macos_agent() {
    local agent="$HOME/Library/LaunchAgents/com.vidotter.mcp.plist"
    local runner="$private/run.sh"
    [[ -f "$profile" && -s "$private/runtime.env" && -x "$client" ]] || {
        echo 'Save Tunnel ID and Runtime API Key before starting.' >&2; return 1;
    }
    mkdir -p "$(dirname "$agent")" "$private"
    cat > "$runner" <<EOF
#!/bin/bash
set -a
source "$private/runtime.env"
set +a
exec "$client" run --config "$profile" --health.url-file "$private/health.url"
EOF
    chmod 700 "$runner"
    local runner_xml
    runner_xml="$(printf '%s' "$runner" | sed -e 's/\&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g')"
    cat > "$agent" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Label</key><string>com.vidotter.mcp</string>
<key>ProgramArguments</key><array><string>$runner_xml</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
EOF
    chmod 600 "$agent"
}

if [[ "$os" == "Linux" ]]; then
    command -v systemctl >/dev/null || { echo 'systemd user service is not available.' >&2; exit 1; }
    if [[ "$action" == start || "$action" == restart ]]; then
        if ! systemctl --user cat vidotter-tunnel.service >/dev/null 2>&1; then
            create_linux_unit
        fi
    fi
    systemctl --user "$action" vidotter-tunnel.service
elif [[ "$os" == "Darwin" ]]; then
    uid="$(id -u)"
    label="gui/$uid/com.vidotter.mcp"
    agent="$HOME/Library/LaunchAgents/com.vidotter.mcp.plist"
    first_start=false
    if [[ ! -f "$agent" && "$action" != stop ]]; then
        create_macos_agent
        first_start=true
    fi
    [[ -f "$agent" ]] || { echo 'VidOtter Tunnel LaunchAgent not configured.' >&2; exit 1; }
    # A manual connection must also work when login auto-connect is disabled.
    bootstrap_once() {
        local disabled=false
        if launchctl print-disabled "gui/$uid" | grep -F "\"com.vidotter.mcp\" => true" >/dev/null; then
            disabled=true
            launchctl enable "$label"
        fi
        local result=0
        launchctl bootstrap "gui/$uid" "$agent" || result=$?
        if [[ "$disabled" == true ]]; then launchctl disable "$label"; fi
        return "$result"
    }
    case "$action" in
      stop)
        launchctl bootout "$label" || { echo 'Tunnel already stopped or LaunchAgent unavailable' >&2; exit 1; } ;;
      start)
        if launchctl print "$label" >/dev/null 2>&1; then
          launchctl kickstart "$label"
        else
          bootstrap_once
          if [[ "$first_start" == true ]]; then launchctl disable "$label"; fi
        fi ;;
      restart)
        launchctl bootout "$label" >/dev/null 2>&1 || true
        bootstrap_once
        if [[ "$first_start" == true ]]; then launchctl disable "$label"; fi ;;
    esac
else
    echo 'Unsupported operating system' >&2
    exit 1
fi
echo 'VidOtter Tunnel service action issued; verify health in VidOtter Settings.'
