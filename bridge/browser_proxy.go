package vidotter

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type browserAutomationCall struct {
	Name   string
	URL    string
	PageID int
	UID    string
}

type browserAutomationProxy struct {
	mu             sync.Mutex
	session        *mcp.ClientSession
	reconnectAfter time.Time
}

const browserAutomationTimeout = 75 * time.Second
const browserAutomationCooldown = 30 * time.Second

func (p *browserAutomationProxy) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		_ = p.session.Close()
		p.session = nil
	}
}
func (p *browserAutomationProxy) connect(ctx context.Context) error {
	if p.session != nil {
		return nil
	}
	if time.Now().Before(p.reconnectAfter) {
		return errors.New("browser automation temporarily unavailable; retry after cooldown")
	}
	node, err := resolveBrowserNode()
	if err != nil {
		return err
	}
	dir, err := browserComponentDir()
	if err != nil {
		return err
	}
	if !browserComponentInstalled() {
		return errors.New("browser automation component not installed")
	}
	// Enforce the component's restrictive policy at each launch, even if an
	// independent Remote Browser MCP installation has different settings.
	policy := "[browser]\nname = \"edge\"\nendpoint = \"http://127.0.0.1:9222\"\n\n[security]\nallow_existing_pages = false\nallow_javascript = false\n"
	config := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(config, []byte(policy), 0600); err != nil {
		return err
	}
	cmd := exec.Command(node, filepath.Join(dir, "scripts", "proxy.mjs"))
	cmd.Dir = dir
	env := make([]string, 0, len(os.Environ())+2)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "REMOTE_BROWSER_CONFIG=") || strings.HasPrefix(entry, "REMOTE_BROWSER_PROFILE_DIR=") || strings.HasPrefix(entry, "CDP_BROWSER_URL=") {
			continue
		}
		env = append(env, entry)
	}
	cmd.Env = append(env, "REMOTE_BROWSER_CONFIG="+config, "CDP_BROWSER_URL=http://127.0.0.1:9222")
	browserClient := mcp.NewClient(&mcp.Implementation{Name: "vidotter-browser-adapter", Version: bridgeVersion}, nil)
	connected, err := browserClient.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return errors.New("browser automation process unavailable")
	}
	p.session = connected
	return nil
}
func (p *browserAutomationProxy) call(ctx context.Context, in browserAutomationCall) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	timeout := browserAutomationTimeout
	if in.Name == "browser_status" {
		timeout = 12 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var upstreamName string
	var args map[string]any
	switch in.Name {
	case "browser_status":
		upstreamName = "browser_status"
		args = map[string]any{}
	case "browser_open":
		if err := validateMCPMediaURL(callCtx, in.URL); err != nil {
			return "", err
		}
		upstreamName = "new_page"
		args = map[string]any{"url": in.URL, "background": true}
	case "browser_snapshot":
		if in.PageID <= 0 {
			return "", errors.New("invalid browser page ID")
		}
		upstreamName = "take_snapshot"
		args = map[string]any{"pageId": in.PageID}
	case "browser_click":
		if in.PageID <= 0 || len(in.UID) == 0 || len(in.UID) > 90 {
			return "", errors.New("invalid browser click target")
		}
		upstreamName = "click"
		args = map[string]any{"pageId": in.PageID, "uid": in.UID}
	case "browser_close":
		if in.PageID <= 0 {
			return "", errors.New("invalid browser page ID")
		}
		upstreamName = "close_page"
		args = map[string]any{"pageId": in.PageID}
	default:
		return "", errors.New("browser command is not on the allowlist")
	}
	// Reject unrecognized or unsafe requests before starting a browser
	// child process or touching the user's existing browser session.
	if err := p.connect(callCtx); err != nil {
		return "", err
	}
	result, err := p.session.CallTool(callCtx, &mcp.CallToolParams{Name: upstreamName, Arguments: args})
	if err != nil {
		// Drop the failed MCP child but never retry in the same call: a retry
		// could spawn another CDP client and trigger a second browser prompt.
		_ = p.session.Close()
		p.session = nil
		p.reconnectAfter = time.Now().Add(browserAutomationCooldown)
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
			return "", errors.New("browser automation timed out; no automatic reconnect was attempted")
		}
		if errors.Is(err, context.Canceled) || errors.Is(callCtx.Err(), context.Canceled) {
			return "", errors.New("browser automation request canceled")
		}
		return "", errors.New("browser automation MCP session failed; no automatic reconnect was attempted")
	}
	if result.IsError {
		return "", errors.New("browser tool refused the operation; check owned tabs and browser consent")
	}
	parts := make([]string, 0, len(result.Content))
	for _, piece := range result.Content {
		if text, ok := piece.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
		}
	}
	output := strings.Join(parts, "\n")
	if len(output) > 22000 {
		output = output[:22000] + "\n[truncated]"
	}
	return output, nil
}

func (s *bridgeServer) browserProxy() *browserAutomationProxy {
	s.componentMu.Lock()
	defer s.componentMu.Unlock()
	if s.browserAgent == nil {
		s.browserAgent = &browserAutomationProxy{}
	}
	return s.browserAgent
}
func (s *bridgeServer) handleBrowserAutomationCall(w http.ResponseWriter, r *http.Request) {
	s.allowExtension(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var in browserAutomationCall
	if decodeBridgeJSON(w, r, &in) != nil {
		return
	}
	s.mu.Lock()
	enabled := s.config.BrowserAutomationEnabled
	interactive := s.config.BrowserInteractionEnabled
	s.mu.Unlock()
	if !enabled || !browserComponentInstalled() {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "optional browser automation is not installed and enabled"})
		return
	}
	if in.Name == "browser_click" && !interactive {
		writeJSON(w, http.StatusForbidden, map[string]any{"ok": false, "error": "browser interaction is disabled; enable it explicitly in local VidOtter settings"})
		return
	}
	text, err := s.browserProxy().call(r.Context(), in)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": text})
}

func (s *bridgeServer) handleBrowserAutomation(w http.ResponseWriter, r *http.Request) {
	s.allowExtension(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false})
		return
	}
	if r.Method == http.MethodGet {
		s.mu.Lock()
		config := s.config
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok": true, "installed": browserComponentInstalled(),
			"enabled": config.BrowserAutomationEnabled, "interactive": config.BrowserInteractionEnabled,
			"version": remoteBrowserVersion,
		})
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var in struct{ Action string }
	if decodeBridgeJSON(w, r, &in) != nil {
		return
	}
	s.browserInstallMu.Lock()
	defer s.browserInstallMu.Unlock()
	switch in.Action {
	case "install":
		s.mu.Lock()
		proxy := s.config.Proxy
		s.mu.Unlock()
		if err := installBrowserAutomation(r.Context(), proxy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
	case "enable":
		if !browserComponentInstalled() {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "install browser automation first"})
			return
		}
	case "disable", "enable-interaction", "disable-interaction":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unsupported browser action"})
		return
	}
	s.mu.Lock()
	config := s.config
	switch in.Action {
	case "install", "enable":
		config.BrowserAutomationEnabled = true
	case "disable":
		config.BrowserAutomationEnabled = false
		config.BrowserInteractionEnabled = false
	case "enable-interaction":
		if !config.BrowserAutomationEnabled || !browserComponentInstalled() {
			s.mu.Unlock()
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "enable browser automation first"})
			return
		}
		config.BrowserInteractionEnabled = true
	case "disable-interaction":
		config.BrowserInteractionEnabled = false
	}
	if err := saveBridgeConfig(config); err != nil {
		s.mu.Unlock()
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "cannot save browser preferences"})
		return
	}
	s.config = config
	s.mu.Unlock()
	if in.Action == "disable" {
		s.browserProxy().close()
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "installed": browserComponentInstalled(),
		"enabled": config.BrowserAutomationEnabled, "interactive": config.BrowserInteractionEnabled, "version": remoteBrowserVersion})
}

func (browserAutomationTool) Name() string { return "browser-automation" }

type browserAutomationTool struct{}

func (browserAutomationTool) Snapshot(_ context.Context, config bridgeConfig) toolSnapshot {
	installed := browserComponentInstalled()
	status := "missing"
	summary := "可选组件未安装"
	actions := []toolAction{{ID: "install", Label: "安装并启用", Description: "下载安装经 SHA-256 校验的 Remote Browser MCP 固定版本及 Node 依赖；不配置第二条 Tunnel，也不自动启用页面点击。"}}
	if installed {
		if config.BrowserAutomationEnabled {
			status = "ready"
			summary = "已启用 · 仅后台新标签页"
		}
		if !config.BrowserAutomationEnabled {
			status = "disabled"
			summary = "已安装 · 未启用"
		}
		actions = []toolAction{{ID: "enable", Label: "启用", Description: "允许 AI 使用受限的独立后台浏览器标签页。"}}
		if config.BrowserAutomationEnabled {
			actions = []toolAction{{ID: "disable", Label: "停用", Description: "关闭本地浏览器代理，不删除浏览器用户数据。"}}
			if config.BrowserInteractionEnabled {
				actions = append(actions, toolAction{ID: "disable-interaction", Label: "禁止自动点击", Description: "恢复只读浏览器权限。"})
			} else {
				actions = append(actions, toolAction{ID: "enable-interaction", Label: "允许自动点击", Description: "允许 AI 在自己创建的后台标签页点击控件；网站可能存在提交表单等副作用。"})
			}
		}
	}
	return toolSnapshot{
		Name: "browser-automation", DisplayName: "Browser Automation", Kind: "dependency",
		Description: "可选依赖：独立 Remote Browser MCP v0.3.0，不复制源码、不启动第二 Tunnel。",
		Platforms:   []string{"windows", "linux", "darwin"}, Delivery: "on-demand", Required: false,
		Capabilities: []string{"browser.open-background", "browser.snapshot", "browser.close-owned", "browser.click-opt-in"},
		SortOrder:    90, Actions: actions,
		Config: toolConfigView{Schema: []toolConfigField{}, Values: map[string]any{
			"enabled": config.BrowserAutomationEnabled, "interactive": config.BrowserInteractionEnabled,
		}},
		Health: toolHealth{Status: status, OK: installed && config.BrowserAutomationEnabled, Summary: summary, Version: remoteBrowserVersion, CheckedAt: time.Now()},
	}
}
