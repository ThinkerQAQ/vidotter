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
old_id=""
if [[ -f "$profile" ]]; then
  old_id="$(sed -nE 's/^[[:space:]]*tunnel_id:[[:space:]]*"?((tunnel_[a-z0-9_]+))"?.*/\1/p' "$profile" | head -n1)"
  [[ "$old_id" =~ ^tunnel_([a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$ ]] || {
    echo "Existing VidOtter profile is invalid. It will not be overwritten." >&2
    exit 1
  }
fi
switching=false
if [[ -n "$old_id" && "$old_id" != "$id" ]]; then
  [[ "$(printenv VIDOTTER_REPLACE_TUNNEL 2>/dev/null || true)" == 1 &&
     "$(printenv VIDOTTER_EXPECTED_TUNNEL_ID 2>/dev/null || true)" == "$old_id" ]] || {
    echo "Changing Tunnel ID requires explicit confirmation using the latest saved ID." >&2
    exit 1
  }
  switching=true
elif [[ "$(printenv VIDOTTER_REPLACE_TUNNEL 2>/dev/null || true)" == 1 ]]; then
  echo "There is no different saved Tunnel ID to replace." >&2
  exit 1
fi

staging=""
rollback=""
replaced=false
completed=false
rollback_on_failure() {
  local status=$?
  trap - EXIT
  if [[ "$status" -ne 0 && "$replaced" == true && -f "$rollback" ]]; then
    if [[ "$os" == "Linux" ]] && command -v systemctl >/dev/null; then
      systemctl --user stop vidotter-tunnel.service >/dev/null 2>&1 || true
    elif [[ "$os" == "Darwin" ]]; then
      launchctl bootout "gui/$(id -u)/com.vidotter.mcp" >/dev/null 2>&1 || true
    fi
    cp -p "$rollback" "$profile"
    if [[ "$os" == "Linux" ]] && command -v systemctl >/dev/null; then
      systemctl --user start vidotter-tunnel.service >/dev/null 2>&1 || true
    elif [[ "$os" == "Darwin" ]]; then
      launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.vidotter.mcp.plist" >/dev/null 2>&1 || true
    fi
    echo "Previous VidOtter Tunnel profile restored." >&2
  fi
  if [[ -n "$staging" && -d "$staging" ]]; then rm -rf "$staging"; fi
  if [[ "$completed" == true && -n "$rollback" && -f "$rollback" ]]; then rm -f "$rollback"; fi
  return "$status"
}
trap rollback_on_failure EXIT
if [[ "$switching" == true ]]; then
  staging="$(mktemp -d "$profile_dir/.vidotter-pending.XXXXXXXX")"
  "$client" init --sample sample_mcp_stdio_local --profile-dir "$staging" --profile vidotter \
    --tunnel-id "$id" --mcp-command "\"$app\" --mcp-stdio" --health-listen-addr 127.0.0.1:0
  candidate="$staging/vidotter.yaml"
  [[ -f "$candidate" ]] || { echo "Candidate Tunnel profile missing" >&2; exit 1; }
  chmod 600 "$candidate"
  "$client" doctor --config "$candidate"
  if [[ "$os" == "Linux" ]] && command -v systemctl >/dev/null; then
    systemctl --user stop vidotter-tunnel.service >/dev/null 2>&1 || true
  elif [[ "$os" == "Darwin" ]]; then
    launchctl bootout "gui/$(id -u)/com.vidotter.mcp" >/dev/null 2>&1 || true
  fi
  rollback="$(mktemp "$profile_dir/.vidotter-rollback.XXXXXXXX")"
  cp -p "$profile" "$rollback"
  mv -f "$candidate" "$profile"
  replaced=true
elif [[ ! -f "$profile" ]]; then
  "$client" init --sample sample_mcp_stdio_local --profile-dir "$profile_dir" --profile vidotter \
    --tunnel-id "$id" --mcp-command "\"$app\" --mcp-stdio" --health-listen-addr 127.0.0.1:0
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
ExecStart="$client" run --config "$profile" --health.url-file "$private/health.url"
Restart=on-failure
RestartSec=5
UMask=0077
[Install]
WantedBy=default.target
EOF
  systemctl --user daemon-reload
  systemctl --user start vidotter-tunnel.service
  echo "VidOtter MCP tunnel started as a systemd user service."
elif [[ "$os" == "Darwin" ]]; then
  runner="$private/run.sh"
  cat > "$runner" <<EOF
#!/bin/bash
set -a
source "$env_file"
set +a
exec "$client" run --config "$profile" --health.url-file "$private/health.url"
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
if [[ "$switching" == true ]]; then
  healthy=false
  for _ in $(seq 1 35); do
    sleep 1
    if "$client" health --url-file "$private/health.url" --require-control-plane-poll --json >/dev/null 2>&1; then
      healthy=true
      break
    fi
  done
  [[ "$healthy" == true ]] || { echo "New Tunnel did not become ready; restoring old profile." >&2; exit 1; }
fi
completed=true
echo "Finish in ChatGPT Plugins -> Add custom MCP server -> Tunnel."
