#!/usr/bin/env bash
# macOS CI only. Mount candidate DMG and test inside a disposable HOME.
set -euo pipefail
dmg="$1"
sandbox="$(mktemp -d "${RUNNER_TEMP:-/tmp}/vidotter-macos.XXXXXX")"
export HOME="$sandbox/home"
mkdir -p "$HOME" "$sandbox/mount"
hdiutil attach -quiet -nobrowse -mountpoint "$sandbox/mount" "$dmg"
trap 'hdiutil detach -quiet "$sandbox/mount" || true' EXIT
app="$(find "$sandbox/mount" -maxdepth 2 -type d -name '*.app' | head -n 1)"
test -n "$app"
installer="$(find "$app/Contents/MacOS" -maxdepth 1 -type f | head -n 1)"
test -x "$installer"
target="$sandbox/install with spaces"
"$installer" --root "$target" --accept-licenses --confirm-command install
test -f "$target/extension/manifest.json"
manifest="$HOME/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.vidotter.bridge.json"
test -f "$manifest"
mkdir -p "$target/Data/downloads"
printf 'sentinel=true\n' > "$target/Data/config.toml"
printf 'keep\n' > "$target/Data/downloads/keep.txt"
if [[ -d "$target/VidOtter-Maintenance.app" ]]; then
  maintenance="$(find "$target/VidOtter-Maintenance.app/Contents/MacOS" -maxdepth 1 -type f | head -n 1)"
else
  maintenance="$target/VidOtter-Maintenance"
fi
test -x "$maintenance"
"$maintenance" --confirm-command remove com.thinkerqaq.vidotter
test -f "$target/Data/config.toml"
test -f "$target/Data/downloads/keep.txt"
test ! -e "$manifest"
echo "macOS Intel installer smoke passed"
