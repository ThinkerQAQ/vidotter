#!/usr/bin/env bash
# Linux-only CI smoke: isolate ALL writes to RUNNER_TEMP, never user's Data.
set -euo pipefail
installer="$1"
sandbox="$(mktemp -d "${RUNNER_TEMP:-/tmp}/vidotter-install.XXXXXX")"
export HOME="$sandbox/home" XDG_CONFIG_HOME="$sandbox/home/.config"
export QT_QPA_PLATFORM=offscreen
mkdir -p "$HOME"
target="$sandbox/install with spaces"
"$installer" --root "$target" --accept-licenses --confirm-command install
test -f "$target/extension/manifest.json"
test -x "$target/bin/vidotter-linux-amd64"
manifest="$HOME/.config/google-chrome/NativeMessagingHosts/com.vidotter.bridge.json"
test -f "$manifest"
mkdir -p "$target/Data/downloads"
printf 'kept=true\n' > "$target/Data/config.toml"
printf 'kept\n' > "$target/Data/downloads/keep.txt"
if [[ -n "${VIDOTTER_TEST_UPDATE_REPO:-}" ]]; then
  uri="$(python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).resolve().as_uri())' "$VIDOTTER_TEST_UPDATE_REPO")"
  "$target/VidOtter-Maintenance" --at "$uri" check-updates | grep -q "version=\"${VIDOTTER_TEST_UPDATE_VERSION:-1.6.3}\""
  "$target/VidOtter-Maintenance" --at "$uri" --confirm-command update
  grep -q "<Version>${VIDOTTER_TEST_UPDATE_VERSION:-1.6.3}</Version>" "$target/components.xml"
  test -f "$target/Data/config.toml"
  test -f "$target/Data/downloads/keep.txt"
  test -f "$manifest"
fi
"$target/VidOtter-Maintenance" --confirm-command remove com.thinkerqaq.vidotter
test -f "$target/Data/config.toml"
test -f "$target/Data/downloads/keep.txt"
test ! -e "$manifest"
echo "Linux installer smoke passed: installed, registered, uninstalled, preserved Data."

# Explicit legacy directory adoption: Qt may replace program files, never Data/.
legacy="$sandbox/old unpacked VidOtter"
mkdir -p "$legacy/Data" "$legacy/bin"
printf 'migrate=safe\n' > "$legacy/Data/config.toml"
printf 'old binary\n' > "$legacy/bin/vidotterdev"
"$installer" --root "$legacy" --accept-licenses --confirm-command --auto-answer OverwriteTargetDirectory=Yes install
test -f "$legacy/components.xml"
grep -q 'migrate=safe' "$legacy/Data/config.toml"
"$legacy/VidOtter-Maintenance" --confirm-command remove com.thinkerqaq.vidotter
test -f "$legacy/Data/config.toml"
echo 'Linux legacy adoption preserved Data.'
