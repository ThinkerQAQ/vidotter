package vidotter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// chatGPTMCPTool is the local management entry point for the *optional*
// ChatGPT Tunnel. VidOtter's MCP server is already in the standard executable.
// Credentials are accepted only for an authenticated local setup submission.
// They are streamed to the fixed setup script through stdin, never command arguments.
type chatGPTMCPTool struct{}

var tunnelIDPattern = regexp.MustCompile(`^tunnel_(?:[a-z0-9]{32}|[a-z0-9]{4}_[a-z0-9]{32})$`)

func validTunnelID(tunnelID string) bool {
	return tunnelIDPattern.MatchString(tunnelID)
}

func (chatGPTMCPTool) Name() string { return "chatgpt-mcp" }

// Tunnel IDs identify connections, but are not credentials. Never return
// control-plane keys, file paths, or authentication environment values.
var tunnelProfileID = regexp.MustCompile(`(?m)^\s*tunnel_id:\s*["']?(tunnel_[a-z0-9_]+)`)

func tunnelIDFromProfile(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 128<<10))
	if err != nil {
		return ""
	}
	match := tunnelProfileID.FindSubmatch(data)
	if len(match) != 2 || !validTunnelID(string(match[1])) {
		return ""
	}
	return string(match[1])
}

func tunnelHealthURLFile(goos, home, localAppData, xdgConfigHome string) string {
	if goos == "windows" {
		return filepath.Join(localAppData, "VidOtter", "MCP", "health.url")
	}
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfigHome, "vidotter", "mcp", "health.url")
}

// Trust only the official tunnel-client's readiness+poll health check.
// A profile file or running process alone cannot establish connectivity.
func tunnelIsReady(clientPath, urlFile string) bool {
	if clientPath == "" {
		return false
	}
	if stat, err := os.Stat(urlFile); err != nil || stat.IsDir() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, clientPath, "health", "--url-file", urlFile,
		"--require-control-plane-poll", "--json")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	// The official client exits nonzero when --require-control-plane-poll
	// finds the daemon unhealthy, not ready, or missing a successful poll.
	return cmd.Run() == nil
}

func mcpProfilePath(goos, home, localAppData, xdgConfigHome string) string {
	if goos == "windows" {
		return filepath.Join(localAppData, "VidOtter", "MCP", "vidotter.yaml")
	}
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(home, ".config")
	}
	return filepath.Join(xdgConfigHome, "tunnel-client", "vidotter.yaml")
}

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

func (chatGPTMCPTool) Snapshot(_ context.Context, _ bridgeConfig) toolSnapshot {
	home, _ := os.UserHomeDir()
	profile := mcpProfilePath(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME"))
	client := findTunnelClient(runtime.GOOS, home)
	tunnelID := tunnelIDFromProfile(profile)
	configured := tunnelID != ""

	status := "disabled"
	summary := "MCP Server 已内置 · ChatGPT 未配置"
	detail := "本地 MCP 不需要独立安装。连接云端 ChatGPT 时才启用个人 Tunnel。"
	ready := false
	if client == "" {
		status = "missing"
		summary = "尚未安装 Tunnel Client"
		detail = "点击安装官方客户端，VidOtter 会自动校验发行包；不会自动开启远程服务。"
	} else if configured {
		status = "checking"
		summary = "Tunnel 已配置 · 连接待验证"
		detail = "配置已存在；需要本地守护进程通过健康检查，ChatGPT 还须单独授权。"
		healthURL := tunnelHealthURLFile(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME"))
		if tunnelIsReady(client, healthURL) {
			status, ready = "ready", true
			summary = "Tunnel 服务 Ready"
			detail = "本机 Tunnel 已经通过控制平面轮询健康检查；ChatGPT 插件是否创建还需用户确认。"
		}
	} else {
		detail = "在同一表单填写 Tunnel ID 和 Runtime API Key；验证通过后保存。"
	}
	private := filepath.Dir(tunnelHealthURLFile(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME")))
	revision, _ := os.ReadFile(filepath.Join(private, "configuration-revision"))
	setupState, _ := os.ReadFile(filepath.Join(private, "setup-state"))
	_, restartRequiredErr := os.Stat(filepath.Join(private, "restart-required"))
	keyName := "runtime.env"
	if runtime.GOOS == "windows" {
		keyName = "runtime-key.dpapi"
	}
	keyInfo, keyErr := os.Stat(filepath.Join(private, keyName))
	keySaved := keyErr == nil && !keyInfo.IsDir()
	return toolSnapshot{
		Name: "chatgpt-mcp", DisplayName: "ChatGPT MCP", Kind: "integration",
		Description: "可选云端连接：不影响普通视频下载，也不会默认开放远程访问。",
		Platforms:   []string{"windows", "linux", "darwin"}, Delivery: "bundled", Required: false,
		Capabilities: []string{"mcp.stdio", "tunnel.optional"}, SortOrder: 25,
		Config: toolConfigView{Schema: []toolConfigField{}, Values: map[string]any{
			"tunnelId": tunnelID, "configured": configured, "clientInstalled": client != "", "ready": ready, "running": tunnelProcessRunning(profile),
			"revision": strings.TrimSpace(string(revision)), "runtimeKeySaved": keySaved, "setupState": strings.TrimSpace(string(setupState)), "restartRequired": restartRequiredErr == nil,
		}},
		Health: toolHealth{Status: status, OK: ready, Summary: summary,
			Detail: detail, CheckedAt: time.Now()},
		Actions: []toolAction{{ID: "setup", Label: "配置并启动",
			Description: "Tunnel Client 安装后，在本机配置用户级后台服务。"},
			{ID: "install-client", Label: "安装官方 Tunnel Client",
				Description: "用户确认后下载 OpenAI 官方发行包，校验 SHA-256 并安装在用户目录。"}},
	}
}

// command creation is separately tested. All paths are derived from the
// installed binary's parent directory; the browser never supplies arguments.
var mcpSetupMu sync.Mutex

func mcpSetupCommand(goos, root, tunnelID string, replace bool, previousID string) (*exec.Cmd, error) {
	return mcpSaveCommand(goos, root, tunnelID, replace, previousID, false, false)
}

func mcpSaveCommand(goos, root, tunnelID string, replace bool, previousID string, saveOnly, reuseKey bool) (*exec.Cmd, error) {
	if !validTunnelID(tunnelID) || (replace && !validTunnelID(previousID)) {
		return nil, errors.New("invalid Tunnel ID")
	}
	if root == "" {
		return nil, errors.New("missing VidOtter installation root")
	}
	var cmd *exec.Cmd
	var script string
	if goos == "windows" {
		script = filepath.Join(root, "scripts", "Setup-VidOtter-MCP.ps1")
		args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-File", script, "-TunnelId", tunnelID, "-KeyFromStdin"}
		if replace {
			args = append(args, "-ReplaceExisting", "-ExpectedTunnelId", previousID)
		}
		if saveOnly {
			args = append(args, "-SaveOnly")
		}
		if reuseKey {
			args = append(args, "-ReuseSavedKey")
		}
		cmd = exec.Command("powershell.exe", args...)
		hideMCPSetupWindow(cmd)
	} else if goos == "linux" || goos == "darwin" {
		script = filepath.Join(root, "scripts", "setup-mcp.sh")
		cmd = exec.Command("bash", script)
		cmd.Env = append(os.Environ(), "VIDOTTER_TUNNEL_ID="+tunnelID, "VIDOTTER_MCP_KEY_STDIN=1")
		if replace {
			cmd.Env = append(cmd.Env, "VIDOTTER_REPLACE_TUNNEL=1", "VIDOTTER_EXPECTED_TUNNEL_ID="+previousID)
		}
		if saveOnly {
			cmd.Env = append(cmd.Env, "VIDOTTER_MCP_SAVE_ONLY=1")
		}
		if reuseKey {
			cmd.Env = append(cmd.Env, "VIDOTTER_MCP_REUSE_KEY=1")
		}
	} else {
		return nil, errors.New("unsupported platform")
	}
	if info, err := os.Stat(script); err != nil || info.IsDir() {
		return nil, errors.New("VidOtter MCP setup script missing; repair installation")
	}
	return cmd, nil
}

func (s *bridgeServer) handleMCPSetup(w http.ResponseWriter, r *http.Request) {
	s.allowExtension(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost || !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"ok": false, "error": "unauthorized"})
		return
	}
	var body struct {
		RuntimeAPIKey    string `json:"runtimeApiKey"`
		TunnelID         string `json:"tunnelId"`
		Replace          bool   `json:"replace"`
		PreviousTunnelID string `json:"previousTunnelId"`
		SaveOnly         bool   `json:"saveOnly"`
		ReuseKey         bool   `json:"reuseKey"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || decoder.Decode(new(any)) != io.EOF || !validTunnelID(body.TunnelID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "请填写有效的 Tunnel ID"})
		return
	}
	home, _ := os.UserHomeDir()
	existingID := tunnelIDFromProfile(mcpProfilePath(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME")))
	if body.Replace {
		if !validTunnelID(existingID) || !validTunnelID(body.PreviousTunnelID) ||
			existingID != body.PreviousTunnelID {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "旧 Tunnel ID 已变化，刷新页面后重试"})
			return
		}
	} else if existingID != "" {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "需要先在设置中确认更换连接配置"})
		return
	}
	body.RuntimeAPIKey = strings.TrimSpace(body.RuntimeAPIKey)
	if (len(body.RuntimeAPIKey) < 16 && !body.ReuseKey) || len(body.RuntimeAPIKey) > 512 || strings.ContainsAny(body.RuntimeAPIKey, "\r\n\x00") || (body.ReuseKey && body.RuntimeAPIKey != "") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "请填写有效的 Runtime API Key"})
		return
	}
	if body.ReuseKey {
		keyName := "runtime.env"
		if runtime.GOOS == "windows" {
			keyName = "runtime-key.dpapi"
		}
		keyPath := filepath.Join(filepath.Dir(tunnelHealthURLFile(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME"))), keyName)
		if !body.Replace || !validTunnelID(existingID) {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "No existing Tunnel credentials to reuse"})
			return
		}
		if info, err := os.Stat(keyPath); err != nil || info.IsDir() || info.Size() == 0 {
			writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "Saved runtime key is unavailable"})
			return
		}
	}
	if findTunnelClient(runtime.GOOS, home) == "" {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false,
			"error": "尚未安装官方 tunnel-client。请先打开 VidOtter 的配置文档，安装后再继续。"})
		return
	}
	executable, err := currentExecutable()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "cannot locate local installation"})
		return
	}
	root := filepath.Dir(filepath.Dir(executable))
	command, err := mcpSaveCommand(runtime.GOOS, root, body.TunnelID, body.Replace, body.PreviousTunnelID, body.SaveOnly, body.ReuseKey)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !mcpSetupMu.TryLock() {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "正在验证连接配置，请稍候"})
		return
	}
	statePath := filepath.Join(filepath.Dir(tunnelHealthURLFile(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME"))), "setup-state")
	if err := os.MkdirAll(filepath.Dir(statePath), 0700); err != nil {
		mcpSetupMu.Unlock()
		writeJSON(w, 500, map[string]any{"ok": false, "error": "无法保存配置状态"})
		return
	}
	if err := os.WriteFile(statePath, []byte("running"), 0600); err != nil {
		mcpSetupMu.Unlock()
		writeJSON(w, 500, map[string]any{"ok": false, "error": "无法保存配置状态"})
		return
	}
	command.Dir = root
	command.Stdin = strings.NewReader(body.RuntimeAPIKey + "\n")
	body.RuntimeAPIKey = ""
	// nil output streams use the OS null device; no secrets or inherited pipe writers.
	if err := command.Start(); err != nil {
		_ = os.WriteFile(statePath, []byte("failed"), 0600)
		mcpSetupMu.Unlock()
		writeJSON(w, 500, map[string]any{"ok": false, "error": "无法启动后台配置验证"})
		return
	}
	go func() {
		defer mcpSetupMu.Unlock()
		state := "succeeded"
		if command.Wait() != nil {
			state = "failed"
		}
		command.Stdin = nil
		_ = os.WriteFile(statePath, []byte(state), 0600)
	}()
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "message": "正在后台验证 Tunnel ID 和 Runtime API Key"})
}
