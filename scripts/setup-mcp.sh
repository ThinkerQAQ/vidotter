#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
os="$(uname -s)"
arch="$(uname -m)"
case "$os:$arch" in
  Linux:x86_64) bin="vidotter-linux-amd64" ;;
  Linux:aarch64|Linux:arm64) bin="vidotter-linux-arm64" ;;
  Darwin:x86_64) bin="vidotter-macos-amd64" ;;
  Darwin:arm64) bin="vidotter-macos-arm64" ;;
  *) echo "Unsupported architecture: $os/$arch" >&2; exit 1 ;;
esac
app="${VIDOTTER_MCP_BINARY:-$root/bin/$bin}"
[[ -x "$app" ]] || { echo "VidOtter binary missing: $app" >&2; exit 1; }
client="$(command -v tunnel-client || true)"
[[ -n "$client" ]] || client="$HOME/.local/bin/tunnel-client"
[[ -x "$client" ]] || { echo "Install official openai/tunnel-client first: https://github.com/openai/tunnel-client" >&2; exit 1; }
id="${VIDOTTER_TUNNEL_ID:-}"
if [[ -z "$id" ]]; then read -r -p "VidOtter Tunnel ID: " id; fi
[[ "$id" =~ ^tunnel_([a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$ ]] || { echo "Invalid Tunnel ID" >&2; exit 1; }
private="${XDG_CONFIG_HOME:-$HOME/.config}/vidotter/mcp"
profile_dir="${XDG_CONFIG_HOME:-$HOME/.config}/tunnel-client"
mkdir -p "$private" "$profile_dir"
chmod 700 "$private"
env_file="$private/runtime.env"
if [[ ! -f "$env_file" ]]; then
  key="${CONTROL_PLANE_API_KEY:-}"
  if [[ -z "$key" ]]; then
    read -r -s -p "Restricted runtime API key (Tunnels Read + Use): " key
    printf '\n'
  fi
  [[ "$key" =~ ^[a-zA-Z0-9_-]{16,}$ ]] || { echo "Invalid runtime key" >&2; exit 1; }
  (umask 077; printf 'CONTROL_PLANE_API_KEY=%s\n' "$key" > "$env_file")
fi
chmod 600 "$env_file"
set -a
# shellcheck disable=SC1090
source "$env_file"
set +a
profile="$profile_dir/vidotter.yaml"
if [[ -f "$profile" ]] && ! grep -Fq -- "$id" "$profile"; then
  echo "Tunnel ID differs from existing VidOtter profile. Reuse the saved ID or explicitly edit the profile." >&2
  exit 1
fi
if [[ ! -f "$profile" ]]; then
  "$client" init --sample sample_mcp_stdio_local --profile-dir "$profile_dir" --profile vidotter \
    --tunnel-id "$id" --mcp-command "$app --mcp-stdio" --health-listen-addr 127.0.0.1:0
fi
chmod 600 "$profile"
"$client" doctor --config "$profile"
if [[ "$os" == "Linux" ]] && command -v systemctl >/dev/null && systemctl --user is-system-running >/dev/null 2>&1; then
  unit="$HOME/.config/systemd/user/vidotter-tunnel.service"
  mkdir -p "$(dirname "$unit")"
  cat > "$unit" <<EOF
[Unit]
Description=VidOtter MCP Tunnel
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
EnvironmentFile="$env_file"
ExecStart="$client" run --config "$profile"
Restart=on-failure
RestartSec=5
UMask=0077
[Install]
WantedBy=default.target
EOF
  systemctl --user daemon-reload
  systemctl --user enable --now vidotter-tunnel.service
  echo "VidOtter MCP tunnel started as a systemd user service."
elif [[ "$os" == "Darwin" ]]; then
  runner="$private/run.sh"
  cat > "$runner" <<EOF
#!/bin/bash
set -a
source "$env_file"
set +a
exec "$client" run --config "$profile"
EOF
  chmod 700 "$runner"
  runner_xml="$(printf "%s" "$runner" | sed -e "s/\&/\&amp;/g" -e "s/</\&lt;/g" -e "s/>/\&gt;/g")"
  agent="$HOME/Library/LaunchAgents/com.vidotter.mcp.plist"
  mkdir -p "$(dirname "$agent")"
  cat > "$agent" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.vidotter.mcp</string>
<key>ProgramArguments</key><array><string>$runner_xml</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
EOF
  launchctl bootout "gui/$(id -u)/com.vidotter.mcp" >/dev/null 2>&1 || true
  if launchctl bootstrap "gui/$(id -u)" "$agent"; then
    echo "VidOtter MCP tunnel started through LaunchAgent."
  else
    echo "No GUI launchctl session. Start manually: $client run --config $profile" >&2
  fi
else
  echo "No supported service manager. Start manually: $client run --config $profile"
fi
echo "Finish in ChatGPT Plugins -> Add custom MCP server -> Tunnel."
