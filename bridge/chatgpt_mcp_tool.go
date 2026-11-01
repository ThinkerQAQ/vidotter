package vidotter

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// chatGPTMCPTool is the local management entry point for the *optional*
// ChatGPT Tunnel. VidOtter's MCP server is already in the standard executable.
// No credential or arbitrary shell command passes through the extension.
type chatGPTMCPTool struct{}

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

	summary := "MCP Server 已内置 · ChatGPT 未配置"
	detail := "本地 MCP 不需要独立安装。连接云端 ChatGPT 时才启用个人 Tunnel。"
	if configured {
		summary = "Tunnel 已配置 · 运行状态未检测"
		detail = "已发现本地配置；请通过 Tunnel 健康检查确认连接是否在线。"
	} else if client == "" {
		detail = "需先安装官方 tunnel-client，然后使用个人 Tunnel ID 和受限运行密钥。"
	}
	return toolSnapshot{
		Name: "chatgpt-mcp", DisplayName: "ChatGPT MCP", Kind: "integration",
		Description: "可选云端连接：不影响普通视频下载，也不会默认开放远程访问。",
		Platforms:   []string{"windows", "linux", "darwin"}, Delivery: "bundled", Required: false,
		Capabilities: []string{"mcp.stdio", "tunnel.optional"}, SortOrder: 25,
		Config: toolConfigView{Schema: []toolConfigField{}, Values: map[string]any{}},
		Health: toolHealth{Status: "disabled", OK: true, Summary: summary,
			Detail: detail, CheckedAt: time.Now()},
		Actions: []toolAction{{ID: "setup", Label: "配置 ChatGPT 连接",
			Description: "明确授权后，启动本机 Tunnel 配置向导；不会传递或保存密钥到浏览器扩展。"}},
	}
}

// command creation is separately tested. All paths are derived from the
// installed binary's parent directory; the browser never supplies arguments.
func mcpSetupCommand(goos, root string) (*exec.Cmd, error) {
	if root == "" {
		return nil, errors.New("missing VidOtter installation root")
	}
	if goos == "windows" {
		scripts := filepath.Join(root, "scripts")
		helper := filepath.Join(scripts, "VidOtter-FirstRun.ps1")
		args := []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-NoExit", "-File"}
		if stat, err := os.Stat(helper); err == nil && !stat.IsDir() {
			args = append(args, helper, "-Mode", "mcp")
		} else {
			script := filepath.Join(scripts, "Setup-VidOtter-MCP.ps1")
			if info, err := os.Stat(script); err != nil || info.IsDir() {
				return nil, errors.New("VidOtter MCP setup script is missing; repair the installation")
			}
			args = append(args, script)
		}
		// A visible terminal is required for consent and secret entry.
		return exec.Command("powershell.exe", args...), nil
	}
	script := filepath.Join(root, "scripts", "setup-mcp.sh")
	if info, err := os.Stat(script); err != nil || info.IsDir() {
		return nil, errors.New("VidOtter MCP setup script is missing; repair the installation")
	}
	if goos == "darwin" {
		// Escape for the shell, then AppleScript: only the fixed local script.
		shellCmd := "bash " + "'" + strings.ReplaceAll(script, "'", "'\\''") + "'"
		command := "tell application \"Terminal\" to do script " + strconv.Quote(shellCmd)
		return exec.Command("osascript", "-e", command), nil
	}
	if goos == "linux" {
		if terminal, err := exec.LookPath("x-terminal-emulator"); err == nil {
			return exec.Command(terminal, "-e", "bash", script), nil
		}
		if terminal, err := exec.LookPath("gnome-terminal"); err == nil {
			return exec.Command(terminal, "--", "bash", script), nil
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
	executable, err := currentExecutable()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "cannot locate local installation"})
		return
	}
	root := filepath.Dir(filepath.Dir(executable))
	command, err := mcpSetupCommand(runtime.GOOS, root)
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
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "MCP setup opened locally; cloud access is not enabled until setup completes"})
}
