"use strict";

const assert = require("assert");

global.VidOtterPopup = {};
const environment = require("../popup/environment.js");

assert.equal(environment.isOutdatedMcpWorkerError(new Error("不支持的 Tool 操作")), true);
assert.equal(environment.isOutdatedMcpWorkerError(new Error("unsupported tool action")), true);
assert.equal(environment.isOutdatedMcpWorkerError(new Error("Tunnel download HTTP 404")), false);
assert.deepEqual(environment.mcpStatus({config:{values:{configured:true,clientInstalled:true,tunnelId:"tunnel_test",ready:false}},health:{ok:false}}),
  { tunnelID:"tunnel_test", configured:true, clientInstalled:true, ready:false, revision:"", runtimeKeySaved:false, setupState:"", restartRequired:false, running:false });
assert.equal(environment.mcpStatus({config:{values:{}},health:{ok:true}}).ready, true);
assert.equal(environment.mcpStatus({}).configured, false);

// The on-screen Tunnel ID must win over a stale draft, e.g. after a browser
// autofill or status re-render that did not emit an input event.
const validTestID = "tunnel_9f0e1d2c3b4a59687766554433221100";
const formWithVisibleID = {
  querySelector(selector) {
    assert.equal(selector, 'input[name="tunnelId"]');
    return {value: "  " + validTestID + "  "};
  }
};
assert.equal(environment.mcpEffectiveTunnelID(formWithVisibleID,
  "tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "invalid-stale-draft", true), validTestID);
assert.equal(environment.mcpEffectiveTunnelID(formWithVisibleID,
  "tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "invalid-stale-draft", false),
  validTestID); // matches the visible valid ID even without an input event
assert.equal(environment.mcpEffectiveTunnelID(null, validTestID, "", false), validTestID);
assert.equal(environment.mcpEffectiveTunnelID({querySelector() { return {value: ""}; }},
  validTestID, validTestID, true), validTestID);
// The runtime key is not a website login field. Saving without editing it
// must not submit a password manager's automatic credentials.
const mcpSource = require("node:fs").readFileSync(require("node:path").join(__dirname, "../popup/environment.js"), "utf8");
assert.match(mcpSource, /input\.autocomplete = "off"/);
assert.match(mcpSource, /input\.readOnly = true/);
assert.match(mcpSource, /input\.addEventListener\("focus"/);
assert.match(mcpSource, /setTimeout\(scrub, 900\)/);
assert.match(mcpSource, /document\.activeElement !== input/);
assert.match(mcpSource, /mcpEffectiveTunnelID\(body, state\.tunnelID, mcpUI\.draftID, mcpUI\.idEdited\)/);
assert.match(mcpSource, /mcpVisibleFieldChanges\(shell, state, mcpUI\)/);
assert.match(mcpSource, /idInput\.placeholder = ""/);
assert.match(mcpSource, /idInput\.value = mcpUI\.draftID/);
assert.match(mcpSource, /input\.placeholder = ""/);
assert.doesNotMatch(mcpSource, /idInput\.value = mcpUI\.draftID \|\| state\.tunnelID/);
assert.doesNotMatch(mcpSource, /keyInput\.placeholder =/);


// A blank form must NOT unlock service controls merely because credentials
// happen to be persisted on disk. Save explicitly confirms using them.
const savedTunnel = {
  tunnelID: validTestID, revision: "rev-001", configured: true,
  runtimeKeySaved: true, running: false, ready: false
};
const confirmedForm = {
  confirmed: true, confirmedID: validTestID, confirmedRevision: "rev-001",
  draftID: "", draftKey: "", idEdited: false, busy: ""
};
const blankForm = { ...confirmedForm, confirmed: false };
const inactive = environment.mcpControlState(savedTunnel, blankForm);
assert.equal(inactive.credentialsSaved, true);
assert.equal(inactive.canStart, false);
assert.equal(inactive.canRefresh, false);
assert.equal(inactive.canAutoStart, false);
assert.equal(inactive.canStop, false);
assert.equal(environment.mcpIsExampleID("tunnel_0123456789abcdef0123456789abcdef"), true);
assert.equal(environment.mcpIsExampleID(validTestID), false);
const exampleStatus = {...savedTunnel, tunnelID:"tunnel_0123456789abcdef0123456789abcdef"};
const exampleForm = {...confirmedForm, confirmedID:exampleStatus.tunnelID};
const blocked = environment.mcpControlState(exampleStatus, exampleForm);
assert.equal(blocked.credentialsSaved, false);
assert.equal(blocked.canStart, false);
assert.equal(blocked.canAutoStart, false);
const confirmed = environment.mcpControlState(savedTunnel, confirmedForm);
assert.equal(confirmed.canStart, true);
// Regression: an ID auto-filled directly into the DOM (without input event)
// invalidates a previously confirmed form and cannot leave Start enabled.
const autoFilled = environment.mcpVisibleFieldChanges({
  querySelector(selector) {
    return selector === 'input[name="tunnelId"]'
      ? { value: "tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" } : null;
  }
}, savedTunnel, { ...confirmedForm, idEdited: false });
assert.equal(autoFilled.idEdited, true);
assert.equal(environment.mcpControlState(savedTunnel, {...confirmedForm, ...autoFilled}).canStart, false);
assert.equal(confirmed.canRefresh, true);
assert.equal(confirmed.canAutoStart, true);
assert.equal(confirmed.canStop, false);
for (const changed of [
  { draftID: "tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", idEdited: true },
  { draftKey: "new-key-entered" },
  { busy: "waiting" },
  { confirmedRevision: "previous-revision" },
  { confirmedID: "tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" }
]) {
  const result = environment.mcpControlState(savedTunnel, {...confirmedForm, ...changed});
  assert.equal(result.canStart, false, JSON.stringify(changed));
  assert.equal(result.canRefresh, false, JSON.stringify(changed));
}
const runningTunnel = environment.mcpControlState(
  {...savedTunnel, running: true}, blankForm);
assert.equal(runningTunnel.canStop, true);
assert.equal(runningTunnel.canRefresh, true);
assert.equal(runningTunnel.canStart, false);
assert.equal(environment.mcpControlState(
  {...savedTunnel, runtimeKeySaved: false}, confirmedForm).canStart, false);
assert.equal(environment.mcpControlState(
  {tunnelID:"", configured:false, runtimeKeySaved:false}, confirmedForm).canRefresh, false);
assert.equal(environment.mcpControlState(
  {...savedTunnel, running:true}, {...blankForm, busy:"stopping"}).canStop, false);

assert.equal(environment.cardState({ status: "ready", ok: true }), "ok");
assert.equal(environment.cardState({ status: "disabled", ok: true }), "disabled");
assert.equal(environment.cardState({ status: "missing", ok: false }), "error");
assert.equal(environment.cardState({ status: "incompatible", ok: false }), "error");
assert.equal(environment.cardState({ status: "unsupported", ok: false }), "checking");
assert.equal(environment.cardState(null), "checking");
assert.equal(environment.formatModelSize(147951465), "141 MiB");
assert.equal(environment.modelActionLabel({ active: true }), "当前模型");
assert.equal(environment.modelActionLabel({ installed: true }), "设为当前模型");
assert.equal(environment.modelActionLabel({}), "下载并使用");
assert.match(environment.modelOptionLabel({ id: "base", language: "多语言（含中文）", variant: "标准版", sizeBytes: 147951465, recommended: true }), /base.*141 MiB.*推荐/);
assert.equal(environment.normalizedKind({ kind: "infra" }), "runtime");
assert.equal(environment.normalizedKind({ kind: "core" }), "capability");
assert.equal(environment.normalizedKind({ kind: "dependency" }), "dependency");
assert.equal(environment.normalizedKind({ kind: "unknown" }), "other");
assert.equal(environment.visibleConfigFields({
  kind: "dependency", health: { ok: true }, config: { schema: [{ key: "path", type: "file" }] }
}, false).length, 0);
assert.equal(environment.visibleConfigFields({
  kind: "dependency", health: { ok: true }, config: { schema: [{ key: "path", type: "file" }] }
}, true).length, 1);
assert.equal(environment.visibleConfigFields({
  kind: "dependency", health: { ok: false }, config: { schema: [{ key: "path", type: "file" }] }
}, false).length, 1);
assert.equal(environment.visibleConfigFields({
  kind: "capability", health: { ok: true }, config: { schema: [{ key: "output", type: "directory" }] }
}, false).length, 1);
assert.equal(environment.editorValue({
  health: { path: "C:\\detected\\ffmpeg.exe" }, config: { values: { ffmpegPath: "C:\\configured\\ffmpeg.exe" } }
}, { key: "ffmpegPath", type: "file" }), "C:\\configured\\ffmpeg.exe");
assert.equal(environment.editorValue({
  health: { path: "C:\\detected\\ffmpeg.exe" }, config: { values: { ffmpegPath: "" } }
}, { key: "ffmpegPath", type: "file" }), "C:\\detected\\ffmpeg.exe");
assert.equal(environment.toggleMetadata({ config: { toggle: { key: "enabled", label: "启用" } } }).key, "enabled");
assert.equal(environment.toggleMetadata({ config: {} }), null);

const environmentSource = require("node:fs").readFileSync(require("node:path").join(__dirname, "../popup/environment.js"), "utf8");
assert.match(environmentSource, /navigation\.push\(\{ type: "edit", id: tool\.name \}\)/);
assert.match(environmentSource, /navigation\.pop\(\)/);
assert.match(environmentSource, /settings-heading|settingsHeading/);
assert.doesNotMatch(environmentSource, /defaultExpanded/);
assert.doesNotMatch(environmentSource, /tool\.name === "go-downloader"/);
assert.doesNotMatch(environmentSource, /dependencyNeedsAttention|hasRecoveryAction/);
assert.match(environmentSource, /function init\(\) {[\s\S]*?void refresh\(\);/);

const backgroundSource = require("node:fs").readFileSync(require("node:path").join(__dirname, "../background.js"), "utf8");




assert.match(backgroundSource, /name: "media-sniffer"/);
assert.match(backgroundSource, /const result = await bridgeFetch\("\/v1\/tools"\)/);
assert.match(backgroundSource, /function localServiceTool\(bridge\)/);
assert.match(backgroundSource, /displayName: "本地服务"/);
assert.doesNotMatch(backgroundSource, /function nativeHostTool\(/);
assert.doesNotMatch(backgroundSource, /name: "native-host"/);
assert.match(backgroundSource, /offlineBridgeTool\("浏览器原生连接失败"/);
assert.match(backgroundSource, /offlineBridgeTool\("下载服务连接失败"/);
assert.match(backgroundSource, /key: "detectHls"/);
assert.match(backgroundSource, /key: "detectDash"/);
assert.match(backgroundSource, /key: "detectMp4"/);
assert.doesNotMatch(backgroundSource, /MEDIA_SNIFFER_CONFIG_KEY|legacyToolSnapshot/);
assert.match(backgroundSource, /preferencesReady = bridgeFetch\("\/v1\/preferences"\)/);
assert.match(backgroundSource, /v1\/tools\/whisper\/models\/install/);
assert.match(backgroundSource, /bridge\.logs\.list/);
assert.match(backgroundSource, /bridge\.logs\.clear/);
assert.match(backgroundSource, /ui\.logs\.set/);
assert.match(backgroundSource, /message\.action === "ui\.about"/);
assert.match(backgroundSource, /chrome\.runtime\.getManifest\(\)\.version/);
assert.match(backgroundSource, /bridgeFetch\("\/v1\/environment"\)/);

assert.match(backgroundSource, /v1\/tools\/llama\/models\/install/);
assert.match(backgroundSource, /model: String\(message\.model/);
assert.match(environmentSource, /requiresLicenseAcceptance/);
assert.match(environmentSource, /acceptLicense: requiresAcceptance/);
assert.match(environmentSource, /field\.type === "boolean"/);
assert.match(environmentSource, /tool\.config\.scope === "extension"/);

console.log("extension environment tests passed");

// An already-ready old connection is not evidence that key rotation completed.
const oldConnection = {ready:true, tunnelID:"tunnel_test", revision:"before"};
assert.equal(environment.mcpSetupIsComplete(oldConnection, "tunnel_test", "before"), false);
assert.equal(environment.mcpSetupIsComplete({...oldConnection, revision:"after"}, "tunnel_test", "before"), true);
assert.equal(environment.mcpSetupIsComplete({...oldConnection, ready:false, revision:"after"}, "tunnel_test", "before"), false);
assert.equal(environment.mcpSetupIsComplete({...oldConnection, tunnelID:"different", revision:"after"}, "tunnel_test", "before"), false);

// ChatGPT MCP controls are single-purpose; do not expose Bridge login startup.
const mcpSection = environmentSource.split("function renderChatGPTMCP(body, tool) {")[1].split("function renderDetail(list, tool, editing) {")[0];
assert.match(mcpSection, /"Save"/);
assert.match(mcpSection, /"Start Service"/);
assert.match(mcpSection, /"Stop Service"/);
assert.match(mcpSection, /"Refresh Status"/);
assert.match(mcpSection, /target: "tunnel"/);
assert.doesNotMatch(mcpSection, /target: "bridge"|登录后后台运行 VidOtter/);
assert.doesNotMatch(mcpSection, /重新连接|更换连接配置|保存并验证/);
