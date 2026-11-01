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
"$target/VidOtter-Maintenance" --confirm-command remove com.thinkerqaq.vidotter
test -f "$target/Data/config.toml"
test -f "$target/Data/downloads/keep.txt"
test ! -e "$manifest"
echo "Linux installer smoke passed: installed, registered, uninstalled, preserved Data."
