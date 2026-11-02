package vidotter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func backgroundCommand(goos, root, action, target string) (*exec.Cmd, error) {
	if (action != "status" && action != "enable" && action != "disable") ||
		(target != "bridge" && target != "tunnel") {
		return nil, errors.New("unsupported background action")
	}
	if goos == "windows" {
		return exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden",
			"-File", filepath.Join(root, "scripts", "Manage-VidOtter-Background.ps1"), "-Action", action, "-Target", target), nil
	}
	if goos == "linux" || goos == "darwin" {
		return exec.Command("bash", filepath.Join(root, "scripts", "manage-background.sh"), action, target), nil
	}
	return nil, errors.New("unsupported background platform")
}

func (s *bridgeServer) handleBackground(w http.ResponseWriter, r *http.Request) {
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
		Target string `json:"target"`
	}
	d := json.NewDecoder(io.LimitReader(r.Body, 4096))
	d.DisallowUnknownFields()
	if d.Decode(&body) != nil || d.Decode(new(any)) != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "invalid background request"})
		return
	}
	executable, err := currentExecutable()
	if err != nil {
		writeJSON(w, 500, map[string]any{"ok": false, "error": "installation unavailable"})
		return
	}
	command, err := backgroundCommand(runtime.GOOS, filepath.Dir(filepath.Dir(executable)), body.Action, body.Target)
	if err != nil {
		writeJSON(w, 400, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, command.Path, command.Args[1:]...)
	output, err := cmd.Output()
	var status struct {
		BridgeEnabled bool `json:"bridgeEnabled"`
		TunnelEnabled bool `json:"tunnelEnabled"`
	}
	if err != nil || json.Unmarshal(output, &status) != nil {
		writeJSON(w, 409, map[string]any{"ok": false, "error": "Background service action failed; check your local service manager"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "bridgeEnabled": status.BridgeEnabled, "tunnelEnabled": status.TunnelEnabled})
}
