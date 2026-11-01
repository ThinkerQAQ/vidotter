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
	if client == "" {
		status = "missing"
		summary = "尚未安装 Tunnel Client"
		detail = "先按 VidOtter 配置文档安装官方 tunnel-client；普通下载不受影响。"
	} else if configured {
		status = "checking"
		summary = "Tunnel 已配置 · 连接待验证"
		detail = "本地已保存配置；请用 doctor/health 检查运行状态，勿将配置视为在线。"
	} else {
		detail = "输入 Tunnel ID；首次启动还需要在本机输入受限 Runtime API Key。"
	}
	return toolSnapshot{
		Name: "chatgpt-mcp", DisplayName: "ChatGPT MCP", Kind: "integration",
		Description: "可选云端连接：不影响普通视频下载，也不会默认开放远程访问。",
		Platforms:   []string{"windows", "linux", "darwin"}, Delivery: "bundled", Required: false,
		Capabilities: []string{"mcp.stdio", "tunnel.optional"}, SortOrder: 25,
		Config: toolConfigView{Schema: []toolConfigField{}, Values: map[string]any{}},
		Health: toolHealth{Status: status, OK: false, Summary: summary,
			Detail: detail, CheckedAt: time.Now()},
		Actions: []toolAction{{ID: "setup", Label: "配置并启动",
			Description: "填写 Tunnel ID 后在本机终端完成 API Key 授权，设置用户级后台服务。"}},
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
