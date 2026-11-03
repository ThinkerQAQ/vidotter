#!/usr/bin/env bash
# Isolated Linux systemd user-service lifecycle; never touches the real user manager.
set -euo pipefail
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
first="$work/First VidOtter"
second="$work/Second VidOtter"
export HOME="$work/home" XDG_CONFIG_HOME="$work/home/.config"
export TEST_STATE="$work/mock-state"
mkdir -p "$first/scripts" "$first/bin" "$second/scripts" "$second/bin" \
  "$HOME/.config/vidotter/mcp" "$HOME/.config/tunnel-client" "$work/mock"
cp "$(dirname "$0")/manage-background.sh" "$first/scripts/"
cp "$(dirname "$0")/manage-background.sh" "$second/scripts/"
printf '#!/bin/sh\nexit 0\n' > "$first/bin/vidotter-linux-amd64"
chmod +x "$first/bin/vidotter-linux-amd64"
cp "$first/bin/vidotter-linux-amd64" "$second/bin/"
printf 'mcp_command: "%s/bin/vidotter-linux-amd64" --mcp-stdio\n' "$first" > "$HOME/.config/tunnel-client/vidotter.yaml"
printf 'CONTROL_PLANE_API_KEY=FAKE_CI_KEY\n' > "$HOME/.config/vidotter/mcp/runtime.env"
printf '#!/bin/sh\nexit 0\n' > "$work/mock/tunnel-client"
chmod +x "$work/mock/tunnel-client"
cat > "$work/mock/systemctl" <<'EOF'
#!/usr/bin/env bash
set -e
[[ "$1" == --user ]] || exit 2
shift
operation="$1"; shift
case "$operation" in
  is-enabled)
    [[ "$1" == --quiet ]] && shift
    [[ -f "$TEST_STATE/$1" ]]
    ;;
  enable)
    mkdir -p "$TEST_STATE"
    touch "$TEST_STATE/$1"
    ;;
  disable)
    [[ "$1" == --now ]] && shift
    rm -f "$TEST_STATE/$1"
    ;;
  daemon-reload|stop|start) ;;
  *) echo "unsupported fake systemctl: $operation" >&2; exit 2 ;;
esac
EOF
chmod +x "$work/mock/systemctl"
export PATH="$work/mock:$PATH"
first_script="$first/scripts/manage-background.sh"
second_script="$second/scripts/manage-background.sh"
check_state() {
  local expected_bridge="$1" expected_tunnel="$2" status
  status="$(bash "$first_script" status)"
  [[ "$status" == "{\"bridgeEnabled\":$expected_bridge,\"tunnelEnabled\":$expected_tunnel}" ]] ||
    { echo "Unexpected status: $status" >&2; exit 1; }
}
check_state false false
bash "$first_script" enable bridge >/dev/null
bash "$first_script" enable tunnel >/dev/null
check_state true true
if bash "$second_script" enable bridge >"$work/conflict.log" 2>&1; then
  echo 'Second install stole existing service' >&2; exit 1
fi
grep -Fq "$first/bin/" "$HOME/.config/systemd/user/vidotter-bridge.service"
bash "$first_script" uninstall >/dev/null
[[ ! -f "$HOME/.config/systemd/user/vidotter-bridge.service" ]]
[[ ! -f "$HOME/.config/systemd/user/vidotter-tunnel.service" ]]
[[ -f "$HOME/.config/vidotter/background-preferences" ]]
check_state false false
bash "$first_script" restore >/dev/null
check_state false true
bash "$first_script" disable tunnel >/dev/null
check_state false false
bash "$first_script" uninstall >/dev/null
bash "$first_script" restore >/dev/null
check_state false false
echo 'Linux background lifecycle: no task theft, clean uninstall, persisted preferences and restore passed.'
