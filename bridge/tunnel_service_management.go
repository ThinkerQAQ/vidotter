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
	"runtime"
	"time"
)

// Runtime lifecycle management is separate from installation and intentionally
// uses only fixed app-owned scripts. No shell command is accepted from browser.
func tunnelServiceCommand(goos, root, action string) (*exec.Cmd, error) {
	if action != "start" && action != "stop" && action != "restart" {
		return nil, errors.New("invalid Tunnel runtime action")
	}
	if root == "" {
		return nil, errors.New("installation root not found")
	}
	var cmd *exec.Cmd
	var script string
	if goos == "windows" {
		script = filepath.Join(root, "scripts", "Manage-VidOtter-MCP.ps1")
		cmd = exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass",
			"-File", script, "-Action", action)
	} else if goos == "linux" || goos == "darwin" {
		script = filepath.Join(root, "scripts", "manage-mcp.sh")
		cmd = exec.Command("bash", script, action)
	} else {
		return nil, errors.New("unsupported Tunnel runtime platform")
	}
	info, err := os.Stat(script)
	if err != nil || info.IsDir() {
		return nil, errors.New("VidOtter Tunnel runtime scripts are missing; repair installation")
	}
	return cmd, nil
}

func (s *bridgeServer) handleMCPServiceManage(w http.ResponseWriter, r *http.Request) {
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
		Action string `json:"action"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || (body.Action != "start" && body.Action != "stop" && body.Action != "restart") {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "unsupported Tunnel action"})
		return
	}
	home, _ := os.UserHomeDir()
	profile := mcpProfilePath(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME"))
	if stat, err := os.Stat(profile); err != nil || stat.IsDir() {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "Configure VidOtter Tunnel before managing its service"})
		return
	}
	executable, err := currentExecutable()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"ok": false, "error": "Cannot determine installed VidOtter path"})
		return
	}
	root := filepath.Dir(filepath.Dir(executable))
	command, err := tunnelServiceCommand(runtime.GOOS, root, body.Action)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command.Path, command.Args[1:]...)
	cmd.Dir = root
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard // OS process command output may contain local paths
	if err := cmd.Run(); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "error": "Unable to " + body.Action + " VidOtter Tunnel service; check local service manager"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Tunnel service action submitted; verify connection readiness separately"})
}
