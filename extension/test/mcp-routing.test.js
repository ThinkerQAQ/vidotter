"use strict";

// Execute the actual registered MV3 onMessage handler, not just text-match
// it. A stale worker used to respond "不支持的 Tool 操作" to the new popup.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const source = fs.readFileSync(path.join(__dirname, "../background.js"), "utf8");
const marker = "chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {";
const location = source.lastIndexOf(marker);
assert.ok(location >= 0);
const registeredHandler = source.slice(location).trim();
assert.ok(registeredHandler.endsWith("});"));

let handler;
const calls = [];
vm.runInNewContext(registeredHandler, {
  chrome: { runtime: { onMessage: { addListener(fn) { handler = fn; } } } },
  bridgeFetch: async (url, options) => {
    calls.push({ url, options });
    return { ok: true };
  },
  // Only the tested action cases execute. Other background services remain
  // mocked out, and no external download/network occurs.
});

function request(action) {
  return new Promise((resolve, reject) => {
    const staysOpen = handler(action, {}, resolve);
    if (staysOpen !== true && action.action !== "bridge.mcp.capabilities") {
      reject(new Error("new MCP action did not retain async response channel"));
    }
  });
}
(async () => {
  const capabilities = await request({ action: "bridge.mcp.capabilities" });
  assert.equal(capabilities.ok, true);
  assert.equal(capabilities.protocol, 2);
  assert.ok(capabilities.actions.includes("install-client"));

  const install = await request({
    action: "bridge.tool.action", tool: "chatgpt-mcp", toolAction: "install-client"
  });
  assert.equal(install.ok, true);
  assert.equal(calls[0].url, "/v1/mcp/client/install");
  assert.equal(calls[0].options.method, "POST");

  const service = await request({
    action: "bridge.tool.action", tool: "chatgpt-mcp",
    toolAction: "service", serviceAction: "restart"
  });
  assert.equal(service.ok, true);
  assert.equal(calls[1].url, "/v1/mcp/service");
  assert.equal(JSON.parse(calls[1].options.body).action, "restart");

  const setup = await request({
    action: "bridge.tool.action", tool: "chatgpt-mcp",
    toolAction: "setup", tunnelId: "tunnel_0123456789abcdef0123456789abcdef"
  });
  assert.equal(setup.ok, true);
  assert.equal(calls[2].url, "/v1/mcp/setup");
  assert.equal(JSON.parse(calls[2].options.body).tunnelId, "tunnel_0123456789abcdef0123456789abcdef");
  console.log("MCP extension routing integration tests passed");
})().catch(error => { console.error(error); process.exitCode = 1; });
