(function (root) {
  "use strict";

  const api = root.VidOtterPopup;
  let tools = [];
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
  const mcpUI = {draftID:"",idEdited:false,draftKey:"",busy:"",error:"",info:"",timer:null,polling:false,
    workerOutdated:false,draftStartup:undefined,draftAutoConnect:undefined,available:true};
  const mcpIDPattern = /^tunnel_(?:[a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$/;
  // This ID is used by our smoke-test fixtures. It is syntactically valid,
  // but must not masquerade as an actual connected user Tunnel.
  const mcpExampleID = "tunnel_0123456789abcdef0123456789abcdef";
  const mcpIsExampleID = id => String(id || "").trim() === mcpExampleID;
  // The rendered field is authoritative at Save time. Password managers can
  // assign .value without emitting "input", leaving draftID and idEdited stale.
  // Blank means "reuse saved ID", even after the user clears an edited field.
  function mcpDisplayedTunnelID(state, ui) {
    return ui.idEdited ? ui.draftID : (state.tunnelID || "");
  }

  function mcpSavedKeyMask(saved, draftKey) {
    // This is a visual indicator, not the real key or its length.
    return saved && !draftKey ? "****************" : "";
  }

  function mcpEffectiveTunnelID(form, savedID, draftID, edited = true) {
    const field = form?.querySelector?.('input[name="tunnelId"]');
    const entered = String(field ? field.value : (edited ? draftID : "") || "").trim();
    return entered || String(savedID || "").trim();
  }

  function mcpVisibleFieldChanges(form, state, ui) {
    const visibleID = form?.querySelector?.('input[name="tunnelId"]')?.value?.trim() || "";
    return {
      draftID: visibleID,
      idEdited: Boolean(ui.idEdited || (visibleID && visibleID !== state.tunnelID)),
      draftKey: ui.draftKey
    };
  }

  function mcpDiscourageAutofill(input, intendedValue = () => "") {
    // A password manager may set .value without an input event. Keep the
    // actual field consistent with intentional edits, not browser credentials.
    input.readOnly = true;
    input.autocomplete = "off";
    input.setAttribute("data-lpignore", "true");
    input.setAttribute("data-1p-ignore", "true");
    input.setAttribute("data-bwignore", "true");
    const scrub = () => {
      if (!input.isConnected) return;
      const expected = String(intendedValue() || "");
      if (input.value !== expected && (document.activeElement !== input ||
          input.matches?.(":-webkit-autofill"))) {
        input.value = expected;
      }
    };
    input.addEventListener("focus", () => {
      // Restore the intended field value before browser suggestions are shown.
      const expected = String(intendedValue() || "");
      if (input.value !== expected) input.value = expected;
      const error = input.closest(".mcp-connection-card")?.querySelector(".mcp-error");
      if (error) { mcpUI.error = ""; error.remove(); }
    });
    const unlock = () => { input.readOnly = false; };
    input.addEventListener("pointerdown", unlock);
    input.addEventListener("keydown", unlock);
    input.addEventListener("paste", unlock);
    setTimeout(scrub, 100);
    setTimeout(scrub, 900);
  }
  // Service controls follow persisted credentials, not an extra click on Save.
  // A running Tunnel must always remain stoppable even while editing.
  function mcpControlState(state, form = {}) {
    const running = Boolean(state.running || state.ready);
    const changedID = Boolean(form.idEdited &&
      String(form.draftID || "").trim() !== String(state.tunnelID || "").trim());
    const hasChanges = changedID || Boolean(String(form.draftKey || "").trim()) ||
      (form.draftStartup !== undefined && form.draftStartup !== state.autostartEnabled) ||
      (form.draftAutoConnect !== undefined && form.draftAutoConnect !== state.tunnelAutoConnect);
    const credentialsSaved = Boolean(state.configured && state.clientInstalled !== false && state.runtimeKeySaved &&
      mcpIDPattern.test(String(state.tunnelID || "")) && !mcpIsExampleID(state.tunnelID));
    const usable = credentialsSaved && !state.serviceError && form.available !== false && !hasChanges && !form.busy;
    return {
      running, hasChanges, credentialsSaved,
      canStart: !running && usable,
      canStop: running && !form.busy,
      canRestart: running && usable,
      canAutoStart: !form.busy && state.autostartEnabled !== null,
      canConnect: running && usable && !state.restartRequired && !state.tunnelConnecting,
      canDisconnect: running && Boolean(state.tunnelConnecting) && !form.busy
    };
  }

  // Top-right status is the single display of local Tunnel state. A ready
  // Tunnel does not imply authorization on the ChatGPT plugin side.
  function mcpHeaderStatus(state, busy = "") {
    if (busy === "saving" || busy === "waiting") return { label: "正在保存配置", kind: "pending" };
    if (busy === "connecting") return {label:"正在连接 Tunnel",kind:"pending"};
    if (busy === "disconnecting") return {label:"正在断开 Tunnel",kind:"pending"};
    if (state.serviceError) return {label:state.serviceError,kind:"error"};
    if (busy === "starting") return { label: "正在启动 Tunnel", kind: "pending" };
    if (busy === "stopping") return { label: "正在停止 Tunnel", kind: "pending" };
    if (busy === "restarting") return { label: "正在重启 Tunnel", kind: "pending" };
    if (state.running && state.restartRequired) {
      return { label: "配置已保存 · 待重启", kind: "pending" };
    }
    if (state.ready) return { label: "Tunnel 服务 Ready", kind: "ready" };
    if (state.running && state.tunnelConnecting === false) return {label:"服务运行中 · Tunnel 未连接",kind:"pending"};
    if (state.running) return {
      label: state.restartRequired ? "Tunnel 运行中 · 配置待重启" : "Tunnel 运行中 · 尚未就绪",
      kind: "pending"
    };
    if (mcpIsExampleID(state.tunnelID)) return { label: "Tunnel 测试配置待更换", kind: "pending" };
    if (state.configured && !state.runtimeKeySaved) return { label: "缺少 Runtime API Key", kind: "pending" };
    if (state.configured) return { label: "Tunnel 已配置 · 未启动", kind: "pending" };
    return { label: "Tunnel 尚未配置", kind: "pending" };
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
      restartRequired: Boolean(values.restartRequired), running: Boolean(values.running),
      autostartEnabled: values.autostartEnabled ?? null, tunnelAutoConnect: values.tunnelAutoConnect !== false,
      tunnelConnecting: values.tunnelConnecting ?? Boolean(values.running), serviceError: values.serviceError || ""
    };
  }

  async function mcpPoll() {
    if (!mcpIsVisible() || mcpUI.polling || mcpUI.busy) { scheduleMcpPoll(); return; }
    mcpUI.polling = true;
    try {
      const response = await api.send("bridge.tools");
      if (!mcpUI.busy) {
        const next = response.tools || [];
        const before = tools.find(tool => tool.name === "chatgpt-mcp");
        const after = next.find(tool => tool.name === "chatgpt-mcp");
        const recovered = !mcpUI.available;
        mcpUI.available = true;
        if (mcpUI.error === "无法读取本地服务状态，正在自动重试。") mcpUI.error = "";
        if (recovered || JSON.stringify(mcpStatus(before)) !== JSON.stringify(mcpStatus(after))) {
          const focused = document.activeElement;
          const name = focused?.name, start = focused?.selectionStart, end = focused?.selectionEnd;
          renderTools(next);
          const replacement = name && document.querySelector(`[name="${name}"]`);
          if (replacement) { replacement.focus(); if (start !== null && start !== undefined) replacement.setSelectionRange?.(start,end); }
        }
      }
    } catch {
      mcpUI.available = false;
      mcpUI.error = "无法读取本地服务状态，正在自动重试。";
      renderTools(tools);
    } finally { mcpUI.polling = false; scheduleMcpPoll(); }
  }

  async function mcpAction(action, tool, body) {
    if (mcpUI.busy) return;
    const state = mcpStatus(tool);
    mcpUI.error = "";
    mcpUI.info = "";
    if (action === "save") {
      const tunnelId = mcpEffectiveTunnelID(body, state.tunnelID, mcpUI.draftID, mcpUI.idEdited);
      const runtimeApiKey = mcpUI.draftKey.trim();
      const reuseKey = !runtimeApiKey && state.runtimeKeySaved;
      if (mcpIsExampleID(tunnelId)) {
        mcpUI.error = "当前保存的是测试用 Tunnel ID，请填写真实 Tunnel ID 后再保存。";
        renderTools(tools);
        return;
      }
      if (!mcpIDPattern.test(tunnelId)) {
        mcpUI.error = "请输入有效的 Tunnel ID。";
        renderTools(tools);
        return;
      }
      if (!reuseKey && (runtimeApiKey.length < 16 || runtimeApiKey.length > 4096 ||
          /[\r\n\x00]/.test(runtimeApiKey))) {
        mcpUI.error = "请输入 Runtime API Key，或保留已保存的密钥。";
        renderTools(tools);
        return;
      }
      mcpUI.busy = "saving";
      renderTools(tools);
      try {
        const capability = await api.send("bridge.mcp.capabilities");
        if (capability.protocol < 6 || !["go-lifecycle", "connection-controls", "inline-credentials"]
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
        const unchanged = !mcpControlState(state,mcpUI).hasChanges;
        await api.send("bridge.tool.action", {
          tool: "chatgpt-mcp", toolAction: "setup",
          tunnelId, runtimeApiKey, reuseKey, saveOnly: true,
          replace: state.configured, previousTunnelId: state.configured ? state.tunnelID : "",
          ...(state.autostartEnabled !== null ? {autostartEnabled: mcpUI.draftStartup ?? state.autostartEnabled} : {}),
          tunnelAutoConnect: mcpUI.draftAutoConnect ?? state.tunnelAutoConnect
        });
        mcpUI.draftKey = ""; mcpUI.draftID = ""; mcpUI.idEdited = false;
        mcpUI.draftStartup = undefined; mcpUI.draftAutoConnect = undefined;
        mcpUI.busy = ""; mcpUI.available = true;
        mcpUI.info = unchanged ? "配置已经保存，无需重复保存。" : "配置已保存。运行中的服务需重启后应用连接修改。";
        await refresh();
      } catch (error) {
        mcpUI.busy = "";
        mcpUI.error = error.message || String(error);
        if (isOutdatedMcpWorkerError(error)) mcpUI.workerOutdated = true;
        renderTools(tools);
      }
      return;
    }
    if (["start", "stop", "restart", "connect", "disconnect"].includes(action)) {
      const controls = mcpControlState(state, {
        ...mcpUI, ...mcpVisibleFieldChanges(body, state, mcpUI)
      });
      const permitted = action === "start" ? controls.canStart :
        action === "stop" ? controls.canStop : action === "connect" ? controls.canConnect : action === "disconnect" ? controls.canDisconnect : controls.canRestart;
      if (!permitted) return;
      mcpUI.busy = action === "start" ? "starting" :
        action === "stop" ? "stopping" : action === "connect" ? "connecting" : action === "disconnect" ? "disconnecting" : "restarting";
      renderTools(tools);
      try {
        await api.send("bridge.tool.action", {
          tool: "chatgpt-mcp", toolAction: "service", serviceAction: action
        });
        mcpUI.busy = "";
        mcpUI.info = action === "disconnect" ? "Tunnel 已断开，本地服务继续运行。" : action === "stop" ? "本地服务已停止，自启动偏好保持不变。" : "操作已完成，正在更新状态。";
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

  function mcpFieldHeading(label, url, helpLabel) {
    const heading = document.createElement("span");
    heading.className = "mcp-field-heading";
    heading.appendChild(document.createTextNode(label));
    const help = document.createElement("a");
    help.className = "mcp-field-help";
    help.href = url;
    help.target = "_blank";
    help.rel = "noopener noreferrer";
    help.textContent = "?";
    help.title = helpLabel;
    help.setAttribute("aria-label", helpLabel);
    heading.appendChild(help);
    return heading;
  }

  function mcpStepHeading(number, label, explanation) {
    const heading = document.createElement("div");
    heading.className = "mcp-section-heading";
    const badge = document.createElement("span");
    badge.className = "mcp-step-badge";
    badge.textContent = number;
    badge.setAttribute("aria-hidden", "true");
    const details = document.createElement("div");
    const title = document.createElement("h3");
    title.textContent = label;
    const intro = document.createElement("p");
    intro.className = "settings-help mcp-workflow-intro";
    intro.textContent = explanation;
    details.append(title, intro);
    heading.append(badge, details);
    return heading;
  }

  function mcpKeyField(busy, keySaved = false, onEdit = () => {}) {
    const label = document.createElement("label");
    label.className = "mcp-id-field";
    label.appendChild(mcpFieldHeading(
      "Runtime API Key",
      "https://platform.openai.com/settings/organization/api-keys",
      "创建 Runtime API Key"
    ));
    const input = document.createElement("input");
    input.name = "runtimeApiKey";
    input.type = "password";
    // This is a runtime API key, not a website login password. Discourage
    // browser password managers from injecting an unrelated saved password.
    mcpDiscourageAutofill(input, () => mcpUI.draftKey);
    input.spellcheck = false;
    input.maxLength = 4096;
    // The real saved key never leaves the Bridge. Show a masked visual
    // indicator only; the input value remains empty and Save reuses the key.
    input.placeholder = mcpSavedKeyMask(keySaved, mcpUI.draftKey);
    input.value = mcpUI.draftKey;
    input.disabled = busy;
    input.addEventListener("input", () => {
      // Browser autofill can dispatch an input event for an unrelated saved
      // website password. Do not treat an unfocused autofill as key rotation.
      if (document.activeElement !== input) {
        input.value = mcpUI.draftKey;
        return;
      }
      if (input.matches?.(":-webkit-autofill")) {
        input.value = mcpUI.draftKey;
        return;
      }
      mcpUI.draftKey = input.value;
      mcpUI.error = "";
      input.closest(".mcp-connection-card")?.querySelector(".mcp-error")?.remove();
      // Any manual edit invalidates the session's previous Save confirmation.
      onEdit();
    });
    const wrap = document.createElement("div");
    wrap.className = "mcp-key-input";
    const reveal = document.createElement("button");
    reveal.type = "button";
    reveal.className = "mcp-key-reveal";
    reveal.disabled = busy || !input.value;
    reveal.setAttribute("aria-label", "显示新输入的 Runtime API Key");
    reveal.setAttribute("aria-pressed", "false");
    reveal.title = "只显示本次新输入的密钥；已保存密钥不会返回页面";
    const eye = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    eye.setAttribute("viewBox", "0 0 24 24");
    eye.setAttribute("width", "19");
    eye.setAttribute("height", "19");
    eye.setAttribute("fill", "none");
    eye.setAttribute("stroke", "currentColor");
    eye.setAttribute("stroke-width", "1.6");
    eye.setAttribute("aria-hidden", "true");
    const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
    path.setAttribute("d", "M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z");
    const pupil = document.createElementNS("http://www.w3.org/2000/svg", "circle");
    pupil.setAttribute("cx", "12");
    pupil.setAttribute("cy", "12");
    pupil.setAttribute("r", "3");
    eye.append(path, pupil);
    reveal.appendChild(eye);
    reveal.addEventListener("click", () => {
      if (!input.value || busy) return;
      input.type = input.type === "password" ? "text" : "password";
      reveal.setAttribute("aria-pressed", String(input.type === "text"));
      reveal.setAttribute("aria-label", input.type === "text"
        ? "隐藏新输入的 Runtime API Key" : "显示新输入的 Runtime API Key");
    });
    input.addEventListener("input", () => {
      reveal.disabled = busy || !mcpUI.draftKey;
      if (!mcpUI.draftKey) {
        input.type = "password";
        reveal.setAttribute("aria-pressed", "false");
        reveal.setAttribute("aria-label", "显示新输入的 Runtime API Key");
      }
    });
    wrap.append(input, reveal);
    label.appendChild(wrap);
    return label;
  }

  function renderChatGPTMCP(body, tool) {
    const state = mcpStatus(tool);
    const busy = Boolean(mcpUI.busy);
    const shell = document.createElement("div");
    shell.className = "mcp-guide mcp-workflow";
    const localPanel = document.createElement("section");
    localPanel.className = "mcp-connection-card mcp-workflow-panel mcp-local-service";
    localPanel.appendChild(mcpStepHeading(
      "01", "本地 Tunnel 服务",
      "保存连接配置后启动服务。服务运行时，重启可应用已保存的修改。"
    ));
    let updateControls = () => {};
    const idLabel = document.createElement("label");
    idLabel.className = "mcp-id-field";
    idLabel.appendChild(mcpFieldHeading(
      "Tunnel ID",
      "https://platform.openai.com/settings/organization/tunnels",
      "创建或管理 Tunnel ID"
    ));
    const idInput = document.createElement("input");
    idInput.name = "tunnelId";
    idInput.type = "text";
    mcpDiscourageAutofill(idInput, () => mcpDisplayedTunnelID(state, mcpUI));
    idInput.spellcheck = false;
    idInput.maxLength = 80;
    idInput.placeholder = "";
    // Tunnel IDs are not secrets. Display the persisted value after Save or
    // reopening the page; preserve draft text only while actively editing.
    idInput.value = mcpDisplayedTunnelID(state, mcpUI);
    idInput.disabled = busy;
    idInput.addEventListener("input", () => {
      if (document.activeElement !== idInput) {
        // A password manager generated an unsolicited input event.
        idInput.value = mcpUI.draftID;
        return;
      }
      if (idInput.matches?.(":-webkit-autofill")) {
        idInput.value = mcpUI.draftID;
        return;
      }
      mcpUI.draftID = idInput.value;
      mcpUI.idEdited = true;
      mcpUI.error = "";
      idInput.closest(".mcp-connection-card")?.querySelector(".mcp-error")?.remove();
      // Even if the user clears this field again, require Save to reconfirm.
      updateControls();
    });
    idLabel.appendChild(idInput);
    localPanel.appendChild(idLabel);
    const idState = document.createElement("small");
    idState.className = "settings-help mcp-credential-state";
    idState.textContent = mcpIsExampleID(state.tunnelID)
      ? "检测到测试示例配置：请填写真实 Tunnel ID"
      : state.configured ? "已保存" : "尚未保存";
    localPanel.appendChild(idState);
    const key = mcpKeyField(busy, state.runtimeKeySaved, () => updateControls());
    localPanel.appendChild(key);
    const keyState = document.createElement("small");
    keyState.className = "settings-help mcp-credential-state";
    keyState.textContent = state.runtimeKeySaved ? "已保存；留空则复用" : "尚未保存";
    localPanel.appendChild(keyState);
    const actions = document.createElement("div");
    actions.className = "mcp-service-actions";
    mcpButton(actions, "Save", () => mcpAction("save", tool, shell), true);
    const running = Boolean(state.running || state.ready);
    const service = mcpButton(actions,
      running ? "Stop Service" : "Start Service",
      () => mcpAction(running ? "stop" : "start", tool, shell), true);
    const restart = mcpButton(actions, "Restart Service",
      () => mcpAction("restart", tool, shell), true);
    localPanel.appendChild(actions);
    const preferences = document.createElement("div");
    preferences.className = "mcp-preferences";
    const preference = (label, hint, checked, field, disabled = false) => {
      const row = document.createElement("label"); row.className = "mcp-preference";
      const input = document.createElement("input"); input.type = "checkbox";
      input.name = field; input.checked = checked; input.disabled = busy || disabled;
      const copy = document.createElement("span"); const title = document.createElement("strong"); title.textContent = label;
      const help = document.createElement("small"); help.textContent = hint; copy.append(title,help);
      input.addEventListener("change", () => {mcpUI[field] = input.checked; mcpUI.info = ""; updateControls();});
      row.append(input,copy); preferences.appendChild(row); return input;
    };
    preference("本地服务登录后自启动", "下次登录系统时启动。保存不会启动或停止当前服务。",
      mcpUI.draftStartup ?? state.autostartEnabled ?? false, "draftStartup", state.autostartEnabled === null);
    preference("Tunnel 自动连接", "本地服务启动后自动连接 Tunnel。修改后重启当前服务生效。",
      mcpUI.draftAutoConnect ?? state.tunnelAutoConnect, "draftAutoConnect");
    localPanel.insertBefore(preferences,actions);
    const connection = mcpButton(localPanel, state.tunnelConnecting ? "断开 Tunnel" : "连接 Tunnel",
      () => mcpAction(state.tunnelConnecting ? "disconnect" : "connect",tool,shell));
    connection.classList.add("mcp-tunnel-action");
    updateControls = () => {
      const visible = mcpVisibleFieldChanges(shell, state, mcpUI);
      const controls = mcpControlState(state, { ...mcpUI, ...visible });
      service.disabled = !(running ? controls.canStop : controls.canStart);
      restart.disabled = !controls.canRestart;
      connection.disabled = !(state.tunnelConnecting ? controls.canDisconnect : controls.canConnect);
      restart.title = "重新启动本地服务并应用保存的配置";
      idState.textContent = visible.draftID && visible.draftID !== state.tunnelID ? "新 Tunnel ID 尚未保存" : state.configured ? "已保存" : "尚未保存";
      keyState.textContent = mcpUI.draftKey ? "新 Runtime API Key 尚未保存" : state.runtimeKeySaved ? "已保存；留空则复用" : "尚未保存";
      if (controls.hasChanges) shell.querySelector(".mcp-info")?.remove();
    };
    updateControls();
    // Browser password managers and some automation tools can update .value
    // without dispatching an input event. Keep hints and service controls
    // consistent with the displayed fields. Stop once this form is replaced.
    let lastIDValue = idInput.value;
    let lastKeyValue = key.querySelector("input").value;
    const reconcile = setInterval(() => {
      if (!shell.isConnected) {
        clearInterval(reconcile);
        return;
      }
      const currentID = idInput.value;
      const currentKey = key.querySelector("input").value;
      if (currentID === lastIDValue && currentKey === lastKeyValue) return;
      lastIDValue = currentID;
      lastKeyValue = currentKey;
      mcpUI.error = "";
      shell.querySelector(".mcp-error")?.remove();
      updateControls();
    }, 350);
    if (state.restartRequired && running) {
      const pending = document.createElement("p");
      pending.className = "settings-help";
      pending.textContent = "配置已保存，当前服务仍使用旧配置，请点击 Restart Service 应用。";
      localPanel.appendChild(pending);
    }
    if (state.autostartEnabled === null) {
      const warning = document.createElement("p"); warning.className = "settings-help mcp-info";
      warning.textContent = "无法读取系统自启动设置，请检查系统权限或登录会话。";
      localPanel.appendChild(warning);
    }
    const pluginPanel = document.createElement("section");
    pluginPanel.className = "mcp-workflow-panel mcp-chatgpt-plugin";
    const pluginNotice = state.running && state.restartRequired
      ? "配置已经保存，请先点击 Restart Service 应用，再去 ChatGPT 验证插件。"
      : state.ready
        ? "本地 Tunnel 已就绪。请在 ChatGPT 中创建或管理插件，授权结果以 ChatGPT 页面为准。"
        : "请先启动本地 Tunnel 并等待就绪，再到 ChatGPT 中创建或管理插件。";
    pluginPanel.appendChild(mcpStepHeading(
      "02", "ChatGPT 插件", pluginNotice
    ));
    const link = document.createElement("a");
    link.className = "mcp-chatgpt-link";
    link.href = chatGPTSettingsURL;
    link.target = "_blank";
    link.rel = "noopener noreferrer";
    link.textContent = "打开 ChatGPT 插件设置 ↗";
    pluginPanel.appendChild(link);
    if (mcpUI.info) {
      const message = document.createElement("p");
      message.className = "settings-help mcp-info";
      message.setAttribute("role", "status");
      message.textContent = mcpUI.info;
      localPanel.appendChild(message);
    }
    if (mcpUI.error) {
      const message = document.createElement("p");
      message.className = "mcp-error";
      message.setAttribute("role", "alert");
      message.textContent = mcpUI.error;
      localPanel.appendChild(message);
    }
    if (mcpUI.workerOutdated) {
      const reloadHint = document.createElement("p");
      reloadHint.className = "mcp-reload-needed";
      localPanel.appendChild(reloadHint);
      showMcpWorkerReload(localPanel, reloadHint);
    }
    shell.append(localPanel, pluginPanel);
    body.appendChild(shell);
    scheduleMcpPoll();
  }

  function renderBrowserAutomation(body, tool) {
    const shell = document.createElement("div");
    shell.className = "mcp-workflow browser-workflow";
    const component = document.createElement("section");
    component.className = "mcp-workflow-panel";
    component.appendChild(mcpStepHeading("01", "浏览器组件",
      "安装 Remote Browser MCP 后，允许 AI 在自己创建的后台标签页查看网页。"));
    const note = document.createElement("p");
    note.className = "settings-help browser-component-note";
    note.textContent = "如果 ChatGPT 已连接独立的 Remote Browser MCP，可直接复用该连接，无需在这里重复启用。";
    component.appendChild(note);
    toolActions(component, tool, tool.health || {});
    const permissions = document.createElement("section");
    permissions.className = "mcp-workflow-panel mcp-chatgpt-plugin";
    permissions.appendChild(mcpStepHeading("02", "页面操作权限",
      tool.config?.values?.interactive
        ? "已允许自动点击。AI 可以在自己创建的标签页中点击页面控件，可随时在上方关闭。"
        : "默认只查看网页。需要自动点击时，请在上方单独开启并确认。"));
    const detail = document.createElement("p");
    detail.className = "settings-help browser-component-note";
    detail.textContent = "已有标签页保持受保护。组件通过 VidOtter 现有连接提供服务，无需另建 Tunnel。";
    permissions.appendChild(detail);
    shell.append(component, permissions);
    body.appendChild(shell);
  }

  function renderDetail(list, tool, editing) {
    const health = tool.health || {};
    const card = document.createElement("section");
    card.className = "settings-detail-card";
    card.dataset.toolName = tool.name || "";
    const isBrowser = tool.name === "browser-automation";
    if (isBrowser || tool.name === "chatgpt-mcp") card.classList.add("connection-detail-card");
    const header = document.createElement("div");
    header.className = "settings-tool-header";
    const isMCP = tool.name === "chatgpt-mcp";
    if (isMCP || isBrowser) {
      header.classList.add("mcp-overview-header");
      const heading = document.createElement("strong");
      heading.className = "mcp-header-name";
      heading.textContent = isBrowser ? "组件状态" : "连接状态";
      header.appendChild(heading);
    }
    const status = document.createElement("div");
    status.className = "settings-tool-status" + (isMCP || isBrowser ? " mcp-header-status" : "");
    const displayed = isMCP ? mcpHeaderStatus(mcpStatus(tool), mcpUI.busy) : null;
    const dot = document.createElement("span");
    dot.className = "dot " + (displayed
      ? displayed.kind === "ready" ? "ok" : displayed.kind === "error" ? "error" : ""
      : cardState(health));
    const summary = document.createElement("span");
    summary.textContent = displayed ? displayed.label :
      health.summary || health.status || "未知";
    status.setAttribute("role", "status");
    status.setAttribute("aria-live", "polite");
    status.dataset.kind = displayed ? displayed.kind : health.ok ? "ready" : health.status === "error" ? "error" : "idle";
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
      if (!isMCP && !isBrowser && tool.description) {
        const description = document.createElement("p");
        description.className = "settings-help";
        description.textContent = tool.description;
        body.appendChild(description);
      }
      const detail = tool.name === "chatgpt-mcp" &&
        mcpIsExampleID(mcpStatus(tool).tunnelID)
        ? "当前保存的是测试示例 ID；请填写真实 Tunnel ID 和对应的 Runtime API Key。"
        : [health.detail, health.path].filter(Boolean).join(" · ");
      if (!isMCP && !isBrowser && detail) {
        const line = document.createElement("p");
        line.className = "tool-detail";
        line.textContent = detail;
        body.appendChild(line);
      }
      if (tool.name === "chatgpt-mcp") {
        renderChatGPTMCP(body, tool);
      } else if (isBrowser) {
        renderBrowserAutomation(body, tool);
      } else {
        renderConfigView(body, tool);
        renderModelManager(body, tool);
      }
      if (visibleConfigFields(tool, true).length) renderEditControls(body, tool, card, false);
      if (!isMCP && !isBrowser) toolActions(body, tool, health);
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
    module.exports = { mcpDisplayedTunnelID, mcpSavedKeyMask, mcpEffectiveTunnelID, mcpVisibleFieldChanges, mcpControlState, mcpHeaderStatus, mcpIsExampleID, cardState, normalizedKind, visibleConfigFields, editorValue, toggleMetadata, formatModelSize, modelOptionLabel, modelActionLabel, isOutdatedMcpWorkerError, mcpStatus };
  }
})(globalThis);
