#!/bin/sh
# Optional VidOtter configuration launcher. No credentials or remote URLs here.
set -eu
root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
case "$(uname -s)" in
  Darwin) os=macos; browser=open ;;
  Linux) os=linux; browser=xdg-open ;;
  *) exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) exit 1 ;;
esac
bridge="$root/bin/vidotter-$os-$arch"
url="http://127.0.0.1:17891/console/popup/popup.html"
[ -x "$bridge" ] || exit 1
up() { command -v curl >/dev/null 2>&1 && curl -fsS --max-time 2 "$url" >/dev/null 2>&1; }
if ! up; then
  nohup "$bridge" --bridge-service </dev/null >/dev/null 2>&1 &
fi
attempt=0
while [ "$attempt" -lt 25 ]; do
  if up; then exec "$browser" "$url"; fi
  attempt=$((attempt + 1))
  sleep 0.2
done
exit 1
