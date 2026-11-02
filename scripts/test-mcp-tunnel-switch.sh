#!/usr/bin/env bash
# Runs with a fake Tunnel Client and private temporary HOME; no real services.
set -euo pipefail
repo="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work"/{app/scripts,app/bin,mock,home}
cp "$repo/scripts/setup-mcp.sh" "$work/app/scripts/setup-mcp.sh"
printf '#!/bin/sh\nexit 0\n' > "$work/app/bin/fake-vidotter"
chmod +x "$work/app/bin/fake-vidotter"
cat > "$work/mock/tunnel-client" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
action="$1";shift
case "$action" in
  init)
    dir="" name="" id=""
    while (( $# )); do
      case "$1" in
        --profile-dir) dir="$2";shift 2 ;;
        --profile) name="$2";shift 2 ;;
        --tunnel-id) id="$2";shift 2 ;;
        *) shift ;;
      esac
    done
    mkdir -p "$dir"
    printf 'tunnel_id: %s\n' "$id" > "$dir/$name.yaml"
    ;;
  doctor)
    [[ -n "$CONTROL_PLANE_API_KEY" ]] || exit 7
    [[ "$1" == "--config" ]] || exit 8
    if [[ -n "$VIDOTTER_FAKE_BAD_ID" ]] && grep -Fq "$VIDOTTER_FAKE_BAD_ID" "$2"; then exit 9; fi
    ;;
  health)
    [[ "$VIDOTTER_FAKE_FAIL_HEALTH" != 1 ]] || exit 10
    ;;
  run) exit 0 ;;
  *) exit 11 ;;
esac
MOCK
cat > "$work/mock/systemctl" <<'MOCK'
#!/usr/bin/env bash
echo "$*" >> "$HOME/systemctl-calls"
exit 1
MOCK
chmod +x "$work/mock/"*
export HOME="$work/home"
export XDG_CONFIG_HOME="$HOME/.config"
export PATH="$work/mock:$PATH"
export CONTROL_PLANE_API_KEY="FAKE_RUNTIME_KEY_FOR_TEST_ONLY_123"
export VIDOTTER_MCP_BINARY="$work/app/bin/fake-vidotter"
export VIDOTTER_FAKE_BAD_ID=""
export VIDOTTER_FAKE_FAIL_HEALTH=""
export VIDOTTER_MCP_HEALTH_ATTEMPTS=1
old="tunnel_0123456789abcdef0123456789abcdef"
next="tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
bad="tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
profile="$XDG_CONFIG_HOME/tunnel-client/vidotter.yaml"
export VIDOTTER_TUNNEL_ID="$old"
bash "$work/app/scripts/setup-mcp.sh" >/dev/null
grep -Fq "$old" "$profile" || { echo "initial setup failed" >&2; exit 1; }

export VIDOTTER_TUNNEL_ID="$next"
if bash "$work/app/scripts/setup-mcp.sh" >/dev/null 2>&1; then
  echo "unconfirmed replacement unexpectedly succeeded" >&2; exit 1
fi
grep -Fq "$old" "$profile"
export VIDOTTER_REPLACE_TUNNEL=1
export VIDOTTER_EXPECTED_TUNNEL_ID="$bad"
if bash "$work/app/scripts/setup-mcp.sh" >/dev/null 2>&1; then
  echo "stale ID unexpectedly succeeded" >&2; exit 1
fi
grep -Fq "$old" "$profile"
export VIDOTTER_EXPECTED_TUNNEL_ID="$old"
bash "$work/app/scripts/setup-mcp.sh" >/dev/null
grep -Fq "$next" "$profile" || { echo "replacement failed" >&2; exit 1; }

export VIDOTTER_TUNNEL_ID="$bad"
export VIDOTTER_EXPECTED_TUNNEL_ID="$next"
export VIDOTTER_FAKE_BAD_ID="$bad"
if bash "$work/app/scripts/setup-mcp.sh" >/dev/null 2>&1; then
  echo "doctor failure not rejected" >&2; exit 1
fi
grep -Fq "$next" "$profile"
export VIDOTTER_FAKE_BAD_ID=""
export VIDOTTER_FAKE_FAIL_HEALTH=1
export VIDOTTER_TUNNEL_ID="$old"
if bash "$work/app/scripts/setup-mcp.sh" >/dev/null 2>&1; then
  echo "health failure not rejected" >&2; exit 1
fi
grep -Fq "$next" "$profile" || { echo "rollback failed" >&2; exit 1; }
if [[ -f "$HOME/systemctl-calls" ]] && grep -q 'enable' "$HOME/systemctl-calls"; then
  echo "user-disabled WSL autostart was re-enabled" >&2; exit 1
fi
echo "Unix Tunnel switching smoke passed: setup, stale-ID rejection, switch, doctor failure, health rollback."
