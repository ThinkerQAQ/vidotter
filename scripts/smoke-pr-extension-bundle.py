#!/usr/bin/env python3
"""CI acceptance: ensure the *installed* unpacked extension and worker agree.

Unlike unit tests of the source tree, this catches a Qt package carrying an
updated popup alongside an older worker, or a candidate with a stale version.
"""
import json
from pathlib import Path
import sys

extension = Path(sys.argv[1])
manifest = json.loads((extension / "manifest.json").read_text(encoding="utf-8"))
version = manifest["version"]
parts = version.split(".")
if len(parts) != 4 or any(not x.isdigit() or int(x) < 0 or int(x) > 65535 for x in parts):
    raise RuntimeError(f"PR preview must have unique four-part browser version; got {version}")
if manifest.get("background", {}).get("service_worker") != "background.js":
    raise RuntimeError("Incorrect service worker manifest entry")

background = (extension / "background.js").read_text(encoding="utf-8")
popup = (extension / "popup" / "environment.js").read_text(encoding="utf-8")
relay = (extension / "console-relay.js").read_text(encoding="utf-8")
for marker in (
    'message.action === "bridge.mcp.capabilities"',
    'action === "install-client"',
    '"/v1/mcp/client/install"',
    'action === "service"',
    '"/v1/mcp/service"',
):
    if marker not in background:
        raise RuntimeError(f"Installed worker missing MCP action: {marker}")
for marker in ("bridge.mcp.capabilities", "showMcpWorkerReload", "renderChatGPTMCP"):
    if marker not in popup:
        raise RuntimeError(f"Installed popup missing compatibility handling: {marker}")
if "bridge.mcp.capabilities" not in relay:
    raise RuntimeError("Installed browser relay lacks capability handshake")
print(f"PR extension packaging accepted: UI + worker + relay, browser version {version}")
