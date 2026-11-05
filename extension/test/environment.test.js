"use strict";

const assert = require("assert");

global.VidOtterPopup = {};
const environment = require("../popup/environment.js");

assert.equal(environment.isOutdatedMcpWorkerError(new Error("不支持的 Tool 操作")), true);
assert.equal(environment.isOutdatedMcpWorkerError(new Error("unsupported tool action")), true);
assert.equal(environment.isOutdatedMcpWorkerError(new Error("Tunnel download HTTP 404")), false);
assert.deepEqual(environment.mcpStatus({config:{values:{configured:true,clientInstalled:true,tunnelId:"tunnel_test",ready:false}},health:{ok:false}}),
  { tunnelID:"tunnel_test", configured:true, clientInstalled:true, ready:false, revision:"", runtimeKeySaved:false, setupState:"", restartRequired:false, running:false, autostartEnabled:null, tunnelAutoConnect:true, tunnelConnecting:false, serviceError:"" });
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
assert.match(mcpSource, /idInput\.value = mcpDisplayedTunnelID\(state, mcpUI\)/);
assert.match(mcpSource, /input\.placeholder = mcpSavedKeyMask\(keySaved, mcpUI\.draftKey\)/);
// Saved Tunnel ID is public in the form; Runtime Key must not be sent back
// to the page (the star mask is only a fixed placeholder, never a value).
assert.equal(environment.mcpDisplayedTunnelID({tunnelID:validTestID}, {idEdited:false, draftID:""}), validTestID);
assert.equal(environment.mcpDisplayedTunnelID({tunnelID:validTestID}, {idEdited:true, draftID:"tunnel_new"}), "tunnel_new");
assert.equal(environment.mcpDisplayedTunnelID({tunnelID:validTestID}, {idEdited:false, draftID:"stale-value"}), validTestID);
assert.equal(environment.mcpDisplayedTunnelID({tunnelID:""}, {idEdited:false, draftID:""}), "");
assert.equal(environment.mcpSavedKeyMask(true, ""), "****************");
assert.equal(environment.mcpSavedKeyMask(true, "new-secret"), "");
assert.equal(environment.mcpSavedKeyMask(false, ""), "");
assert.match(mcpSource, /input\.value = mcpUI\.draftKey/);
assert.doesNotMatch(mcpSource, /idInput\.value = mcpUI\.draftID \|\| state\.tunnelID/);
assert.doesNotMatch(mcpSource, /keyInput\.placeholder =/);


// Save persists the credentials, Start is available on opening whenever both
// persisted fields are valid. Unsaved edits disable Start, but never Stop.
const savedTunnel = {
  tunnelID: validTestID, revision:"rev-001",
  configured:true, runtimeKeySaved:true, running:false, ready:false
};
const form = {draftID:validTestID, draftKey:"", idEdited:false, busy:""};
const prepared = environment.mcpControlState(savedTunnel, form);
assert.equal(prepared.credentialsSaved, true);
assert.equal(prepared.canStart, true); // No redundant Save after reopening.
assert.equal(prepared.canAutoStart, true);
assert.equal(prepared.canStop, false);
assert.equal(prepared.canRestart, false);
assert.equal(environment.mcpControlState({...savedTunnel, running:true}, form).canRestart, true);
assert.equal(environment.mcpControlState({...savedTunnel, running:true},
  {...form, draftKey:"new-key"}).canRestart, false);
assert.equal(environment.mcpControlState({...savedTunnel, running:true, runtimeKeySaved:false},
  form).canRestart, false);
assert.equal(environment.mcpControlState({...savedTunnel, running:true},
  {...form, busy:"restarting"}).canRestart, false);

assert.deepEqual(environment.mcpHeaderStatus({...savedTunnel, running:false}, ""),
  {label:"Tunnel 已配置 · 未启动", kind:"pending"});
assert.deepEqual(environment.mcpHeaderStatus({...savedTunnel, ready:true, running:true}, ""),
  {label:"Tunnel 服务 Ready", kind:"ready"});
assert.deepEqual(environment.mcpHeaderStatus({...savedTunnel, running:true}, ""),
  {label:"Tunnel 运行中 · 尚未就绪", kind:"pending"});
assert.deepEqual(environment.mcpHeaderStatus({...savedTunnel, ready:true, running:true, restartRequired:true}, ""),
  {label:"配置已保存 · 待重启", kind:"pending"});
assert.deepEqual(environment.mcpHeaderStatus(savedTunnel, "restarting"),
  {label:"正在重启 Tunnel", kind:"pending"});
const noConfig = {tunnelID:"", configured:false, runtimeKeySaved:false};
assert.equal(environment.mcpControlState(noConfig, form).canStart, false);
assert.equal(environment.mcpControlState({...savedTunnel, runtimeKeySaved:false}, form).canStart, false);
assert.equal(environment.mcpControlState({...savedTunnel, tunnelID:"invalid-id"}, form).canStart, false);
assert.equal(environment.mcpIsExampleID("tunnel_0123456789abcdef0123456789abcdef"), true);
assert.equal(environment.mcpControlState(
  {...savedTunnel,tunnelID:"tunnel_0123456789abcdef0123456789abcdef"}, form).canStart,false);
for (const unsaved of [
  {draftID:"tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", idEdited:true},
  {draftID:"", idEdited:true},
  {draftKey:"replacement-runtime-key"},
  {busy:"saving"}
]) {
  const controls = environment.mcpControlState(savedTunnel, {...form,...unsaved});
  assert.equal(controls.canStart,false,JSON.stringify(unsaved));
  assert.equal(controls.canAutoStart,!unsaved.busy,JSON.stringify(unsaved));
}
const autoFilled = environment.mcpVisibleFieldChanges({
  querySelector(selector) {
    return selector === 'input[name="tunnelId"]'
      ? {value:"tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} : null;
  }
},savedTunnel,form);
assert.equal(autoFilled.idEdited,true);
assert.equal(environment.mcpControlState(savedTunnel,{...form,...autoFilled}).canStart,false);
const active = environment.mcpControlState({...savedTunnel,running:true},
  {...form, draftKey:"pending replacement"});
assert.equal(active.canStop,true); // Always stoppable despite dirty draft.
assert.equal(active.canStart,false);
assert.equal(environment.mcpControlState({...savedTunnel,running:true},
  {...form,busy:"stopping"}).canStop,false);
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

// ChatGPT MCP controls are single-purpose; do not expose Bridge login startup.
const mcpSection = environmentSource.split("function renderChatGPTMCP(body, tool) {")[1].split("function renderDetail(list, tool, editing) {")[0];
assert.match(mcpSection, /"Save"/);
assert.match(mcpSection, /"Start Service"/);
assert.match(mcpSection, /"Stop Service"/);
assert.match(mcpSection, /"Restart Service"/);
assert.match(mcpSection, /mcpAction\("restart", tool, shell\)/);
assert.match(environmentSource, /serviceAction: action/);
assert.match(mcpSection, /shell\.append\(localPanel, pluginPanel\)/);
assert.match(mcpSection, /mcp-local-service/);
assert.match(mcpSection, /mcp-chatgpt-plugin/);
assert.match(mcpSection, /"01", "本地 Tunnel 服务"/);
assert.match(mcpSection, /"02", "ChatGPT 插件"/);
assert.match(mcpSource, /mcp-key-reveal/);
assert.match(mcpSource, /reveal\.disabled = busy \|\| !input\.value/);
assert.match(mcpSource, /saved key never leaves the Bridge/i);
assert.doesNotMatch(mcpSection, /Local Tunnel:/);
assert.match(environmentSource, /mcp-overview-header/);
assert.match(environmentSource, /mcpHeaderStatus\(mcpStatus\(tool\), mcpUI\.busy\)/);
assert.doesNotMatch(mcpSection, /"Refresh Status"|refreshButton|mcpAction\("refresh"/);
assert.match(mcpSection, /mcpFieldHeading\(/);
assert.match(mcpSection, /mcpButton\(actions, "Save", .*true\)/);
assert.match(mcpSection, /\(\) => mcpAction\(running \? "stop" : "start", tool, shell\), true\)/);
assert.match(mcpSource, /setTimeout\(mcpPoll, 4000\)/);

assert.match(mcpSection, /Tunnel 自动连接/);
assert.match(mcpSection, /draftStartup/);
assert.doesNotMatch(mcpSection, /target: "bridge"|登录后后台运行 VidOtter/);
assert.doesNotMatch(mcpSection, /重新连接|更换连接配置|保存并验证/);

// Independent service and connection lifecycle: drafts never prevent Stop or Disconnect.
const standby = {...savedTunnel,running:true,tunnelConnecting:false,autostartEnabled:false,tunnelAutoConnect:false};
assert.equal(environment.mcpControlState(standby,form).canConnect,true);
assert.equal(environment.mcpControlState(standby,form).canDisconnect,false);
assert.equal(environment.mcpControlState({...standby,restartRequired:true},form).canConnect,false);
const preferenceDraft = {...form,draftAutoConnect:true};
assert.equal(environment.mcpControlState(standby,preferenceDraft).canRestart,false);
assert.equal(environment.mcpControlState(standby,preferenceDraft).canStop,true);
assert.equal(environment.mcpControlState({...standby,tunnelConnecting:true},preferenceDraft).canDisconnect,true);
assert.equal(environment.mcpControlState(savedTunnel,{...form,available:false}).canStart,false);
assert.deepEqual(environment.mcpHeaderStatus(standby),{label:"服务运行中 · Tunnel 未连接",kind:"pending"});

// The optional MCP tunnel has a standalone recovery action in the component UI.
assert.match(mcpSource, /if \(action === "install-client"\)/);
assert.match(mcpSource, /mcpButton\(localPanel, "安装官方 Tunnel Client"/);
assert.match(mcpSource, /toolAction: "install-client"/);
