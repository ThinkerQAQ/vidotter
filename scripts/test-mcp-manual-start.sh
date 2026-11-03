#!/usr/bin/env bash
# Isolated Linux manual-start regression: Save creates only credentials;
# Start registers a user unit, without enabling login startup.
set -euo pipefail
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
export HOME="$work/home" XDG_CONFIG_HOME="$work/home/.config" TEST_CALLS="$work/calls"
mkdir -p "$work/mock" "$HOME/.config/vidotter/mcp" "$HOME/.config/tunnel-client"
printf 'tunnel_id: tunnel_0123456789abcdef0123456789abcdef\n' > "$HOME/.config/tunnel-client/vidotter.yaml"
printf 'CONTROL_PLANE_API_KEY=FAKE_TEST_VALUE\n' > "$HOME/.config/vidotter/mcp/runtime.env"
printf '#!/bin/sh\nexit 0\n' > "$work/mock/tunnel-client"
cat > "$work/mock/systemctl" <<'EOF'
#!/usr/bin/env bash
echo "$*" >> "$TEST_CALLS"
[[ "$1" == --user ]] || exit 2
shift
case "$1" in
  cat) [[ -f "$HOME/.config/systemd/user/vidotter-tunnel.service" ]] ;;
  daemon-reload|start|stop|restart) exit 0 ;;
  enable|disable) echo 'manual Start unexpectedly changed login autostart' >&2; exit 6 ;;
  *) exit 5 ;;
esac
EOF
cat > "$work/mock/uname" <<'EOF'
#!/usr/bin/env bash
if [[ "$1" == -s ]]; then echo Linux
elif [[ "$1" == -m ]]; then echo x86_64
else /usr/bin/uname "$@"; fi
EOF
chmod +x "$work/mock/"*
export PATH="$work/mock:$PATH"
bash "$(dirname "$0")/manage-mcp.sh" start >/dev/null
unit="$HOME/.config/systemd/user/vidotter-tunnel.service"
[[ -f "$unit" ]] || { echo 'Manual start failed to create unit' >&2; exit 1; }
grep -Fq "$HOME/.config/tunnel-client/vidotter.yaml" "$unit"
! grep -q -- '--user enable' "$TEST_CALLS"
bash "$(dirname "$0")/manage-mcp.sh" stop >/dev/null
! grep -q -- '--user enable' "$TEST_CALLS"
echo 'Linux manual MCP Start/Stop: creates unit on demand, never enables login startup.'
