"use strict";

function activateTab(name) {
  document.querySelectorAll("[data-tab]").forEach(tab => {
    const active = tab.dataset.tab === name;
    tab.classList.toggle("active", active);
    tab.setAttribute("aria-selected", active ? "true" : "false");
    tab.tabIndex = active ? 0 : -1;
  });
  document.querySelectorAll("[data-panel]").forEach(panel => {
    const active = panel.dataset.panel === name;
    panel.classList.toggle("active", active);
    panel.hidden = !active;
  });
  VidOtterDownloads.deactivate();
  VidOtterJobs.deactivate();
  VidOtterLogs.deactivate();
  if (name === "logs") return VidOtterLogs.activate();
	if (name === "environment") return VidOtterEnvironment.activate();
	if (name === "sniffer") return VidOtterDownloads.activate();
	if (name === "jobs") return VidOtterJobs.activate();
}

VidOtterPopup.activateTab = activateTab;

async function initOpenMode() {
  const select = document.getElementById("openMode");
  const hint = document.getElementById("openModeHint");
  try {
    const result = await VidOtterPopup.send("ui.open-mode.get");
    select.value = result.mode;
    if (!result.supportsSidePanel) {
      select.querySelector('[value="sidePanel"]').disabled = true;
      hint.textContent = "当前浏览器不支持侧边栏";
    }
  } catch (error) {
    select.disabled = true;
    hint.textContent = error.message || String(error);
  }
  select.addEventListener("change", async () => {
    select.disabled = true;
    hint.textContent = "正在保存…";
    try {
      const result = await VidOtterPopup.send("ui.open-mode.set", { mode: select.value });
      select.value = result.mode;
      hint.textContent = "已保存，下次点击扩展图标时生效";
    } catch (error) {
      hint.textContent = error.message || String(error);
    } finally {
      select.disabled = false;
    }
  });
}

function setLogsVisible(visible) {
  const tab = document.getElementById("logsTab");
  tab.hidden = !visible;
  document.getElementById("showLogsTab").checked = visible;
  if (!visible && document.querySelector(".tab.active") === tab) activateTab("environment");
}

async function refreshLogsVisibility() {
  try {
    const result = await VidOtterPopup.send("ui.logs.get");
    setLogsVisible(result.showLogs !== false);
  } catch (error) {
    VidOtterPopup.setMessage("environmentMessage", error.message || String(error), "error");
  }
}

function initLogsVisibility() {
  const control = document.getElementById("showLogsTab");
  control.addEventListener("change", async () => {
    const wanted = control.checked;
    control.disabled = true;
    try {
      const result = await VidOtterPopup.send("ui.logs.set", { showLogs: wanted });
      setLogsVisible(result.showLogs !== false);
      VidOtterPopup.setMessage("environmentMessage", "日志入口设置已保存。", "ok");
    } catch (error) {
      control.checked = !wanted;
      VidOtterPopup.setMessage("environmentMessage", error.message || String(error), "error");
    } finally {
      control.disabled = false;
    }
  });
  refreshLogsVisibility();
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) refreshLogsVisibility();
  });
}

document.addEventListener("DOMContentLoaded", () => {
  const open = document.getElementById("openWorkspace");
  if (VidOtterPopup.host === "web") open.hidden = true;
  open.addEventListener("click", async () => {
    try {
      await VidOtterPopup.send("bridge.environment");
      await chrome.tabs.create({ url: "http://127.0.0.1:17891/console/" });
    } catch (error) {
      VidOtterPopup.setMessage("environmentMessage", error.message || String(error), "error");
    }
  });
  document.getElementById("refreshWorkspace").addEventListener("click", () => activateTab(
    document.querySelector(".tab.active")?.dataset.tab || "sniffer"));
  VidOtterI18n.init();
  VidOtterEnvironment.init();
  VidOtterJobs.init();
  VidOtterLogs.init();
  initLogsVisibility();
  VidOtterDownloads.init();
  initOpenMode();
  const tabs = [...document.querySelectorAll("[data-tab]")];
  for (const tab of tabs) {
    tab.addEventListener("click", () => activateTab(tab.dataset.tab));
    tab.addEventListener("keydown", event => {
      if (!["ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
      event.preventDefault();
      const visible = tabs.filter(candidate => !candidate.hidden);
      const index = visible.indexOf(tab);
      const next = event.key === "Home" ? 0 :
        event.key === "End" ? visible.length - 1 :
          (index + (["ArrowRight", "ArrowDown"].includes(event.key) ? 1 : -1) + visible.length) % visible.length;
      visible[next].focus();
      activateTab(visible[next].dataset.tab);
    });
  }
  if (VidOtterPopup.host === "web") activateTab("jobs");
  else activateTab("sniffer");
});

window.addEventListener("unload", () => {
  VidOtterDownloads.deactivate();
  VidOtterJobs.deactivate();
  VidOtterLogs.deactivate();
});
