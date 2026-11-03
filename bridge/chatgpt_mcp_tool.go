package vidotter

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"vidotter/internal/tunnel"
)

type chatGPTMCPTool struct{}

var tunnelIDPattern = regexp.MustCompile(`^tunnel_(?:[a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$`)

func validTunnelID(tunnelID string) bool {
	return tunnelIDPattern.MatchString(tunnelID)
}

func (chatGPTMCPTool) Name() string { return "chatgpt-mcp" }

func findTunnelClient(goos, home string) string {
	if path, err := exec.LookPath("tunnel-client"); err == nil {
		return path
	}
	name := "tunnel-client"
	if goos == "windows" {
		name += ".exe"
	}
	candidate := filepath.Join(home, ".local", "bin", name)
	if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
		return candidate
	}
	return ""
}

var mcpSetupMu sync.Mutex
var mcpRootForRequest = mcpRuntimeRoot

func mcpRuntimeRoot() string {
	exe, err := currentExecutable()
	if err != nil {
		return ""
	}
	return tunnel.RootForExecutable(exe)
}
func (chatGPTMCPTool) Snapshot(_ context.Context, _ bridgeConfig) toolSnapshot {
	root := mcpRootForRequest()
	state := tunnel.Status(root)
	id := state.ID
	status, summary := "disabled", "本地服务已停止"
	if state.ID == "" {
		summary = "尚未配置"
	}
	if state.Managed {
		status, summary = "checking", "本地服务运行中 · Tunnel 未连接"
		if state.Connecting {
			summary = "Tunnel 连接中"
		}
	}
	if state.RestartRequired {
		status, summary = "checking", "配置已保存 · 等待重启"
	}
	if state.Ready {
		status, summary = "ready", "Tunnel 已就绪"
	}
	if state.Error != "" {
		status, summary = "error", "服务需要检查"
	}
	return toolSnapshot{Name: "chatgpt-mcp", DisplayName: "ChatGPT MCP", Kind: "integration", Description: "让 ChatGPT 连接 VidOtter。", Platforms: []string{"windows", "linux", "darwin"}, Delivery: "bundled", Required: false, SortOrder: 25, Capabilities: []string{"mcp.stdio", "tunnel.optional"},
		Config: toolConfigView{Schema: []toolConfigField{}, Values: map[string]any{
			"tunnelId": id, "configured": id != "", "clientInstalled": tunnel.ClientInstalled(root), "ready": state.Ready, "running": state.Managed, "revision": state.Revision, "runtimeKeySaved": state.HasKey, "restartRequired": state.RestartRequired, "setupState": "",
			"autostartEnabled": state.Autostart, "tunnelAutoConnect": state.AutoConnect, "tunnelConnecting": state.Connecting, "serviceError": state.Error,
		}}, Health: toolHealth{Status: status, OK: state.Ready, Summary: summary, Detail: state.Error, CheckedAt: time.Now()}, Actions: []toolAction{{ID: "setup", Label: "保存连接配置"}, {ID: "install-client", Label: "安装官方 Tunnel Client"}}}
}
func (s *bridgeServer) handleMCPSetup(w http.ResponseWriter, r *http.Request) {
	s.allowExtension(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost || !s.authorized(r) {
		writeJSON(w, 401, map[string]any{"ok": false, "error": "unauthorized"})
		return
	}
	var body struct {
		RuntimeAPIKey    string `json:"runtimeApiKey"`
		TunnelID         string `json:"tunnelId"`
		Replace          bool   `json:"replace"`
		PreviousTunnelID string `json:"previousTunnelId"`
		SaveOnly         bool   `json:"saveOnly"`
		ReuseKey         bool   `json:"reuseKey"`
		Autostart        *bool  `json:"autostartEnabled"`
		AutoConnect      *bool  `json:"tunnelAutoConnect"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || !validTunnelID(body.TunnelID) {
		writeJSON(w, 400, map[string]any{"ok": false, "error": "请填写有效的 Tunnel ID"})
		return
	}
	if !mcpSetupMu.TryLock() {
		writeJSON(w, 409, map[string]any{"ok": false, "error": "正在执行 MCP 操作，请稍候"})
		return
	}
	defer mcpSetupMu.Unlock()
	root := mcpRootForRequest()
	state := tunnel.Status(root)
	existingID := state.ID
	if existingID != "" && (!body.Replace || body.PreviousTunnelID != existingID) || existingID == "" && body.Replace {
		writeJSON(w, 409, map[string]any{"ok": false, "error": "已保存连接发生变化，请刷新后重试"})
		return
	}
	if len(body.RuntimeAPIKey) > 4096 || strings.ContainsAny(body.RuntimeAPIKey, "\r\n\x00") || body.RuntimeAPIKey != "" && strings.Trim(body.RuntimeAPIKey, "*") == "" || body.ReuseKey && body.RuntimeAPIKey != "" {
		writeJSON(w, 400, map[string]any{"ok": false, "error": "Runtime API Key 必须是单行密钥，不能提交占位符"})
		return
	}
	if body.RuntimeAPIKey == "" && !state.HasKey || body.RuntimeAPIKey != "" && len(body.RuntimeAPIKey) < 16 {
		writeJSON(w, 400, map[string]any{"ok": false, "error": "请填写有效的 Runtime API Key"})
		return
	}
	if err := tunnel.RequireClient(root); err != nil {
		writeJSON(w, 409, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	err := tunnel.Save(root, body.TunnelID, body.RuntimeAPIKey, body.Autostart, body.AutoConnect)
	body.RuntimeAPIKey = ""
	if err != nil {
		writeJSON(w, 409, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "message": "配置已保存；当前运行保持不变"})
}
