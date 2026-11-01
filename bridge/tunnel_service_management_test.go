package vidotter

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTunnelServiceActionsAreFixed(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Manage-VidOtter-MCP.ps1", "manage-mcp.sh"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", name), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		for _, action := range []string{"start", "stop", "restart"} {
			command, err := tunnelServiceCommand(goos, root, action)
			if err != nil {
				t.Fatalf("%s/%s: %v", goos, action, err)
			}
			if len(command.Args) == 0 || command.Args[len(command.Args)-1] != action {
				t.Fatalf("unsafe service args: %#v", command.Args)
			}
		}
	}
	for _, action := range []string{"", "delete", "reload;rm -rf", "start && evil"} {
		if _, err := tunnelServiceCommand("linux", root, action); err == nil {
			t.Fatalf("unsafe action accepted: %s", action)
		}
	}
}

func TestTunnelManageRequiresAuthAndActionAllowlist(t *testing.T) {
	server := &bridgeServer{state: bridgeState{Token: "test-secret"}}
	request := httptest.NewRequest(http.MethodPost, "/v1/mcp/service", strings.NewReader(`{"action":"stop"}`))
	writer := httptest.NewRecorder()
	server.handleMCPServiceManage(writer, request)
	if writer.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", writer.Code)
	}
	for _, body := range []string{`{"action":"delete"}`, `{"action":"restart; rm -rf /"}`, `{}`,
		`{"action":"start","command":"whoami"}`} {
		request := httptest.NewRequest(http.MethodPost, "/v1/mcp/service", strings.NewReader(body))
		request.Header.Set("X-VidOtter-Token", "test-secret")
		writer := httptest.NewRecorder()
		server.handleMCPServiceManage(writer, request)
		if writer.Code != http.StatusBadRequest {
			t.Fatalf("%s returned %d", body, writer.Code)
		}
	}
}

func TestTunnelConfigIDExtractionNoSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vidotter.yaml")
	raw := "control_plane:\n  tunnel_id: \"tunnel_0123456789abcdef0123456789abcdef\"\n  api_key: env:CONTROL_PLANE_API_KEY\n"
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	got := tunnelIDFromProfile(path)
	if got != "tunnel_0123456789abcdef0123456789abcdef" {
		t.Fatalf("incorrect tunnel ID %q", got)
	}
	if strings.Contains(got, "API_KEY") {
		t.Fatal("secret leaked")
	}
	if tunnelIDFromProfile(filepath.Join(dir, "missing.yaml")) != "" {
		t.Fatal("missing profile returned id")
	}
}
