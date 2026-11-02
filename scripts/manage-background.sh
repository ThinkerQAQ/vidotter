#!/usr/bin/env bash
set -euo pipefail
action="${1:-status}"
target="${2:-bridge}"
[[ "$action" =~ ^(status|enable|disable)$ && "$target" =~ ^(bridge|tunnel)$ ]] || exit 2
root="$(cd "$(dirname "$0")/.." && pwd)"
os="$(uname -s)"
enabled() {
  if [[ "$os" == Linux ]]; then
    systemctl --user is-enabled --quiet "vidotter-$1.service" 2>/dev/null
  else
    local label="com.vidotter.$1"
    [[ "$1" == tunnel ]] && label=com.vidotter.mcp
    [[ -f "$HOME/Library/LaunchAgents/$label.plist" ]] &&
      ! launchctl print-disabled "gui/$(id -u)" | grep -F "\"$label\" => true" >/dev/null
  fi
}
if [[ "$os" == Linux ]]; then
  if [[ "$action" == enable ]]; then
    if [[ "$target" == bridge ]]; then
      bin="$root/bin/vidotter-linux-$(uname -m)"
      [[ "$(uname -m)" == x86_64 ]] && bin="$root/bin/vidotter-linux-amd64"
      [[ "$(uname -m)" == aarch64 ]] && bin="$root/bin/vidotter-linux-arm64"
      [[ -x "$bin" ]] || { echo 'Bridge binary missing' >&2; exit 1; }
      # systemd specifiers and environment expansion must remain literal in paths.
      safe="${bin//\\/\\\\}"; safe="${safe//\"/\\\"}"; safe="${safe//%/%%}"; safe="${safe//\$/\$\$}"
      mkdir -p "$HOME/.config/systemd/user"
      cat > "$HOME/.config/systemd/user/vidotter-bridge.service" <<EOF
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
    fi
    systemctl --user enable "vidotter-$target.service" >/dev/null
  elif [[ "$action" == disable ]]; then
    systemctl --user disable "vidotter-$target.service" >/dev/null
  fi
elif [[ "$os" == Darwin ]]; then
  label="com.vidotter.$target"
  [[ "$target" == tunnel ]] && label=com.vidotter.mcp
  agent="$HOME/Library/LaunchAgents/$label.plist"
  if [[ "$action" == enable ]]; then
    if [[ "$target" == bridge ]]; then
      arch="$(uname -m)"; [[ "$arch" == x86_64 ]] && arch=amd64
      bin="$root/bin/vidotter-macos-$arch"
      [[ -x "$bin" ]] || { echo 'Bridge binary missing' >&2; exit 1; }
      mkdir -p "$(dirname "$agent")"
      xml="$(printf '%s' "$bin" | sed -e 's/&/\&amp;/g' -e 's/</\&lt;/g' -e 's/>/\&gt;/g')"
      cat > "$agent" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict>
<key>Label</key><string>$label</string>
<key>ProgramArguments</key><array><string>$xml</string><string>--bridge-service</string></array>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
</dict></plist>
EOF
    fi
    [[ -f "$agent" ]] || { echo 'Configure MCP first' >&2; exit 1; }
    launchctl enable "gui/$(id -u)/$label"
    launchctl print "gui/$(id -u)/$label" >/dev/null 2>&1 || launchctl bootstrap "gui/$(id -u)" "$agent"
  elif [[ "$action" == disable ]]; then
    launchctl disable "gui/$(id -u)/$label"
  fi
else
  echo 'Unsupported background service platform' >&2; exit 1
fi
bridge=false; tunnel=false
enabled bridge && bridge=true
enabled tunnel && tunnel=true
printf '{"bridgeEnabled":%s,"tunnelEnabled":%s}\n' "$bridge" "$tunnel"
