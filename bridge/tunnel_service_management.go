package vidotter

import (
	"encoding/json"
	"io"
	"net/http"
	"vidotter/internal/tunnel"
)

func (s *bridgeServer) handleMCPServiceManage(w http.ResponseWriter, r *http.Request) {
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
		Action string `json:"action"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || decoder.Decode(new(any)) != io.EOF || !validMCPServiceAction(body.Action) {
		writeJSON(w, 400, map[string]any{"ok": false, "error": "unsupported Tunnel action"})
		return
	}
	if !mcpSetupMu.TryLock() {
		writeJSON(w, 409, map[string]any{"ok": false, "error": "正在执行 MCP 操作"})
		return
	}
	defer mcpSetupMu.Unlock()
	if err := tunnel.Service(mcpRootForRequest(), body.Action); err != nil {
		writeJSON(w, 409, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
func validMCPServiceAction(action string) bool {
	switch action {
	case "start", "stop", "restart", "connect", "disconnect":
		return true
	}
	return false
}
