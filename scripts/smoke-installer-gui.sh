#!/usr/bin/env bash
# Qt GUI wizard regression without a real display, isolated from user settings.
set -euo pipefail
installer="$1"
sandbox="$(mktemp -d "${RUNNER_TEMP:-/tmp}/vidotter-gui-smoke.XXXXXX")"
export HOME="$sandbox/home" XDG_CONFIG_HOME="$sandbox/home/.config" QT_QPA_PLATFORM=minimal
mkdir -p "$HOME"
"$installer" --script scripts/installer-gui-smoke.qs
log="$HOME/VidOtter/InstallationLog.txt"
test -f "$log"
grep -q 'VIDOTTER_GUI_INTRO' "$log"
grep -q 'VIDOTTER_GUI_TARGET' "$log"
grep -q 'VIDOTTER_GUI_LICENSE' "$log"
grep -q 'VIDOTTER_GUI_READY' "$log"
grep -q 'VIDOTTER_GUI_FINISHED' "$log"
grep -q 'VIDOTTER_GUI_EXTENSION_OPTION_PRESENT' "$log"
grep -q 'VIDOTTER_GUI_MCP_OPTION_PRESENT' "$log"
if grep -q 'VIDOTTER_GUI_BAD_COMPONENT_SELECTION' "$log"; then
    echo "Unexpected component selection page" >&2
    exit 1
fi
"$HOME/VidOtter/VidOtter-Maintenance" --confirm-command remove com.thinkerqaq.vidotter >/dev/null
echo 'Qt GUI smoke passed: one product, browser+MCP options on Finish.'
