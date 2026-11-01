"use strict";

importScripts("core/media.js", "core/store.js", "core/cookies.js", "core/launcher.js");

const requestMeta = new Map();

// This is a single private Native Messaging connection, independent from the
// regular one-shot ensure_bridge requests. It allows MCP to ask the extension
// to submit a media item without moving cookies or signed URLs into AI tools.
let browserCommandPort = null;
let browserCommandRetry = null;
const waitingNoticePageHashes = new Map();

async function showBrowserWaitNotice(command) {
  if (!/^[a-f0-9]{12}$/.test(command.resumeJobId || "") ||
      !/^[a-f0-9]{64}$/.test(command.pageHash || "") ||
      !command.expiresAt || Date.now() >= Date.parse(command.expiresAt)) return;
  const id = "vidotter-wait-" + command.resumeJobId;
  waitingNoticePageHashes.set(id, command.pageHash);
  await chrome.notifications.create(id, {
    type: "basic", iconUrl: "icons/icon-128.png",
    title: "VidOtter 正在等待媒体",
    message: "请在浏览器中完成必要操作并播放视频。检测到媒体后会自动继续下载。",
    priority: 0
  });
}

async function showBrowserJobResultNotice(command) {
  if (!/^[a-f0-9]{12}$/.test(command.resumeJobId || "") ||
      !["completed", "failed"].includes(command.jobStatus) ||
      !command.expiresAt || Date.now() >= Date.parse(command.expiresAt)) return;
  const complete = command.jobStatus === "completed";
  await chrome.notifications.create("vidotter-result-" + command.resumeJobId, {
    type: "basic", iconUrl: "icons/icon-128.png",
    title: complete ? "VidOtter 下载完成" : "VidOtter 下载失败",
    message: complete
      ? "任务已完成，文件已保存到下载目录。"
      : "任务未完成，请到下载任务列表查看错误。",
    priority: 0
  });
}

chrome.notifications.onClicked.addListener(notificationId => {
  const pageHash = waitingNoticePageHashes.get(notificationId);
  if (!pageHash) return;
  (async () => {
    for (const tab of await chrome.tabs.query({})) {
      if (!Number.isInteger(tab.id) || !/^https?:\/\//.test(String(tab.url || ""))) continue;
      const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(tab.url));
      const hash = [...new Uint8Array(digest)].map(value => value.toString(16).padStart(2, "0")).join("");
      if (hash !== pageHash) continue;
      await chrome.tabs.update(tab.id, { active: true });
      await chrome.windows.update(tab.windowId, { focused: true });
      return;
    }
  })().catch(() => {});
});

async function handleBrowserCommand(command) {
  if (!command) return;
  if (command.type === "browser_wait_notice") {
    try { await showBrowserWaitNotice(command); } catch (_) {}
    return;
  }
  if (command.type === "browser_job_result_notice") {
    try { await showBrowserJobResultNotice(command); } catch (_) {}
    return;
  }
  if (!["browser_media_download", "browser_media_refresh"].includes(command.type)) return;
  const reply = { commandId: command.commandId, ok: false };
  try {
    if (!command.expiresAt || Date.now() >= Date.parse(command.expiresAt)) {
      throw new Error("expired browser command");
    }
    if (command.type === "browser_media_refresh") {
      await syncRecentBrowserMedia(command.pageHash || "");
      reply.ok = true;
    } else {
      if (!Number.isInteger(command.tabId) || !command.resourceKey) {
        throw new Error("invalid command");
      }
      const [resources, tab] = await Promise.all([
        VidOtterStore.get(command.tabId),
        chrome.tabs.get(command.tabId)
      ]);
      const resource = resources.find(item => item.id === command.resourceKey);
      if (!resource || !/^https?:\/\//.test(String(tab.url || ""))) {
        throw new Error("resource no longer present");
      }
      const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(tab.url));
      const pageHash = [...new Uint8Array(digest)].map(value => value.toString(16).padStart(2, "0")).join("");
      if (pageHash !== command.pageHash) {
        throw new Error("browser tab navigated since capture");
      }
      const page = { url: tab.url, title: tab.title || "video", userAgent: navigator.userAgent };
      const result = await submitBridgeTask(resource, page, command.quality || "best",
        "single", command.subtitles, true, command.resumeJobId || "");
      if (command.resumeJobId) {
        await chrome.notifications.clear("vidotter-wait-" + command.resumeJobId).catch(() => {});
        waitingNoticePageHashes.delete("vidotter-wait-" + command.resumeJobId);
      }
      reply.ok = true;
      reply.taskId = result.taskId;
    }
  } catch (_) {
    // The Bridge's public MCP response never includes details from the
    // browser session or media URL; errors stay deliberately generic.
    reply.error = "cannot submit browser media";
  }
  try {
    await bridgeFetch("/v1/browser/commands/result", {
      method: "POST", body: JSON.stringify(reply)
    });
  } catch (_) {
    console.warn("[media-sniffer] browser command acknowledgement unavailable");
  }
}

function connectBrowserCommands() {
  if (browserCommandPort) return;
  try {
    const port = chrome.runtime.connectNative("com.vidotter.bridge");
    browserCommandPort = port;
    port.onMessage.addListener(message => {
      if (message?.type === "browser_subscribed" && message.ok) {
        syncRecentBrowserMedia().catch(() => {});
      }
      if (["browser_media_download", "browser_media_refresh", "browser_wait_notice", "browser_job_result_notice"].includes(message?.type)) {
        handleBrowserCommand(message).catch(() => {});
      }
    });
    port.onDisconnect.addListener(() => {
      if (browserCommandPort === port) browserCommandPort = null;
      if (browserCommandRetry) clearTimeout(browserCommandRetry);
      browserCommandRetry = setTimeout(() => {
        browserCommandRetry = null;
        connectBrowserCommands();
      }, 5000);
    });
    port.postMessage({ command: "subscribe_browser" });
  } catch (_) {
    browserCommandPort = null;
  }
}
connectBrowserCommands();
const tabQueues = new Map();
const DEFAULT_OPEN_MODE = "popup";
const POPUP_PATH = "popup/popup.html";
let currentOpenMode = DEFAULT_OPEN_MODE;
let currentMediaSnifferConfig = VidOtterMedia.normalizeSnifferConfig();

function hasEnabledMediaType(config) {
  return Boolean(config.detectHls || config.detectDash || config.detectMp4);
}

// Data/config.toml is the sole source of persisted preferences.
// Only per-tab media detections live in browser session storage.
const preferencesReady = bridgeFetch("/v1/preferences").then(result => result.preferences);

const mediaSnifferConfigReady = preferencesReady
  .then(preferences => {
    currentMediaSnifferConfig = VidOtterMedia.normalizeSnifferConfig(preferences);
    return currentMediaSnifferConfig;
  })
  .catch(error => {
    console.warn("[media-sniffer] TOML preferences unavailable", error);
    return currentMediaSnifferConfig;
  });

function normalizeOpenMode(value) {
  return value === "sidePanel" && chrome.sidePanel ? "sidePanel" : DEFAULT_OPEN_MODE;
}

async function applyOpenMode(value) {
  const mode = normalizeOpenMode(value);
  await chrome.action.setPopup({ popup: mode === "popup" ? POPUP_PATH : "" });
  if (chrome.sidePanel) {
    await chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: mode === "sidePanel" });
  }
  return mode;
}

const openModeReady = preferencesReady
  .then(preferences => applyOpenMode(preferences.openMode))
  .catch(error => {
    console.warn(error);
    return applyOpenMode(DEFAULT_OPEN_MODE);
  })
  .then(mode => { currentOpenMode = mode; return mode; });

function headersToContext(headers) {
  const context = {};
  const requestHeaders = {};
  for (const header of headers || []) {
    const name = String(header.name || "").toLowerCase();
    const value = String(header.value || "");
    if (name !== "cookie") requestHeaders[name] = value;
    if (name === "referer") context.referer = value;
    if (name === "origin") context.origin = value;
    if (name === "user-agent") context.userAgent = value;
  }
  context.requestHeaders = VidOtterMedia.sanitizeRequestHeaders(requestHeaders);
  return context;
}

let bridgeSession = null;

function requestNativeBridge() {
  return new Promise((resolve, reject) => {
    chrome.runtime.sendNativeMessage("com.vidotter.bridge", { command: "ensure_bridge" }, response => {
      const error = chrome.runtime.lastError;
      if (error) {
        reject(new Error(`无法连接 VidOtter Bridge：${error.message}`));
        return;
      }
      if (!response || !response.ok || !response.baseUrl || !response.token) {
        reject(new Error(response && response.error || "VidOtter Bridge 返回无效结果"));
        return;
      }
      resolve(response);
    });
  });
}

async function ensureBridge(force) {
  if (!force && bridgeSession && Date.now() - bridgeSession.checkedAt < 30000) return bridgeSession;
  const response = await requestNativeBridge();
  bridgeSession = { ...response, checkedAt: Date.now() };
  return bridgeSession;
}

async function bridgeFetch(path, options = {}, retry = true) {
  const bridge = await ensureBridge(false);
  const headers = {
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    "X-VidOtter-Token": bridge.token,
    ...(options.headers || {})
  };
  const response = await fetch(`${bridge.baseUrl}${path}`, { ...options, headers });
  if (response.status === 401 && retry) {
    bridgeSession = null;
    return bridgeFetch(path, options, false);
  }
  const result = await response.json().catch(() => ({}));
  if (!response.ok || result.ok === false) {
    throw new Error(result.error || `Bridge 请求失败：HTTP ${response.status}`);
  }
  return result;
}

function delay(milliseconds) {
  return new Promise(resolve => setTimeout(resolve, milliseconds));
}

async function restartBridge() {
  const previousPID = bridgeSession && bridgeSession.pid;
  await bridgeFetch("/v1/restart", { method: "POST" }, false);
  bridgeSession = null;
  await delay(300);
  const deadline = Date.now() + 10000;
  let lastError;
  while (Date.now() < deadline) {
    try {
      const session = await requestNativeBridge();
      if (!previousPID || session.pid !== previousPID) {
        bridgeSession = { ...session, checkedAt: Date.now() };
        return bridgeSession;
      }
    } catch (error) {
      lastError = error;
    }
    await delay(250);
  }
  throw new Error(lastError && lastError.message || "Bridge 重启超时");
}

async function buildBridgeTask(resource, page, quality, playlist, subtitles) {
  if (VidOtterMedia.mediaExpiring(resource)) {
    throw new Error("媒体链接已过期或即将过期，请刷新页面并重新播放后选择新媒体");
  }
  const resolveFromPage = resource.kind === "resolved-media-page" || resource.kind === "dash";
  const mediaURL = resolveFromPage ? String(page && page.url || resource.url) : resource.url;
  const pageURL = String(page && page.url || "");
  const cookieStoreId = await VidOtterCookies.storeIDForTab(chrome.cookies, resource.tabId);
  const [mediaCookies, pageCookies] = await Promise.all([
    VidOtterCookies.forURL(chrome.cookies, mediaURL, resource.tabId, resource.frameId, cookieStoreId),
    pageURL && pageURL !== mediaURL ? VidOtterCookies.forURL(chrome.cookies, pageURL, resource.tabId, 0, cookieStoreId) : Promise.resolve([])
  ]);
  return VidOtterLauncher.buildTask(
    resource,
    page,
    quality || "",
    mediaCookies,
    pageCookies.length ? pageCookies : (resolveFromPage ? mediaCookies : []),
    playlist || "ask",
    cookieStoreId,
    subtitles
  );
}

async function refreshedJobCredentials(job) {
  const sourceURL = String(job && job.sourceUrl || "");
  const pageURL = String(job && job.pageUrl || "");
  const cookieStoreId = String(job && job.cookieStoreId || "");
  const tabId = pageURL && Number.isInteger(job && job.sourceTabId) && job.sourceTabId >= 0
    ? job.sourceTabId
    : undefined;
  const frameId = Number.isInteger(job && job.sourceFrameId) && job.sourceFrameId >= 0 ? job.sourceFrameId : undefined;
  const [mediaCookies, pageCookies] = await Promise.all([
    VidOtterCookies.forURL(chrome.cookies, sourceURL, tabId, frameId, cookieStoreId),
    pageURL && pageURL !== sourceURL
      ? VidOtterCookies.forURL(chrome.cookies, pageURL, tabId, 0, cookieStoreId)
      : Promise.resolve([])
  ]);
  return {
    userAgent: navigator.userAgent,
    // An omitted cookie collection means that Chromium could not provide a
    // fresher value (for example, the source tab was closed). The Bridge then
    // keeps the still-memory-only credentials from the paused run.
    ...(mediaCookies.length ? { mediaCookies } : {}),
    ...(pageCookies.length ? { pageCookies } : {})
  };
}

async function probeBridgePlaylist(resource, page, quality) {
  const task = await buildBridgeTask(resource, page, quality, "single");
  return bridgeFetch("/v1/playlist/probe", {
    method: "POST",
    body: JSON.stringify(task)
  });
}

async function submitBridgeTask(resource, page, quality, playlist, subtitles, fromMCP = false, resumeJobId = "") {
  const task = await buildBridgeTask(resource, page, quality, playlist || "single", subtitles);
  if (fromMCP) task.viaMcp = true;
  if (resumeJobId) task.resumeJobId = resumeJobId;
  return bridgeFetch("/v1/tasks", {
    method: "POST",
    body: JSON.stringify(task)
  });
}

function mediaSnifferTool() {
  const config = VidOtterMedia.normalizeSnifferConfig(currentMediaSnifferConfig);
  const enabledLabels = [
    config.detectHls && "HLS",
    config.detectDash && "DASH",
    config.detectMp4 && "MP4"
  ].filter(Boolean);
  return {
    name: "media-sniffer",
    displayName: "媒体嗅探器",
    kind: "capability",
    description: "监听当前页面的媒体请求，并归并 HLS、DASH、MP4 与分离媒体轨。",
    platforms: ["chrome", "edge"],
    delivery: "extension",
    required: true,
    capabilities: ["media.detect", "media.group", "cookie.read-on-demand"],
    sortOrder: 30,
    config: {
      scope: "extension",
      schema: [
        { key: "detectHls", label: "HLS（m3u8）", type: "boolean", description: "检测 HLS 主清单和媒体清单。" },
        { key: "detectDash", label: "DASH / 分离媒体轨", type: "boolean", description: "检测 mpd、m4s 及页面级音视频轨。" },
        { key: "detectMp4", label: "MP4 直链", type: "boolean", description: "检测 mp4 URL 或 video/mp4 响应。" }
      ],
      values: config,
    },
    health: {
      status: "ready",
      ok: true,
      summary: `正在检测 ${enabledLabels.join("、")}`,
      detail: "仅在点击下载时读取匹配 URL 的 Cookie"
    }
  };
}

// Native Messaging is the transport used to reach the Bridge, not a second
// independently configurable runtime. Show one local-service status while
// retaining distinct transport and server failure details for diagnostics.
function offlineBridgeTool(layer, detail) {
  return {
    name: "bridge",
    displayName: "本地服务",
    kind: "runtime",
    description: "浏览器原生连接与 VidOtter 下载服务。",
    platforms: ["windows", "linux", "darwin"],
    delivery: "bundled",
    required: true,
    capabilities: ["bridge.launch", "task.submit", "job.manage", "config.persist"],
    sortOrder: 20,
    config: { schema: [], values: {} },
    health: {
      status: "error",
      ok: false,
      summary: "本地服务不可用",
      detail: `${layer}：${String(detail || "连接失败")}`
    }
  };
}

function localServiceTool(bridge) {
  const health = bridge.health || {};
  return {
    ...bridge,
    displayName: "本地服务",
    description: "浏览器原生连接与 VidOtter 下载服务。",
    health: {
      ...health,
      summary: health.ok ? "浏览器与下载服务连接正常" : (health.summary || "本地服务不可用"),
      detail: ["Native Messaging 已连接", health.detail].filter(Boolean).join(" · ")
    }
  };
}

async function bridgeToolsSnapshot() {
  await mediaSnifferConfigReady;
  try {
    await ensureBridge(false);
  } catch (error) {
    return {
      ok: true,
      version: 1,
      tools: [mediaSnifferTool(), offlineBridgeTool("浏览器原生连接失败", error.message || String(error))]
    };
  }

  const sniffer = mediaSnifferTool();
  try {
    const result = await bridgeFetch("/v1/tools");
    return {
      ...result,
      ok: true,
      tools: [sniffer, ...(result.tools || []).map(tool =>
        tool.name === "bridge" ? localServiceTool(tool) : tool)]
    };
  } catch (error) {
    return {
      ok: true,
      version: 1,
      tools: [sniffer, offlineBridgeTool("下载服务连接失败", error.message || String(error))]
    };
  }
}

function responseContentType(headers) {
  const found = (headers || []).find(header => String(header.name || "").toLowerCase() === "content-type");
  return found && found.value || "";
}

function enqueue(tabId, work) {
  const previous = tabQueues.get(tabId) || Promise.resolve();
  const current = previous.catch(() => {}).then(work);
  let queued;
  queued = current.finally(() => {
    if (tabQueues.get(tabId) === queued) tabQueues.delete(tabId);
  });
  tabQueues.set(tabId, queued);
  return queued;
}

async function record(input) {
  if (!Number.isInteger(input.tabId) || input.tabId < 0) return;
  await mediaSnifferConfigReady;
  const resource = VidOtterMedia.makeResource(input, currentMediaSnifferConfig);
  if (!resource) return;
  const items = await enqueue(resource.tabId, () => VidOtterStore.put(resource));
  const visibleItems = items.filter(item => VidOtterMedia.configAllowsKind(item.kind, currentMediaSnifferConfig));

  // Only nonsecret metadata is announced. Never send the resource URL,
  // authorization headers, or cookies to MCP/browser media listings.
  try {
    const tab = await chrome.tabs.get(resource.tabId);
    if (/^https?:\/\//.test(String(tab.url || ""))) {
      await bridgeFetch("/v1/browser/media", {
        method: "POST",
        body: JSON.stringify({
          resourceKey: resource.id,
          tabId: resource.tabId,
          kind: resource.kind,
          capturedAt: Number(resource.detectedAt || 0),
          expiresAt: Number(resource.expiresAt || 0),
          pageTitle: tab.title || "",
          pageUrl: tab.url
        })
      });
    }
  } catch (_) {
    // Sniffing still works without the optional MCP media index.
  }
  await chrome.action.setBadgeBackgroundColor({ tabId: resource.tabId, color: "#2563eb" });
  await chrome.action.setBadgeText({ tabId: resource.tabId, text: String(Math.min(visibleItems.length, 99)) });
}

// Recover the ephemeral Bridge media index after Bridge/native-host reconnects,
// and on demand from MCP. Never read cookies here; only local metadata leaves
// the extension, and the Bridge exposes only a redacted page path to MCP.
async function syncRecentBrowserMedia(pageHash = "") {
  await mediaSnifferConfigReady;
  const tabs = (await chrome.tabs.query({}))
    .filter(tab => Number.isInteger(tab.id) && /^https?:\/\//i.test(String(tab.url || "")))
    .sort((a, b) => (b.lastAccessed || 0) - (a.lastAccessed || 0));
  let announced = 0;
  const now = Date.now();
  for (const tab of tabs) {
    if (pageHash) {
      const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(tab.url));
      const candidateHash = [...new Uint8Array(digest)].map(value => value.toString(16).padStart(2, "0")).join("");
      if (candidateHash !== pageHash) continue;
    }
    const items = VidOtterMedia.recentResourcesForSync(
      await VidOtterStore.get(tab.id), currentMediaSnifferConfig, now);
    for (const resource of items) {
      if (announced >= 48) return announced;
      // A navigation can occur while the metadata scan is in flight.
      const current = await chrome.tabs.get(tab.id).catch(() => null);
      if (!current || current.url !== tab.url) break;
      await bridgeFetch("/v1/browser/media", {
        method: "POST",
        body: JSON.stringify({
          resourceKey: resource.id,
          tabId: tab.id,
          kind: resource.kind,
          capturedAt: Number(resource.detectedAt || 0),
          expiresAt: Number(resource.expiresAt || 0),
          pageTitle: tab.title || "",
          pageUrl: tab.url
        })
      });
      announced += 1;
    }
  }
  return announced;
}

async function refreshMediaBadges() {
  const tabs = await chrome.tabs.query({});
  await Promise.all(tabs.filter(tab => Number.isInteger(tab.id)).map(async tab => {
    try {
      const items = await VidOtterStore.get(tab.id);
      const visibleItems = items.filter(item => VidOtterMedia.configAllowsKind(item.kind, currentMediaSnifferConfig));
      await chrome.action.setBadgeText({
        tabId: tab.id,
        text: visibleItems.length ? String(Math.min(visibleItems.length, 99)) : ""
      });
    } catch (error) {
      console.warn(`[media-sniffer] 刷新标签页 ${tab.id} 计数失败`, error);
    }
  }));
}

chrome.webRequest.onBeforeRequest.addListener(details => {
  const current = {
    url: details.url,
    tabId: details.tabId,
    frameId: details.frameId,
    initiator: details.initiator || "",
    detectedAt: Date.now()
  };
  if (!VidOtterMedia.classify(current, currentMediaSnifferConfig)) return;
  requestMeta.set(details.requestId, current);
  record(current).catch(console.warn);
}, { urls: ["<all_urls>"] });

chrome.webRequest.onBeforeSendHeaders.addListener(details => {
  const current = requestMeta.get(details.requestId);
  if (!current) return;
  Object.assign(current, headersToContext(details.requestHeaders));
  requestMeta.set(details.requestId, current);
  record(current).catch(console.warn);
}, { urls: ["<all_urls>"] }, ["requestHeaders", "extraHeaders"]);

chrome.webRequest.onHeadersReceived.addListener(details => {
  const current = requestMeta.get(details.requestId) || {
    url: details.url,
    tabId: details.tabId,
    frameId: details.frameId,
    initiator: details.initiator || "",
    detectedAt: Date.now()
  };
  Object.assign(current, {
    contentType: responseContentType(details.responseHeaders),
    statusCode: details.statusCode
  });
  requestMeta.set(details.requestId, current);
  record(current).catch(console.warn);
}, { urls: ["<all_urls>"] }, ["responseHeaders", "extraHeaders"]);

function forget(details) { requestMeta.delete(details.requestId); }
chrome.webRequest.onCompleted.addListener(forget, { urls: ["<all_urls>"] });
chrome.webRequest.onErrorOccurred.addListener(forget, { urls: ["<all_urls>"] });

chrome.tabs.onRemoved.addListener(tabId => {
  VidOtterStore.clear(tabId).catch(() => {});
  tabQueues.delete(tabId);
});

chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
  if (!changeInfo.url) return;
  VidOtterStore.clear(tabId).catch(() => {});
  chrome.action.setBadgeText({ tabId, text: "" }).catch(() => {});
});

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (!message || !message.action) return;

  if (message.action === "ui.about") {
    const extensionVersion = chrome.runtime.getManifest().version;
    bridgeFetch("/v1/environment")
      .then(result => sendResponse({
        ok: true,
        extensionVersion,
        bridgeVersion: result.environment?.version || null
      }), () => sendResponse({ ok: true, extensionVersion, bridgeVersion: null }));
    return true;
  }

  if (message.action === "ui.language.get") {
    bridgeFetch("/v1/preferences")
      .then(result => sendResponse({ ok: true, locale: result.preferences.uiLanguage }),
        error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }
  if (message.action === "ui.language.set") {
    const locale = message.locale === "en" ? "en" : "zh-CN";
    bridgeFetch("/v1/preferences", { method: "PUT", body: JSON.stringify({ uiLanguage: locale }) })
      .then(() => sendResponse({ ok: true, locale }),
        error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }

  if (message.action === "ui.active-tab") {
    chrome.tabs.query({})
      .then(tabs => {
        const candidates = tabs.filter(tab => /^https?:\/\//.test(tab.url || "") &&
          !String(tab.url).startsWith("http://127.0.0.1:17891/console/"));
        const current = candidates.find(tab => tab.active && tab.lastFocusedWindow) ||
          candidates.find(tab => tab.active) || candidates[0];
        sendResponse(current ? { ok: true, tab: { id: current.id, title: current.title, url: current.url } } :
          { ok: false, error: "No browser media tab available" });
      }, error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }

  if (message.action === "ui.open-mode.get") {
    openModeReady
      .then(() => sendResponse({ ok: true, mode: currentOpenMode, supportsSidePanel: Boolean(chrome.sidePanel) }), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "ui.open-mode.set") {
    openModeReady
      .then(() => applyOpenMode(message.mode))
      .then(mode => bridgeFetch("/v1/preferences", {
        method: "PUT", body: JSON.stringify({ openMode: mode })
      }).then(() => mode))
      .then(mode => {
        currentOpenMode = mode;
        sendResponse({ ok: true, mode, supportsSidePanel: Boolean(chrome.sidePanel) });
      }, error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "ui.logs.get") {
    bridgeFetch("/v1/preferences")
      .then(result => sendResponse({ ok: true, showLogs: result.preferences.showLogs !== false }),
        error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }
  if (message.action === "ui.logs.set") {
    bridgeFetch("/v1/preferences", { method: "PUT", body: JSON.stringify({ showLogs: Boolean(message.showLogs) }) })
      .then(result => sendResponse({ ok: true, showLogs: result.preferences.showLogs }),
        error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }
  if (message.action === "bridge.logs.list") {
    const limit = Math.max(1, Math.min(Number(message.limit) || 1000, 1000));
    bridgeFetch("/v1/logs?limit=" + limit)
      .then(sendResponse, error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }
  if (message.action === "bridge.logs.clear") {
    bridgeFetch("/v1/logs", { method: "DELETE" })
      .then(sendResponse, error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }

  if (message.action === "media.detected") {
    const tabId = sender.tab && sender.tab.id;
    record({
      url: message.url,
      contentType: message.contentType || "",
      tabId,
      initiator: sender.origin || "",
      referer: sender.tab && sender.tab.url || "",
      userAgent: message.userAgent || "",
      detectedAt: Date.now()
    }).then(() => sendResponse({ ok: true }), error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }

  if (message.action === "media.list") {
    Promise.all([mediaSnifferConfigReady, VidOtterStore.get(message.tabId), chrome.tabs.get(message.tabId)])
      .then(([, items, tab]) => {
        const page = { url: tab.url || "", title: tab.title || "", userAgent: navigator.userAgent };
        sendResponse({ ok: true, page, items: VidOtterMedia.displayList(items, page, currentMediaSnifferConfig) });
      }, error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }

  if (message.action === "media.clear") {
    VidOtterStore.clear(message.tabId)
      .then(() => chrome.action.setBadgeText({ tabId: message.tabId, text: "" }))
      .then(() => sendResponse({ ok: true }), error => sendResponse({ ok: false, error: String(error) }));
    return true;
  }

  if (message.action === "media.download") {
    submitBridgeTask(message.resource, message.page, message.quality || "", message.playlist || "single", message.subtitles)
      .then(result => sendResponse({ ok: true, taskId: result.taskId }), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "media.playlist.probe") {
    probeBridgePlaylist(message.resource, message.page, message.quality || "")
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.environment") {
    Promise.all([bridgeFetch("/v1/environment"), bridgeFetch("/v1/config")])
      .then(([environment, config]) => sendResponse({
        ok: true,
        nativeHost: true,
        environment: environment.environment,
        config: config.config
      }), error => sendResponse({ ok: false, nativeHost: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.tools") {
    bridgeToolsSnapshot()
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.tool.action") {
    const tool = String(message.tool || "");
    const action = String(message.toolAction || "");
    if (tool === "bridge" && action === "restart") {
      restartBridge()
        .then(session => sendResponse({ ok: true, pid: session.pid }), error => sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    if (tool === "chatgpt-mcp" && action === "install-client") {
      bridgeFetch("/v1/mcp/client/install", { method: "POST" })
        .then(result => sendResponse(result), error =>
          sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    if (tool === "chatgpt-mcp" && action === "service") {
      const serviceAction = String(message.serviceAction || "");
      if (!["start", "stop", "restart"].includes(serviceAction)) {
        sendResponse({ ok: false, error: "不支持的 Tunnel 服务操作" });
        return;
      }
      bridgeFetch("/v1/mcp/service", {
        method: "POST",
        body: JSON.stringify({ action: serviceAction })
      }).then(result => sendResponse(result), error =>
        sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    if (tool === "chatgpt-mcp" && action === "setup") {
      bridgeFetch("/v1/mcp/setup", {
        method: "POST",
        body: JSON.stringify({ tunnelId: String(message.tunnelId || "") })
      })
        .then(result => sendResponse(result), error =>
          sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    if (tool === "browser-automation" &&
      ["install", "enable", "disable", "enable-interaction", "disable-interaction"].includes(action)) {
    bridgeFetch("/v1/browser/automation", {
      method: "POST",
      body: JSON.stringify({ action })
    }).then(result => sendResponse(result), error =>
      sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }
  if (tool === "go-downloader" && action === "open-output") {
      bridgeFetch("/v1/open-output", { method: "POST" })
        .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    if (tool === "whisper.cpp" && action === "install-model") {
      bridgeFetch("/v1/tools/whisper/models/install", {
        method: "POST",
        body: JSON.stringify({ model: String(message.model || "") })
      }).then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    if (tool === "llama.cpp" && action === "install-model") {
      bridgeFetch("/v1/tools/llama/models/install", {
        method: "POST",
        body: JSON.stringify({ model: String(message.model || ""), acceptLicense: Boolean(message.acceptLicense) })
      }).then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    if (tool !== "yt-dlp" || action !== "install") {
      sendResponse({ ok: false, error: "不支持的 Tool 操作" });
      return;
    }
    bridgeFetch("/v1/tools/yt-dlp/install", { method: "POST" })
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.config.save") {
    bridgeFetch("/v1/config", { method: "PUT", body: JSON.stringify(message.config || {}) })
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "tool.config.save") {
    const tool = String(message.tool || "");
    if (tool !== "media-sniffer") {
      bridgeFetch("/v1/config", { method: "PUT", body: JSON.stringify(message.config || {}) })
        .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
      return true;
    }
    const next = VidOtterMedia.normalizeSnifferConfig(message.config);
    if (!hasEnabledMediaType(next)) {
      sendResponse({ ok: false, error: "至少保留一种媒体检测类型" });
      return;
    }
    bridgeFetch("/v1/preferences", { method: "PUT", body: JSON.stringify(next) })
      .then(async () => {
        currentMediaSnifferConfig = next;
        requestMeta.clear();
        await refreshMediaBadges();
        sendResponse({ ok: true, config: next });
      }, error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.jobs.list") {
    bridgeFetch("/v1/jobs")
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.job.retry") {
    refreshedJobCredentials(message.job || {})
      .then(credentials => bridgeFetch(`/v1/jobs/${encodeURIComponent(message.taskId || "")}/retry`, {
        method: "POST",
        body: JSON.stringify(credentials)
      }))
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.job.resume") {
    refreshedJobCredentials(message.job || {})
      .then(credentials => bridgeFetch(`/v1/jobs/${encodeURIComponent(message.taskId || "")}/resume`, {
        method: "POST",
        body: JSON.stringify(credentials)
      }))
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (["pause", "open", "reveal", "delete"].includes(message.action.replace("bridge.job.", "")) && message.action.startsWith("bridge.job.")) {
    const action = message.action.slice("bridge.job.".length);
    const body = action === "delete"
      ? { deleteFiles: Boolean(message.deleteFiles) }
      : action === "open" || action === "reveal"
        ? { outputPath: String(message.outputPath || "") }
        : null;
    bridgeFetch(`/v1/jobs/${encodeURIComponent(message.taskId || "")}/${action}`, {
      method: "POST",
      ...(body ? { body: JSON.stringify(body) } : {})
    })
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.jobs.clear") {
    bridgeFetch("/v1/jobs/clear", { method: "POST" })
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }

  if (message.action === "bridge.output.open") {
    bridgeFetch("/v1/open-output", { method: "POST" })
      .then(result => sendResponse(result), error => sendResponse({ ok: false, error: error.message || String(error) }));
    return true;
  }
});
