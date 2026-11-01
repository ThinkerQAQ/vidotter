#!/usr/bin/env bash
set -euo pipefail
action="${1:-}"
[[ "$action" =~ ^(start|stop|restart)$ ]] || { echo 'Use start|stop|restart' >&2; exit 2; }
os="$(uname -s)"
if [[ "$os" == "Linux" ]]; then
    command -v systemctl >/dev/null || { echo 'systemd user service is not available.' >&2; exit 1; }
    systemctl --user cat vidotter-tunnel.service >/dev/null 2>&1 ||
        { echo 'VidOtter Tunnel user service is not configured.' >&2; exit 1; }
    systemctl --user "$action" vidotter-tunnel.service
elif [[ "$os" == "Darwin" ]]; then
    uid="$(id -u)"
    label="gui/$uid/com.vidotter.mcp"
    agent="$HOME/Library/LaunchAgents/com.vidotter.mcp.plist"
    [[ -f "$agent" ]] || { echo 'VidOtter Tunnel LaunchAgent not configured.' >&2; exit 1; }
    case "$action" in
      stop)
        launchctl bootout "$label" || { echo 'Tunnel already stopped or LaunchAgent unavailable' >&2; exit 1; } ;;
      start)
        if launchctl print "$label" >/dev/null 2>&1; then
          launchctl kickstart "$label"
        else
          launchctl bootstrap "gui/$uid" "$agent"
        fi ;;
      restart)
        launchctl bootout "$label" >/dev/null 2>&1 || true
        launchctl bootstrap "gui/$uid" "$agent" ;;
    esac
else
    echo 'Unsupported operating system' >&2
    exit 1
fi
echo 'VidOtter Tunnel service action issued; verify health in VidOtter Settings.'
