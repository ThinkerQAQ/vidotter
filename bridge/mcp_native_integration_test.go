package vidotter

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"vidotter/internal/tunnel"
)

func isolateMCP(t *testing.T) string {
	t.Helper()
	install := t.TempDir()
	root := filepath.Join(install, "Data", "mcp")
	previous := mcpRootForRequest
	mcpRootForRequest = func() string { return root }
	t.Cleanup(func() { mcpRootForRequest = previous })
	t.Setenv("USERPROFILE", filepath.Join(install, "home"))
	t.Setenv("HOME", filepath.Join(install, "home"))
	t.Setenv("LOCALAPPDATA", filepath.Join(install, "local"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(install, "config"))
	name := "tunnel-client"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.MkdirAll(filepath.Join(install, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "bin", name), []byte("synthetic-client"), 0700); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestMCPNativeSaveRotationStateAndNoSecretResponse(t *testing.T) {
	root := isolateMCP(t)
	s := &bridgeServer{state: bridgeState{Token: "synthetic-local-token"}}
	request := func(body map[string]any) *httptest.ResponseRecorder {
		t.Helper()
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", "/v1/mcp/setup", strings.NewReader(string(raw)))
		r.Header.Set("X-VidOtter-Token", "synthetic-local-token")
		w := httptest.NewRecorder()
		s.handleMCPSetup(w, r)
		return w
	}
	id := "tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	first := map[string]any{"tunnelId": id, "runtimeApiKey": "SYNTHETIC_PRIVATE_KEY_FOR_SAVE", "tunnelAutoConnect": false, "saveOnly": true}
	if w := request(first); w.Code != 200 {
		t.Fatalf("first Save: %d %s", w.Code, w.Body.String())
	}
	state := tunnel.Status(root)
	if state.Managed || state.AutoConnect || !state.HasKey || state.ID != id {
		t.Fatalf("Save changed runtime or lost settings: %+v", state)
	}
	revision := state.Revision
	snapshot := chatGPTMCPTool{}.Snapshot(context.Background(), defaultBridgeConfig())
	raw, _ := json.Marshal(snapshot)
	if strings.Contains(string(raw), "SYNTHETIC_PRIVATE_KEY") || snapshot.Config.Values["tunnelAutoConnect"] != false {
		t.Fatal("snapshot leaked secret or lost auto-connect state")
	}
	if w := request(map[string]any{"tunnelId": id, "replace": true, "previousTunnelId": id, "runtimeApiKey": "", "reuseKey": true, "saveOnly": true, "tunnelAutoConnect": false}); w.Code != 200 {
		t.Fatalf("reuse: %d %s", w.Code, w.Body.String())
	}
	if tunnel.Status(root).Revision != revision {
		t.Fatal("unchanged Save changed revision")
	}
	if w := request(map[string]any{"tunnelId": id, "replace": true, "previousTunnelId": "tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "runtimeApiKey": "SYNTHETIC_PRIVATE_KEY_CHANGED"}); w.Code != 409 {
		t.Fatal("stale Save accepted")
	}
	if tunnel.Status(root).Revision != revision {
		t.Fatal("stale Save modified configuration")
	}
}
