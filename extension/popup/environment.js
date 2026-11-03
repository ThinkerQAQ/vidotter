(function (root) {
  "use strict";

  const api = root.VidOtterPopup;
  let tools = [];
  let background = null;
  let backgroundBusy = false;
  const navigation = [];

  function setIndicator(state, text) {
    const indicator = document.getElementById("bridgeIndicator");
    indicator.className = `indicator ${state}`;
    indicator.textContent = text;
  }

  function cardState(health) {
    if (health && health.status === "disabled") return "disabled";
    if (health && health.ok) return "ok";
    if (health && ["missing", "incompatible", "error"].includes(health.status)) return "error";
    return "checking";
  }

  function inputFor(field, value) {
    const input = document.createElement(field.type === "select" ? "select" : "input");
    if (field.type === "select") {
      for (const option of field.options || []) {
        const node = document.createElement("option");
        node.value = option.value ?? "";
        node.textContent = option.label || option.value || "";
        input.appendChild(node);
      }
    } else {
      input.type = field.type === "integer" ? "number" : field.type === "boolean" ? "checkbox" : "text";
      input.placeholder = field.placeholder || "";
      input.autocomplete = "off";
      input.readOnly = Boolean(field.readOnly);
      if (field.min !== undefined && field.min !== null) input.min = field.min;
      if (field.max !== undefined && field.max !== null) input.max = field.max;
    }
    if (field.type === "boolean") input.checked = Boolean(value ?? field.default);
    else input.value = value ?? field.default ?? "";
    if (!field.readOnly) input.dataset.configKey = field.key;
    return input;
  }

  function normalizedKind(tool) {
    const kind = String(tool && tool.kind || "").toLowerCase();
    if (kind === "runtime" || kind === "infra") return "runtime";
    if (kind === "capability" || kind === "core" || kind === "platform") return "capability";
    if (kind === "dependency") return "dependency";
    if (kind === "integration") return "dependency";
    return "other";
  }

  function visibleConfigFields(tool, editing) {
    const config = tool && tool.config || {};
    const schema = Array.isArray(config.schema) ? config.schema : [];
    if (editing) return schema.filter(field => !field.readOnly);
    if (normalizedKind(tool) !== "dependency" || !(tool.health && tool.health.ok)) return schema;
    return schema.filter(field => field.type !== "file");
  }

  function currentConfigValues(card, selectedTool) {
    const config = {};
    const extensionScope = Boolean(selectedTool && selectedTool.config && selectedTool.config.scope === "extension");
    for (const tool of tools) {
      const view = tool.config || {};
      if (extensionScope && tool.name !== selectedTool.name) continue;
      if (!extensionScope && view.scope === "extension") continue;
      const toggle = view.toggle;
      if (toggle && toggle.key) config[toggle.key] = Boolean(view.values && view.values[toggle.key]);
      for (const field of Array.isArray(view.schema) ? view.schema : []) {
        if (!field.readOnly) config[field.key] = (view.values && view.values[field.key]) ?? field.default ?? "";
      }
    }
    card.querySelectorAll("[data-config-key]").forEach(input => {
      config[input.dataset.configKey] = input.type === "checkbox"
        ? input.checked
        : input.type === "number" ? Number(input.value) : input.value.trim();
    });
    return config;
  }

  function saveToolConfig(tool, card) {
    const local = tool.config && tool.config.scope === "extension";
    return api.send("tool.config.save", {
      tool: tool.name,
      config: currentConfigValues(card, local ? tool : null)
    });
  }

  function toggleMetadata(tool) {
    const config = tool && tool.config || {};
    const toggle = config.toggle;
    return toggle && toggle.key ? toggle : null;
  }

  function renderToolToggle(summary, tool, card) {
    const toggle = toggleMetadata(tool);
    if (!toggle) return;
    const config = tool.config || {};
    const control = document.createElement("label");
    control.className = "tool-toggle";
    control.title = toggle.description || toggle.label || "切换 Tool";
    const input = document.createElement("input");
    input.type = "checkbox";
    input.checked = Boolean(config.values && config.values[toggle.key]);
    input.dataset.configKey = toggle.key;
    input.setAttribute("role", "switch");
    input.setAttribute("aria-label", toggle.label || `切换 ${tool.displayName || tool.name}`);
    const track = document.createElement("span");
    track.className = "tool-toggle-track";
    control.append(input, track);
    control.addEventListener("click", event => event.stopPropagation());
    input.addEventListener("change", async () => {
      input.disabled = true;
      try {
        await saveToolConfig(tool, card);
        api.setMessage("environmentMessage", `${tool.displayName || tool.name} 已${input.checked ? "开启" : "关闭"}。`, "ok");
        await refresh();
      } catch (error) {
        input.checked = !input.checked;
        input.disabled = false;
        api.setMessage("environmentMessage", error.message || String(error), "error");
      }
    });
    summary.appendChild(control);
  }

  function displayValue(field, value) {
    if (field.type === "boolean") return value === false ? "关闭" : "开启";
    if (field.type === "select") {
      const option = (field.options || []).find(item => String(item.value ?? "") === String(value ?? ""));
      return option && (option.label || option.value) || value || "未设置";
    }
    return value === undefined || value === null || value === "" ? "未设置" : String(value);
  }

  function formatModelSize(value) {
    const bytes = Number(value) || 0;
    if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(1)} GiB`;
    return `${Math.max(1, Math.round(bytes / 1024 ** 2))} MiB`;
  }

  function modelOptionLabel(model) {
    const state = model.active ? "当前" : model.installed ? "已下载" : model.recommended ? "推荐" : "";
    return [model.id, model.language, model.variant, formatModelSize(model.sizeBytes), state].filter(Boolean).join(" · ");
  }

  function modelActionLabel(model) {
    if (model.active) return "当前模型";
    return model.installed ? "设为当前模型" : "下载并使用";
  }

  function renderModelManager(body, tool) {
    const models = Array.isArray(tool.models) ? tool.models : [];
    if (!models.length) return;
    const manager = document.createElement("div");
    manager.className = "model-manager";
    const title = document.createElement("strong");
    const toolName = tool.displayName || tool.name || "本地";
    title.textContent = `${toolName} 模型目录`;
    const select = document.createElement("select");
    select.setAttribute("aria-label", "选择 Whisper 模型");
    for (const model of models) {
      const option = document.createElement("option");
      option.value = model.id;
      option.textContent = modelOptionLabel(model);
      select.appendChild(option);
    }
    const initial = models.find(model => model.active) || models.find(model => model.recommended) || models[0];
    select.value = initial.id;
    const detail = document.createElement("small");
    const licenseLink = document.createElement("a");
    licenseLink.target = "_blank";
    licenseLink.rel = "noreferrer";
    licenseLink.hidden = true;
    const button = document.createElement("button");
    button.type = "button";
    button.className = "primary";

    function selectedModel() {
      return models.find(model => model.id === select.value) || models[0];
    }
    function update() {
      const model = selectedModel();
      detail.textContent = `${model.source} · ${model.language} · ${model.variant} · ${formatModelSize(model.sizeBytes)}`;
      licenseLink.hidden = !model.licenseUrl;
      licenseLink.href = model.licenseUrl || "#";
      licenseLink.textContent = model.license ? `查看 ${model.license} 使用条款` : "查看模型许可";
      button.textContent = modelActionLabel(model);
      button.disabled = Boolean(model.active);
    }
    select.addEventListener("change", update);
    button.addEventListener("click", async () => {
      const model = selectedModel();
      const requiresAcceptance = !model.installed && Boolean(model.requiresLicenseAcceptance);
      const confirmation = requiresAcceptance
        ? `该模型受 ${model.license || "单独"} 使用条款约束。请先点击条款链接阅读。\n\n确认你已阅读并接受条款，并下载 ${model.id}（${formatModelSize(model.sizeBytes)}）？`
        : `下载 ${model.id}（${formatModelSize(model.sizeBytes)}）并设为当前模型？`;
      if (!model.installed && root.confirm && !root.confirm(confirmation)) return;
      button.disabled = true;
      button.textContent = model.installed ? "正在切换…" : "正在下载…";
      api.setMessage("environmentMessage", `正在准备 ${toolName} 模型 ${model.id}，请勿关闭扩展弹窗…`);
      try {
        await api.send("bridge.tool.action", { tool: tool.name, toolAction: "install-model", model: model.id, acceptLicense: requiresAcceptance });
        api.setMessage("environmentMessage", `${toolName} 模型 ${model.id} 已可用并设为当前模型。`, "ok");
        await refresh();
      } catch (error) {
        api.setMessage("environmentMessage", error.message || String(error), "error");
        button.disabled = false;
        update();
      }
    });
    manager.append(title, select, button, detail, licenseLink);
    body.appendChild(manager);
    update();
  }

  function editorValue(tool, field) {
    const config = tool && tool.config || {};
    const configured = config.values && config.values[field.key];
    if (configured !== undefined && configured !== null && configured !== "") return configured;
    if (field.type === "file") return tool && tool.health && tool.health.path || "";
    return configured ?? field.default ?? "";
  }

  function renderConfigView(body, tool) {
    const config = tool.config || {};
    const schema = visibleConfigFields(tool, false).filter(field => !field.readOnly);
    if (!schema.length) return;
    const values = document.createElement("div");
    values.className = "tool-values";
    for (const field of schema) {
      const item = document.createElement("div");
      item.className = `tool-value${field.type === "directory" || field.type === "file" ? " wide" : ""}`;
      const label = document.createElement("span");
      label.className = "tool-value-label";
      label.textContent = field.label || field.key;
      const data = document.createElement("span");
      data.className = "tool-value-data";
      data.textContent = displayValue(field, config.values && config.values[field.key]);
      data.title = data.textContent;
      item.append(label, data);
      values.appendChild(item);
    }
    body.appendChild(values);
  }

  function renderConfigEditor(body, tool) {
    const config = tool.config || {};
    const schema = visibleConfigFields(tool, true);
    if (!schema.length) return;
    const fields = document.createElement("div");
    fields.className = "tool-fields";
    for (const field of schema) {
      const label = document.createElement("label");
      label.className = `tool-field${field.type === "directory" || field.type === "file" ? " wide" : ""}${field.type === "boolean" ? " boolean" : ""}`;
      const input = inputFor(field, editorValue(tool, field));
      if (field.type === "boolean") label.append(input, document.createTextNode(field.label || field.key));
      else label.append(document.createTextNode(field.label || field.key), input);
      if (field.description) {
        const help = document.createElement("small");
        help.textContent = field.description;
        label.appendChild(help);
      }
      fields.appendChild(label);
    }
    body.appendChild(fields);
  }

  function renderEditControls(container, tool, card, editing) {
    const controls = document.createElement("div");
    controls.className = "tool-edit-controls";
    if (!editing) {
      const editButton = document.createElement("button");
      editButton.type = "button";
      editButton.className = "secondary tool-edit";
      editButton.textContent = "编辑";
      editButton.title = `编辑 ${tool.displayName || tool.name} 配置`;
      editButton.setAttribute("aria-label", editButton.title);
      editButton.addEventListener("click", () => {
        navigation.push({ type: "edit", id: tool.name });
        renderTools(tools);
      });
      controls.appendChild(editButton);
      container.appendChild(controls);
      return;
    }

    const saveButton = document.createElement("button");
    saveButton.type = "button";
    saveButton.className = "primary";
    saveButton.textContent = "保存";
    const cancelButton = document.createElement("button");
    cancelButton.type = "button";
    cancelButton.className = "secondary";
    cancelButton.textContent = "取消";
    cancelButton.addEventListener("click", () => {
      navigation.pop();
      renderTools(tools);
    });
    saveButton.addEventListener("click", async () => {
      saveButton.disabled = true;
      cancelButton.disabled = true;
      try {
        await saveToolConfig(tool, card);
          navigation.pop();
        api.setMessage("environmentMessage", `${tool.displayName || tool.name} 配置已保存。`, "ok");
        await refresh();
      } catch (error) {
        api.setMessage("environmentMessage", error.message || String(error), "error");
        saveButton.disabled = false;
        cancelButton.disabled = false;
      }
    });
    controls.append(cancelButton, saveButton);
    container.appendChild(controls);
  }

  const categoryMeta = {
    appearance: { title: "界面与交互", description: "语言、导航方式与工作台偏好", icon: "appearance" },
    runtime: { title: "连接与运行时", description: "浏览器连接、Bridge 与网络", icon: "runtime" },
    capability: { title: "下载能力", description: "媒体嗅探、下载与处理", icon: "capability" },
    dependency: { title: "外部组件", description: "ChatGPT、FFmpeg、yt-dlp、识别和翻译", icon: "dependency" },
    other: { title: "其他组件", description: "其他可选模块", icon: "settings" }
  };

  function symbol(name, className = "ui-icon") {
    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.classList.add(className);
    svg.setAttribute("aria-hidden", "true");
    const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
    use.setAttribute("href", "#i-" + name);
    svg.appendChild(use);
    return svg;
  }

  function settingsRow({ title, subtitle, icon, trailing, health, action }) {
    const row = document.createElement("button");
    row.type = "button";
    row.className = "settings-row";
    const symbolWrap = document.createElement("span");
    symbolWrap.className = "settings-row-symbol";
    symbolWrap.appendChild(symbol(icon || "settings"));
    const copy = document.createElement("span");
    copy.className = "settings-row-copy";
    const name = document.createElement("strong");
    name.textContent = title;
    const secondary = document.createElement("small");
    secondary.textContent = subtitle || "";
    copy.append(name, secondary);
    row.append(symbolWrap, copy);
    if (trailing) {
      const badge = document.createElement("span");
      badge.className = "settings-row-trailing" + (health ? " " + cardState(health) : "");
      badge.textContent = trailing;
      row.appendChild(badge);
    }
    row.appendChild(symbol("chevron", "settings-chevron"));
    row.addEventListener("click", action);
    return row;
  }

  function settingsHeading(title, description) {
    const back = document.getElementById("settingsBack");
    back.hidden = navigation.length === 0;
    document.getElementById("settingsTitle").textContent = title;
    document.getElementById("settingsSubtitle").textContent = description;
  }

  function enter(type, id) {
    navigation.push({ type, id });
    renderTools(tools);
  }

  function goBack() {
    if (!navigation.length) return;
    navigation.pop();
    renderTools(tools);
  }

  function addSection(list, label) {
    const section = document.createElement("h3");
    section.className = "settings-section-label";
    section.textContent = label;
    list.appendChild(section);
  }

  function toolActions(body, tool, health) {
    const actions = Array.isArray(tool.actions) ? tool.actions : [];
    if (!actions.length) return;
    const actionRow = document.createElement("div");
    actionRow.className = "tool-actions";
    for (const action of actions) {
      const button = document.createElement("button");
      button.type = "button";
      button.className = health.ok ? "secondary" : "primary";
      button.textContent = action.label || action.id;
      button.title = action.description || "";
      button.addEventListener("click", async () => {
        if (tool.name === "browser-automation" && action.id === "enable-interaction") {
          // A separate, local user decision is mandatory. MCP cannot enable
          // interactive browser control on its own.
          const accepted = window.confirm(
            "启用浏览器自动点击？\n\n" +
            "AI 将只能操作自身新建的后台标签页，但页面操作仍可能触发表单、跳转或其他副作用。\n" +
            "不会开放已有标签页、任意 JavaScript、Cookie 或浏览器存储。\n\n" +
            "你可以随时在这里停用。"
          );
          if (!accepted) return;
        }
        button.disabled = true;
        api.setMessage("environmentMessage", "正在执行 " + (tool.displayName || tool.name) + "：" + (action.label || action.id) + "…");
        try {
          await api.send("bridge.tool.action", { tool: tool.name, toolAction: action.id });
          const success = action.id === "restart" ? "Bridge 已重启并重新连接。"
            : action.id === "open-output" ? "已打开下载目录。"
            : tool.name === "browser-automation" ? ("浏览器自动化：" + (action.label || action.id) + "已完成。")
            : (tool.displayName || tool.name) + " 已安装。";
          await refresh();
          api.setMessage("environmentMessage", success, "ok");
        } catch (error) {
          api.setMessage("environmentMessage", error.message || String(error), "error");
        } finally {
          button.disabled = false;
        }
      });
      actionRow.appendChild(button);
    }
    body.appendChild(actionRow);
  }

  // Credentials are submitted together; the Key stays only in transient form state
  // until the local service saves it in private OS storage.
  const mcpUI = { draftID: "", idEdited: false, draftKey: "", confirmed: false, confirmedID: "", confirmedRevision: "", busy: "", error: "", info: "", deadline: 0, timer: null, polling: false, advanced: false, workerOutdated: false,
    editing: false, pendingID: "", pendingSetup: false, pendingRevision: "" };
  const mcpIDPattern = /^tunnel_(?:[a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$/;
  // Both fields start blank. Only explicit user input may replace a saved ID:
  // password managers can populate visible inputs without the user's intent.
  function mcpEffectiveTunnelID(form, savedID, draftID, edited = true) {
    const field = form?.querySelector?.('input[name="tunnelId"]');
    const entered = String(field ? field.value : (draftID || "")).trim();
    return edited && entered ? entered : String(savedID || "").trim();
  }
  // Service controls require an explicit Save/confirmation in this view.
  // Empty inputs mean "reuse stored credentials" but NOT "ready to start".
  // A running Tunnel can always be stopped even with an unconfirmed form.
  function mcpControlState(state, form = {}) {
    const running = Boolean(state.running || state.ready);
    const changedID = Boolean(form.idEdited && String(form.draftID || "").trim() &&
      String(form.draftID).trim() !== state.tunnelID);
    const hasChanges = changedID || Boolean(String(form.draftKey || "").trim());
    const credentialsSaved = Boolean(state.configured && state.runtimeKeySaved &&
      mcpIDPattern.test(String(state.tunnelID || "")));
    const confirmedCurrent = Boolean(form.confirmed && form.confirmedID === state.tunnelID &&
      form.confirmedRevision === (state.revision || ""));
    const usable = credentialsSaved && confirmedCurrent && !hasChanges && !form.busy;
    return {
      running, hasChanges, credentialsSaved,
      canStart: !running && usable,
      canStop: running && !form.busy,
      canRefresh: !form.busy && (running || usable),
      canAutoStart: !form.busy && (running || usable)
    };
  }

  const chatGPTSettingsURL = "https://chatgpt.com/settings/plugins-settings";

  function isOutdatedMcpWorkerError(error) {
    return /不支持的 Tool 操作|不支持的 Tool|unsupported tool (?:action|operation)|unsupported action/i
      .test(String(error && error.message || error || ""));
  }

  function showMcpWorkerReload(form, message) {
    message.textContent = "VidOtter 扩展后台仍是旧版。请重新加载扩展，再执行 MCP 连接。";
    message.classList.add("mcp-reload-needed");
    if (form.querySelector("[data-mcp-worker-reload]")) return;
    if (api.host === "extension" && root.chrome?.runtime?.reload) {
      const reload = document.createElement("button");
      reload.type = "button";
      reload.className = "secondary";
      reload.dataset.mcpWorkerReload = "1";
      reload.textContent = "重新加载 VidOtter 扩展";
      reload.addEventListener("click", () => root.chrome.runtime.reload());
      form.appendChild(reload);
    } else {
      const instruction = document.createElement("p");
      instruction.dataset.mcpWorkerReload = "1";
      instruction.textContent = "在 edge://extensions 或 chrome://extensions 中重新加载 VidOtter，然后刷新本页面。";
      form.appendChild(instruction);
    }
  }

  function mcpIsVisible() {
    const frame = navigation[navigation.length - 1];
    const panel = document.querySelector('[data-panel="environment"]');
    return frame?.type === "tool" && frame.id === "chatgpt-mcp" &&
      (!panel || panel.classList.contains("active")) && !document.hidden;
  }

  function scheduleMcpPoll() {
    clearTimeout(mcpUI.timer);
    mcpUI.timer = null;
    if (mcpIsVisible()) mcpUI.timer = setTimeout(mcpPoll, 4000);
  }

  function mcpStatus(tool) {
    const values = tool?.config?.values || {};
    const status = tool?.health || {};
    return {
      tunnelID: values.tunnelId || "",
      configured: Boolean(values.configured),
      clientInstalled: Boolean(values.clientInstalled),
      ready: Boolean(values.ready || status.ok),
      revision: values.revision || "", runtimeKeySaved: Boolean(values.runtimeKeySaved), setupState: values.setupState || "",
      restartRequired: Boolean(values.restartRequired), running: Boolean(values.running)
    };
  }

  function mcpSetupIsComplete(status, tunnelID, previousRevision) {
    return status.ready && status.tunnelID === tunnelID &&
      Boolean(status.revision) && status.revision !== previousRevision;
  }

  async function mcpPoll() {
    if (!mcpIsVisible() || mcpUI.polling) return;
    mcpUI.polling = true;
    try {
      const response = await api.send("bridge.tools");
      const next = Array.isArray(response.tools) ? response.tools : [];
      const previous = tools.find(item => item.name === "chatgpt-mcp");
      const current = next.find(item => item.name === "chatgpt-mcp");
      const status = mcpStatus(current);
      // Avoid replacing the Tunnel ID input every four seconds while typing.
      let changed = JSON.stringify([mcpStatus(previous), previous?.health?.status]) !==
        JSON.stringify([status, current?.health?.status]);
      if (mcpUI.busy === "waiting" &&
          (!mcpUI.pendingID || status.tunnelID === mcpUI.pendingID) &&
          (!mcpUI.pendingSetup || (status.configured && status.revision && status.revision !== mcpUI.pendingRevision))) {
        mcpUI.busy = "";
        mcpUI.error = "";
        mcpUI.pendingID = "";
        mcpUI.draftID = "";
        mcpUI.idEdited = false;
        mcpUI.confirmed = Boolean(status.configured && status.runtimeKeySaved);
        mcpUI.confirmedID = mcpUI.confirmed ? status.tunnelID : "";
        mcpUI.confirmedRevision = mcpUI.confirmed ? (status.revision || "") : "";
        mcpUI.info = "配置已保存，可启动服务。";
        changed = true;
      } else if (mcpUI.busy === "waiting" && mcpUI.pendingSetup && status.setupState === "failed") {
        mcpUI.busy = "";
        mcpUI.pendingID = "";
        mcpUI.error = "连接验证失败，原配置已保留。请检查 Tunnel ID、Key 和 Tunnels Read + Use 权限后重试。";
        mcpUI.editing = status.configured;
        changed = true;
      } else if (mcpUI.busy === "waiting" && Date.now() > mcpUI.deadline) {
        mcpUI.busy = "";
        mcpUI.error = mcpUI.pendingID && status.tunnelID !== mcpUI.pendingID
          ? "新 Tunnel 未能就绪，当前仍保留原连接。请检查 Tunnel ID 和 Key 后重试。"
          : "连接尚未就绪。请检查 Tunnel ID 和 Key 后重试。";
        mcpUI.pendingID = "";
        changed = true;
      }
      if (changed) renderTools(next);
      else scheduleMcpPoll();
    } catch (error) {
      mcpUI.error = "无法读取 MCP 连接状态：" + (error.message || String(error));
      renderTools(tools);
      // Keep the next retry automatic even when the Bridge temporarily fails.
      scheduleMcpPoll();
    } finally {
      mcpUI.polling = false;
    }
  }

  async function mcpAction(action, tool, body) {
    if (mcpUI.busy) return;
    const state = mcpStatus(tool);
    mcpUI.error = "";
    mcpUI.info = "";
    if (action === "refresh") {
      if (!mcpControlState(state, mcpUI).canRefresh) return;
      await refresh();
      return;
    }
    if (action === "save") {
      const tunnelId = mcpEffectiveTunnelID(body, state.tunnelID, mcpUI.draftID, mcpUI.idEdited);
      const runtimeApiKey = mcpUI.draftKey.trim();
      const reuseKey = !runtimeApiKey && state.runtimeKeySaved;
      if (state.configured && state.runtimeKeySaved &&
          tunnelId === state.tunnelID && !runtimeApiKey) {
        // Explicitly approve using the existing saved credentials. No backend
        // write, service restart, or login-task modification is performed.
        mcpUI.confirmed = true;
        mcpUI.confirmedID = state.tunnelID;
        mcpUI.confirmedRevision = state.revision || "";
        mcpUI.draftID = "";
        mcpUI.idEdited = false;
        mcpUI.info = "已确认复用已保存的配置，现在可以启动服务。";
        renderTools(tools);
        return;
      }
      if (!mcpIDPattern.test(tunnelId)) {
        mcpUI.error = "请输入有效的 Tunnel ID。";
        renderTools(tools);
        return;
      }
      if (!reuseKey && (runtimeApiKey.length < 16 || runtimeApiKey.length > 512 ||
          /[\r\n\x00]/.test(runtimeApiKey))) {
        mcpUI.error = "请输入 Runtime API Key，或保留已保存的密钥。";
        renderTools(tools);
        return;
      }
      mcpUI.confirmed = false;
      mcpUI.confirmedID = "";
      mcpUI.confirmedRevision = "";
      mcpUI.busy = "saving";
      renderTools(tools);
      try {
        const capability = await api.send("bridge.mcp.capabilities");
        if (capability.protocol < 5 || !["save-only", "reuse-saved-key", "inline-credentials"]
          .every(id => capability.actions?.includes(id))) {
          throw new Error("unsupported tool action: old extension worker");
        }
        if (!state.clientInstalled) {
          if (root.confirm && !root.confirm("安装 OpenAI 官方 Tunnel Client？将校验发布包；不会启动远程服务。")) {
            mcpUI.busy = "";
            renderTools(tools);
            return;
          }
          await api.send("bridge.tool.action", {
            tool: "chatgpt-mcp", toolAction: "install-client"
          });
        }
        await api.send("bridge.tool.action", {
          tool: "chatgpt-mcp", toolAction: "setup",
          tunnelId, runtimeApiKey, reuseKey, saveOnly: true,
          replace: state.configured, previousTunnelId: state.configured ? state.tunnelID : ""
        });
        mcpUI.draftKey = "";
        mcpUI.busy = "waiting";
        mcpUI.pendingSetup = true;
        mcpUI.pendingID = tunnelId;
        mcpUI.pendingRevision = state.revision;
        mcpUI.deadline = Date.now() + 120000;
        await refresh();
      } catch (error) {
        mcpUI.busy = "";
        mcpUI.error = error.message || String(error);
        if (isOutdatedMcpWorkerError(error)) mcpUI.workerOutdated = true;
        renderTools(tools);
      }
      return;
    }
    if (action === "start" || action === "stop") {
      const controls = mcpControlState(state, mcpUI);
      if (action === "start" ? !controls.canStart : !controls.canStop) return;
      mcpUI.busy = action === "start" ? "starting" : "stopping";
      renderTools(tools);
      try {
        await api.send("bridge.tool.action", {
          tool: "chatgpt-mcp", toolAction: "service", serviceAction: action
        });
        mcpUI.busy = "";
        await refresh();
      } catch (error) {
        mcpUI.busy = "";
        mcpUI.error = error.message || String(error);
        renderTools(tools);
      }
    }
  }

  function mcpButton(parent, label, action, primary = false) {
    const control = document.createElement("button");
    control.type = "button";
    control.className = primary ? "primary" : "secondary";
    control.textContent = label;
    control.disabled = Boolean(mcpUI.busy);
    control.addEventListener("click", action);
    parent.appendChild(control);
    return control;
  }

  function mcpKeyField(busy, onEdit = () => {}) {
    const label = document.createElement("label");
    label.className = "mcp-id-field";
    label.textContent = "Runtime API Key";
    const input = document.createElement("input");
    input.name = "runtimeApiKey";
    input.type = "password";
    // This is a runtime API key, not a website login password. Discourage
    // browser password managers from injecting an unrelated saved password.
    input.autocomplete = "new-password";
    input.setAttribute("data-lpignore", "true");
    input.setAttribute("data-1p-ignore", "true");
    input.spellcheck = false;
    input.maxLength = 512;
    input.placeholder = "";
    input.value = mcpUI.draftKey;
    input.disabled = busy;
    input.addEventListener("input", () => {
      // Browser autofill can dispatch an input event for an unrelated saved
      // website password. Do not treat an unfocused autofill as key rotation.
      if (document.activeElement !== input) {
        input.value = mcpUI.draftKey;
        return;
      }
      mcpUI.draftKey = input.value;
      // Any manual edit invalidates the session's previous Save confirmation.
      mcpUI.confirmed = false;
      mcpUI.confirmedID = "";
      mcpUI.confirmedRevision = "";
      onEdit();
    });
    label.appendChild(input);
    return label;
  }

  function renderChatGPTMCP(body, tool) {
    const state = mcpStatus(tool);
    const busy = Boolean(mcpUI.busy);
    const shell = document.createElement("section");
    shell.className = "mcp-guide mcp-connection-card";
    const title = document.createElement("h3");
    title.textContent = "ChatGPT MCP";
    shell.appendChild(title);
    const helpLinks = document.createElement("div");
    helpLinks.className = "mcp-help-links";
    for (const [label, url] of [
      ["1. 创建 / 管理 Tunnel ID", "https://platform.openai.com/settings/organization/tunnels"],
      ["2. 创建 Runtime API Key", "https://platform.openai.com/api-keys"]
    ]) {
      const link = document.createElement("a");
      link.href = url;
      link.target = "_blank";
      link.rel = "noopener noreferrer";
      link.textContent = label;
      helpLinks.appendChild(link);
    }
    shell.appendChild(helpLinks);
    let updateControls = () => {};
    const idLabel = document.createElement("label");
    idLabel.className = "mcp-id-field";
    idLabel.textContent = "Tunnel ID";
    const idInput = document.createElement("input");
    idInput.name = "tunnelId";
    idInput.type = "text";
    idInput.autocomplete = "off";
    idInput.spellcheck = false;
    idInput.maxLength = 80;
    idInput.placeholder = "";
    idInput.value = mcpUI.draftID;
    idInput.disabled = busy;
    idInput.addEventListener("input", () => {
      // Ignore silent form autofill; a focused edit is required to replace
      // the saved ID.
      if (document.activeElement !== idInput) {
        idInput.value = mcpUI.draftID;
        return;
      }
      mcpUI.draftID = idInput.value;
      mcpUI.idEdited = true;
      // Even if the user clears this field again, require Save to reconfirm.
      mcpUI.confirmed = false;
      mcpUI.confirmedID = "";
      mcpUI.confirmedRevision = "";
      updateControls();
    });
    idLabel.appendChild(idInput);
    shell.appendChild(idLabel);
    const idState = document.createElement("small");
    idState.className = "settings-help mcp-credential-state";
    idState.textContent = state.configured ? "已保存；留空则复用" : "尚未保存";
    shell.appendChild(idState);
    const key = mcpKeyField(busy, () => updateControls());
    shell.appendChild(key);
    const keyState = document.createElement("small");
    keyState.className = "settings-help mcp-credential-state";
    keyState.textContent = state.runtimeKeySaved ? "已保存；留空则复用" : "尚未保存";
    shell.appendChild(keyState);
    const actions = document.createElement("div");
    actions.className = "mcp-service-actions";
    mcpButton(actions, "Save", () => mcpAction("save", tool, shell), true);
    const running = Boolean(state.running || state.ready);
    const service = mcpButton(actions,
      running ? "Stop Service" : "Start Service",
      () => mcpAction(running ? "stop" : "start", tool, shell));
    const refreshButton = mcpButton(actions, "Refresh Status",
      () => mcpAction("refresh", tool, shell));
    shell.appendChild(actions);
    let autoStartCheckbox = null;
    updateControls = () => {
      const controls = mcpControlState(state, mcpUI);
      service.disabled = !(running ? controls.canStop : controls.canStart);
      refreshButton.disabled = !controls.canRefresh;
      service.title = running ? "停止正在运行的 MCP Tunnel" :
        controls.canStart ? "启动已确认的 MCP 配置" : "请先点击 Save 保存或确认配置";
      refreshButton.title = controls.canRefresh ? "刷新 Tunnel 状态" : "请先保存配置或启动服务";
      if (autoStartCheckbox) autoStartCheckbox.disabled = backgroundBusy || !controls.canAutoStart;
      const notice = shell.querySelector(".mcp-info");
      if (notice && controls.hasChanges) notice.remove();
    };
    if (background) {
      const toggleLabel = document.createElement("label");
      toggleLabel.className = "settings-help";
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.checked = Boolean(background.tunnelEnabled);
      autoStartCheckbox = checkbox;
      checkbox.addEventListener("change", async () => {
        backgroundBusy = true;
        try {
          background = await api.send("bridge.tool.action", {
            tool: "background", toolAction: checkbox.checked ? "enable" : "disable", target: "tunnel"
          });
          await refresh();
        } catch (error) {
          mcpUI.error = error.message || String(error);
        } finally {
          backgroundBusy = false;
          renderTools(tools);
        }
      });
      toggleLabel.append(checkbox, document.createTextNode(" Auto-start MCP at login"));
      shell.appendChild(toggleLabel);
    }
    updateControls();
    if (!running && state.configured && state.runtimeKeySaved &&
        !(mcpUI.confirmed && mcpUI.confirmedID === state.tunnelID &&
          mcpUI.confirmedRevision === (state.revision || ""))) {
      const confirmHint = document.createElement("p");
      confirmHint.className = "settings-help mcp-confirmation-hint";
      confirmHint.textContent = "凭据已保存。请点击 Save 确认使用现有配置，之后才能启动或刷新状态。";
      shell.appendChild(confirmHint);
    }
    const status = document.createElement("p");
    status.className = "mcp-description";
    status.setAttribute("role", "status");
    status.textContent = mcpUI.busy === "saving" ? "正在保存配置…" :
      mcpUI.busy === "waiting" ? "正在验证并保存配置…" :
      mcpUI.busy === "starting" ? "正在启动 Tunnel…" :
      mcpUI.busy === "stopping" ? "正在停止 Tunnel…" :
      state.ready ? "Local Tunnel: Connected" :
      running ? "Local Tunnel: Starting / Not ready" :
      state.configured ? "Local Tunnel: Configured, stopped" : "Local Tunnel: Not configured";
    shell.appendChild(status);
    if (state.restartRequired && running) {
      const pending = document.createElement("p");
      pending.className = "settings-help";
      pending.textContent = "配置已保存，当前服务仍使用旧配置。请 Stop Service 后再 Start Service。";
      shell.appendChild(pending);
    }
    const plugin = document.createElement("p");
    plugin.className = "settings-help";
    plugin.textContent = "ChatGPT Plugin: 尚未在本机验证（需在 ChatGPT 中完成授权）";
    shell.appendChild(plugin);
    const link = document.createElement("a");
    link.href = chatGPTSettingsURL;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = "Open ChatGPT Plugins";
    shell.appendChild(link);
    if (mcpUI.info) {
      const message = document.createElement("p");
      message.className = "settings-help mcp-info";
      message.setAttribute("role", "status");
      message.textContent = mcpUI.info;
      shell.appendChild(message);
    }
    if (mcpUI.error) {
      const message = document.createElement("p");
      message.className = "mcp-error";
      message.setAttribute("role", "alert");
      message.textContent = mcpUI.error;
      shell.appendChild(message);
    }
    if (mcpUI.workerOutdated) showMcpWorkerReload(shell, shell.querySelector(".mcp-error") || status);
    body.appendChild(shell);
    scheduleMcpPoll();
  }

  function renderDetail(list, tool, editing) {
    const health = tool.health || {};
    const card = document.createElement("section");
    card.className = "settings-detail-card";
    card.dataset.toolName = tool.name || "";
    const header = document.createElement("div");
    header.className = "settings-tool-header";
    const status = document.createElement("div");
    status.className = "settings-tool-status";
    const dot = document.createElement("span");
    dot.className = "dot " + cardState(health);
    const summary = document.createElement("span");
    summary.textContent = health.summary || health.status || "等待检测";
    status.append(dot, summary);
    if (health.version) {
      const version = document.createElement("small");
      version.textContent = health.version;
      status.append(version);
    }
    header.appendChild(status);
    if (!editing) renderToolToggle(header, tool, card);
    const body = document.createElement("div");
    body.className = "settings-detail-body";
    if (editing) {
      const help = document.createElement("p");
      help.className = "settings-help";
      help.textContent = "修改后保存到 Data/config.toml，所有入口共用。";
      body.appendChild(help);
      renderConfigEditor(body, tool);
      renderEditControls(body, tool, card, true);
    } else {
      if (tool.description) {
        const description = document.createElement("p");
        description.className = "settings-help";
        description.textContent = tool.description;
        body.appendChild(description);
      }
      const detail = [health.detail, health.path].filter(Boolean).join(" · ");
      if (detail) {
        const line = document.createElement("p");
        line.className = "tool-detail";
        line.textContent = detail;
        body.appendChild(line);
      }
      if (tool.name === "chatgpt-mcp") {
        renderChatGPTMCP(body, tool);
      } else {
        renderConfigView(body, tool);
        renderModelManager(body, tool);
      }
      if (visibleConfigFields(tool, true).length) renderEditControls(body, tool, card, false);
      if (tool.name !== "chatgpt-mcp") toolActions(body, tool, health);
    }
    card.append(header, body);
    list.appendChild(card);
  }

  async function refreshAbout() {
    const appVersion = document.getElementById("aboutAppVersion");
    const bridgeVersion = document.getElementById("aboutBridgeVersion");
    appVersion.textContent = "读取中…";
    bridgeVersion.textContent = "读取中…";
    try {
      const info = await api.send("ui.about");
      appVersion.textContent = info.extensionVersion ? "v" + info.extensionVersion : "未知";
      bridgeVersion.textContent = info.bridgeVersion ? "v" + info.bridgeVersion : "未连接";
    } catch {
      appVersion.textContent = "未连接";
      bridgeVersion.textContent = "未连接";
    }
  }

  function renderTools(nextTools) {
    tools = Array.isArray(nextTools) ? nextTools : [];
    const list = document.getElementById("toolList");
    const preferences = document.getElementById("settingsInterface");
    const about = document.getElementById("settingsAbout");
    if (!list || !preferences || !about) return;
    list.replaceChildren();
    list.hidden = false;
    preferences.hidden = true;
    about.hidden = true;
    const frame = navigation[navigation.length - 1];
    if (!frame) {
      settingsHeading("设置", "下载、连接、组件与界面偏好");
      addSection(list, "偏好设置");
      list.appendChild(settingsRow({
        title: categoryMeta.appearance.title, subtitle: categoryMeta.appearance.description,
        icon: "appearance", action: () => enter("category", "appearance")
      }));
      addSection(list, "系统与组件");
      for (const kind of ["runtime", "capability", "dependency", "other"]) {
        const count = tools.filter(tool => normalizedKind(tool) === kind).length;
        if (!count) continue;
        list.appendChild(settingsRow({
          title: categoryMeta[kind].title, subtitle: categoryMeta[kind].description,
          icon: categoryMeta[kind].icon, trailing: String(count) + " 项",
          action: () => enter("category", kind)
        }));
      }
      addSection(list, "其他");
      list.appendChild(settingsRow({
        title: "关于", subtitle: "版本与应用信息",
        icon: "about", action: () => enter("about", "vidotter")
      }));
      return;
    }

    if (frame.type === "about") {
      settingsHeading("关于", "VidOtter 版本与应用信息");
      list.hidden = true;
      about.hidden = false;
      refreshAbout();
      return;
    }

    if (frame.type === "category") {
      const group = categoryMeta[frame.id] || categoryMeta.other;
      settingsHeading(group.title, group.description);
      if (frame.id === "appearance") {
        list.hidden = true;
        preferences.hidden = false;
        return;
      }
      const groupTools = tools.filter(tool => normalizedKind(tool) === frame.id);
      for (const tool of groupTools) {
        const health = tool.health || {};
        list.appendChild(settingsRow({
          title: tool.displayName || tool.name, subtitle: health.summary || tool.description,
          icon: group.icon,
          trailing: health.status === "ready" ? "就绪" : health.status === "disabled" ? "未启用" : (health.version || health.status || "未知"),
          health, action: () => enter("tool", tool.name)
        }));
      }
      if (!groupTools.length) addSection(list, "暂无可用组件");
      return;
    }

    const tool = tools.find(item => item.name === frame.id);
    if (!tool) {
      navigation.pop();
      renderTools(tools);
      return;
    }
    const editing = frame.type === "edit";
    settingsHeading(editing ? "编辑 " + (tool.displayName || tool.name) : (tool.displayName || tool.name),
      editing ? "保存后更新 TOML 配置" : (tool.description || "查看组件状态与设置"));
    renderDetail(list, tool, editing);
  }

  async function refresh() {
    setIndicator("checking", "环境检测中");
    api.setMessage("environmentMessage", "");
    try {
      const result = await api.send("bridge.tools");
      try { background = await api.send("bridge.tool.action", {tool: "background", toolAction: "status", target: "bridge"}); }
      catch (_) { background = null; }
      renderTools(result.tools);
      const requiredTools = tools.filter(tool => tool.required);
      const requiredReady = requiredTools.length > 0 && requiredTools.every(tool => tool.health && tool.health.ok);
      setIndicator(requiredReady ? "ok" : "error", requiredReady ? "环境正常" : "环境异常");
    } catch (error) {
      setIndicator("error", "环境异常");
      api.setMessage("environmentMessage", error.message || String(error), "error");
    }
  }

  function init() {
    document.getElementById("refreshEnvironment").addEventListener("click", refresh);
    document.getElementById("settingsBack").addEventListener("click", goBack);
    // The status badge is shared by all tabs. Refresh it even when the
    // popup opens directly on media detection rather than Settings.
    void refresh();
  }

  root.VidOtterEnvironment = { init, activate: refresh, renderTools };

  if (typeof module !== "undefined" && module.exports) {
    module.exports = { mcpSetupIsComplete, mcpEffectiveTunnelID, mcpControlState, cardState, normalizedKind, visibleConfigFields, editorValue, toggleMetadata, formatModelSize, modelOptionLabel, modelActionLabel, isOutdatedMcpWorkerError, mcpStatus };
  }
})(globalThis);
