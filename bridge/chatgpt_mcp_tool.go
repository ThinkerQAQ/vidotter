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
	"strconv"
	"strings"
	"time"
)

// chatGPTMCPTool is the local management entry point for the *optional*
// ChatGPT Tunnel. VidOtter's MCP server is already in the standard executable.
// No credential or arbitrary shell command passes through the extension.
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
	_, profileErr := os.Stat(profile)
	configured := profileErr == nil

	status := "disabled"
	summary := "MCP Server 已内置 · ChatGPT 未配置"
	detail := "本地 MCP 不需要独立安装。连接云端 ChatGPT 时才启用个人 Tunnel。"
	tunnelID := tunnelIDFromProfile(profile)
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
		detail = "输入 Tunnel ID；首次需要本机有效的 Runtime API Key，已保存的凭据可以复用。"
	}
	return toolSnapshot{
		Name: "chatgpt-mcp", DisplayName: "ChatGPT MCP", Kind: "integration",
		Description: "可选云端连接：不影响普通视频下载，也不会默认开放远程访问。",
		Platforms:   []string{"windows", "linux", "darwin"}, Delivery: "bundled", Required: false,
		Capabilities: []string{"mcp.stdio", "tunnel.optional"}, SortOrder: 25,
		Config: toolConfigView{Schema: []toolConfigField{}, Values: map[string]any{"tunnelId": tunnelID}},
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
func mcpSetupCommand(goos, root, tunnelID string) (*exec.Cmd, error) {
	if !validTunnelID(tunnelID) {
		return nil, errors.New("invalid Tunnel ID")
	}
	if root == "" {
		return nil, errors.New("missing VidOtter installation root")
	}
	if goos == "windows" {
		script := filepath.Join(root, "scripts", "Setup-VidOtter-MCP.ps1")
		if info, err := os.Stat(script); err != nil || info.IsDir() {
			return nil, errors.New("VidOtter MCP setup script missing; repair installation")
		}
		// Tunnel ID is an identifier; the restricted Runtime API Key is
		// entered in the local terminal and never passed over HTTP.
		return exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass",
			"-NoExit", "-File", script, "-TunnelId", tunnelID), nil
	}
	script := filepath.Join(root, "scripts", "setup-mcp.sh")
	if info, err := os.Stat(script); err != nil || info.IsDir() {
		return nil, errors.New("VidOtter MCP setup script is missing; repair the installation")
	}
	if goos == "darwin" {
		// Escape for the shell, then AppleScript: only the fixed local script.
		shellCmd := "VIDOTTER_TUNNEL_ID=" + tunnelID + " bash " + "'" + strings.ReplaceAll(script, "'", "'\\''") + "'"
		command := "tell application \"Terminal\" to do script " + strconv.Quote(shellCmd)
		return exec.Command("osascript", "-e", command), nil
	}
	if goos == "linux" {
		if terminal, err := exec.LookPath("x-terminal-emulator"); err == nil {
			cmd := exec.Command(terminal, "-e", "bash", script)
			cmd.Env = append(os.Environ(), "VIDOTTER_TUNNEL_ID="+tunnelID)
			return cmd, nil
		}
		if terminal, err := exec.LookPath("gnome-terminal"); err == nil {
			cmd := exec.Command(terminal, "--", "bash", script)
			cmd.Env = append(os.Environ(), "VIDOTTER_TUNNEL_ID="+tunnelID)
			return cmd, nil
		}
		return nil, errors.New("no graphical terminal; run scripts/setup-mcp.sh in a local terminal")
	}
	return nil, errors.New("unsupported platform")
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
		TunnelID string `json:"tunnelId"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || !validTunnelID(body.TunnelID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "请填写有效的 Tunnel ID"})
		return
	}
	home, _ := os.UserHomeDir()
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
	command, err := mcpSetupCommand(runtime.GOOS, root, body.TunnelID)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	command.Dir = root
	if err := command.Start(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "cannot open MCP setup terminal"})
		return
	}
	_ = command.Process.Release()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "已启动本地 Tunnel 配置向导；请在系统终端完成密钥授权和服务检查"})
}
