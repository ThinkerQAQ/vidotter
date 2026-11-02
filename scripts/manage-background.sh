#!/usr/bin/env bash
set -euo pipefail
umask 077
action="${1:-status}"
target="${2:-bridge}"
[[ "$action" =~ ^(status|enable|disable|uninstall|restore)$ && "$target" =~ ^(bridge|tunnel)$ ]] || exit 2
root="$(cd "$(dirname "$0")/.." && pwd -P)"
os="$(uname -s)"
private="${XDG_CONFIG_HOME:-$HOME/.config}/vidotter"
preferences="$private/background-preferences"
mcp_private="$private/mcp"
profile="${XDG_CONFIG_HOME:-$HOME/.config}/tunnel-client/vidotter.yaml"
service_file() {
  if [[ "$os" == Linux ]]; then
    printf '%s/.config/systemd/user/vidotter-%s.service' "$HOME" "$1"
  else
    local label="com.vidotter.$1"
    [[ "$1" == tunnel ]] && label=com.vidotter.mcp
    printf '%s/Library/LaunchAgents/%s.plist' "$HOME" "$label"
  fi
}
owned_service() {
  local file
  file="$(service_file "$1")"
  [[ -f "$file" ]] || return 0
  if [[ "$1" == bridge ]] && ! grep -Fq -- "$root/bin/" "$file"; then
    echo "VidOtter $1 service belongs to another installation; refusing to modify it" >&2
    return 1
  fi
}
enabled() {
  owned_service "$1" || return 1
  if [[ "$os" == Linux ]]; then
    systemctl --user is-enabled --quiet "vidotter-$1.service" 2>/dev/null
  else
    local label="com.vidotter.$1"
    [[ "$1" == tunnel ]] && label=com.vidotter.mcp
    [[ -f "$(service_file "$1")" ]] &&
      ! launchctl print-disabled "gui/$(id -u)" | grep -F "\"$label\" => true" >/dev/null
  fi
}
save_preferences() {
  mkdir -p "$private"
  local bridge=false tunnel=false
  enabled bridge && bridge=true || true
  enabled tunnel && tunnel=true || true
  printf 'bridge=%s\ntunnel=%s\n' "$bridge" "$tunnel" > "$preferences"
}
write_bridge_service() {
  local bin arch
  arch="$(uname -m)"
  if [[ "$os" == Linux ]]; then
    case "$arch" in
      x86_64) bin="$root/bin/vidotter-linux-amd64" ;;
      aarch64|arm64) bin="$root/bin/vidotter-linux-arm64" ;;
      *) echo "Unsupported architecture" >&2; return 1 ;;
    esac
    [[ -x "$bin" ]] || { echo 'Bridge binary missing' >&2; return 1; }
    local safe="${bin//\\/\\\\}"
    safe="${safe//\"/\\\"}"; safe="${safe//%/%%}"; safe="${safe//\$/\$\$}"
    mkdir -p "$HOME/.config/systemd/user"
    cat > "$(service_file bridge)" <<EOF
[Unit]
Description=VidOtter Bridge
[Service]
ExecStart="$safe" --bridge-service
Restart=on-failure
RestartSec=5
UMask=0077
[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
  else
    [[ "$arch" == x86_64 ]] && arch=amd64
    bin="$root/bin/vidotter-macos-$arch"
    [[ -x "$bin" ]] || { echo 'Bridge binary missing' >&2; return 1; }
    local xml
    xml="$(printf '%s' "$bin" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g')"
    mkdir -p "$HOME/Library/LaunchAgents"
    cat > "$(service_file bridge)" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Label</key><string>com.vidotter.bridge</string>
<key>ProgramArguments</key><array><string>$xml</string><string>--bridge-service</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
EOF
  fi
}
write_tunnel_service_if_missing() {
  local file client
  file="$(service_file tunnel)"
  [[ -f "$file" ]] && return 0
  [[ -f "$profile" ]] || { echo 'Configure MCP first' >&2; return 1; }
  grep -Fq -- "$root/bin/" "$profile" || { echo 'Tunnel profile belongs to another installation; reconfigure MCP first' >&2; return 1; }
  client="$(command -v tunnel-client || true)"
  [[ -n "$client" ]] || client="$HOME/.local/bin/tunnel-client"
  [[ -x "$client" ]] || { echo 'Tunnel Client missing' >&2; return 1; }
  if [[ "$os" == Linux ]]; then
    [[ -f "$mcp_private/runtime.env" ]] || { echo 'Runtime key missing' >&2; return 1; }
    mkdir -p "$(dirname "$file")"
    cat > "$file" <<EOF
[Unit]
Description=VidOtter MCP Tunnel
After=network-online.target
Wants=network-online.target
[Service]
EnvironmentFile="$mcp_private/runtime.env"
ExecStart="$client" run --config "$profile" --health.url-file "$mcp_private/health.url"
Restart=on-failure
RestartSec=5
UMask=0077
[Install]
WantedBy=default.target
EOF
    systemctl --user daemon-reload
  else
    local runner="$mcp_private/run.sh"
    [[ -x "$runner" ]] || { echo 'Tunnel runner missing' >&2; return 1; }
    mkdir -p "$(dirname "$file")"
    local xml
    xml="$(printf '%s' "$runner" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g')"
    cat > "$file" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Label</key><string>com.vidotter.mcp</string>
<key>ProgramArguments</key><array><string>$xml</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
EOF
  fi
}
toggle() {
  local kind="$1" mode="$2" label
  owned_service "$kind"
  if [[ "$kind" == tunnel && "$mode" == enable ]]; then
    [[ -f "$profile" ]] && grep -Fq -- "$root/bin/" "$profile" || {
      echo 'Tunnel profile belongs to another installation; reconfigure MCP first' >&2
      return 1
    }
  fi
  if [[ "$os" == Linux ]]; then
    if [[ "$mode" == enable ]]; then
      if [[ "$kind" == bridge ]]; then write_bridge_service
      else write_tunnel_service_if_missing; fi
      systemctl --user enable "vidotter-$kind.service" >/dev/null
    else
      systemctl --user disable "vidotter-$kind.service" >/dev/null 2>&1 || true
    fi
  else
    label="com.vidotter.$kind"; [[ "$kind" == tunnel ]] && label=com.vidotter.mcp
    if [[ "$mode" == enable ]]; then
      if [[ "$kind" == bridge ]]; then write_bridge_service
      else write_tunnel_service_if_missing; fi
      launchctl enable "gui/$(id -u)/$label"
      launchctl print "gui/$(id -u)/$label" >/dev/null 2>&1 ||
        launchctl bootstrap "gui/$(id -u)" "$(service_file "$kind")"
    else
      launchctl disable "gui/$(id -u)/$label"
    fi
  fi
}
case "$action" in
  enable|disable)
    toggle "$target" "$action"
    save_preferences
    ;;
  uninstall)
    owned_service bridge
    owned_service tunnel
    save_preferences
    for kind in bridge tunnel; do
      if [[ "$os" == Linux ]]; then
        systemctl --user disable --now "vidotter-$kind.service" >/dev/null 2>&1 || true
      else
        label="com.vidotter.$kind"; [[ "$kind" == tunnel ]] && label=com.vidotter.mcp
        launchctl bootout "gui/$(id -u)/$label" >/dev/null 2>&1 || true
        launchctl disable "gui/$(id -u)/$label" >/dev/null 2>&1 || true
      fi
      rm -f "$(service_file "$kind")"
    done
    [[ "$os" != Linux ]] || systemctl --user daemon-reload >/dev/null 2>&1 || true
    ;;
  restore)
    if [[ -f "$preferences" ]]; then
      owned_service bridge
      owned_service tunnel
      if grep -qx 'bridge=true' "$preferences"; then toggle bridge enable; fi
      if grep -qx 'tunnel=true' "$preferences" && [[ -f "$profile" ]] && grep -Fq -- "$root/bin/" "$profile"; then
        toggle tunnel enable
      fi
    fi
    ;;
esac
bridge=false; tunnel=false
enabled bridge && bridge=true || true
enabled tunnel && tunnel=true || true
printf '{"bridgeEnabled":%s,"tunnelEnabled":%s}\n' "$bridge" "$tunnel"
