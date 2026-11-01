#!/usr/bin/env python3
"""Smoke-test the MCP protocol against the exact binary installed by Qt IFW."""
import json
from pathlib import Path
import queue
import subprocess
import sys
import threading


def main() -> None:
    binary = Path(sys.argv[1]).resolve()
    if not binary.is_file():
        raise RuntimeError(f"MCP binary missing: {binary}")

    process = subprocess.Popen([str(binary), "--mcp-stdio"], stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                               text=True, bufsize=1)
    replies: queue.Queue[dict] = queue.Queue()
    def consume() -> None:
        assert process.stdout is not None
        for raw in process.stdout:
            try:
                replies.put(json.loads(raw))
            except json.JSONDecodeError:
                continue
    thread = threading.Thread(target=consume, daemon=True)
    thread.start()

    def send(frame: dict) -> None:
        assert process.stdin is not None
        process.stdin.write(json.dumps(frame, separators=(",", ":")) + "\n")
        process.stdin.flush()

    def wait_for(target: int) -> dict:
        while True:
            result = replies.get(timeout=12)
            if result.get("id") != target:
                continue
            if "error" in result:
                raise RuntimeError(f"MCP error: {result['error']}")
            return result["result"]

    try:
        send({"jsonrpc": "2.0", "id": 1, "method": "initialize",
              "params": {"protocolVersion": "2025-06-18", "capabilities": {},
                         "clientInfo": {"name": "vidotter-installer-smoke",
                                        "version": "1"}}})
        init = wait_for(1)
        if "protocolVersion" not in init:
            raise RuntimeError("Missing MCP protocolVersion")
        send({"jsonrpc": "2.0", "method": "notifications/initialized"})
        send({"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}})
        tools = wait_for(2).get("tools", [])
        if len(tools) != 13:
            raise RuntimeError(f"Expected 13 MCP tools; found {len(tools)}")
        print(f"Installed MCP stdio ready: {len(tools)} tools")
    finally:
        if process.stdin:
            process.stdin.close()
        try:
            process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            process.terminate()
            process.wait(timeout=5)


if __name__ == "__main__":
    main()
