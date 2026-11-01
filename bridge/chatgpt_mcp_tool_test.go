package vidotter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestMCPProfilePaths(t *testing.T) {
	for _, tc := range []struct {
		goos, home, local, xdg, expected string
	}{
		{"windows", "/home/example", "C:/LocalAppData", "", filepath.Join("C:/LocalAppData", "VidOtter", "MCP", "vidotter.yaml")},
		{"linux", "/home/example", "", "", filepath.Join("/home/example", ".config", "tunnel-client", "vidotter.yaml")},
		{"linux", "/home/example", "", "/custom/config", filepath.Join("/custom/config", "tunnel-client", "vidotter.yaml")},
		{"darwin", "/Users/example", "", "", filepath.Join("/Users/example", ".config", "tunnel-client", "vidotter.yaml")},
	} {
		if got := mcpProfilePath(tc.goos, tc.home, tc.local, tc.xdg); got != tc.expected {
			t.Errorf("%s profile=%q, want %q", tc.goos, got, tc.expected)
		}
	}
}

func TestMCPSetupLaunchesOnlyBundledScripts(t *testing.T) {
	root := t.TempDir()
	scripts := filepath.Join(root, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"Setup-VidOtter-MCP.ps1", "VidOtter-FirstRun.ps1", "setup-mcp.sh"} {
		if err := os.WriteFile(filepath.Join(scripts, file), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	validID := "tunnel_0123456789abcdef0123456789abcdef"
	win, err := mcpSetupCommand("windows", root, validID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(win.Args[len(win.Args)-4:], []string{"-File", filepath.Join(scripts, "Setup-VidOtter-MCP.ps1"), "-TunnelId", validID}) {
		t.Fatalf("unexpected Windows setup args: %#v", win.Args)
	}
	if strings.Contains(strings.Join(win.Args, " "), "sk-") {
		t.Fatal("setup must never receive credentials via its process arguments")
	}
	mac, err := mcpSetupCommand("darwin", root, validID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(mac.Args[0]) != "osascript" || !strings.Contains(mac.Args[len(mac.Args)-1], "setup-mcp.sh") {
		t.Fatalf("unexpected macOS setup command: %#v", mac.Args)
	}
	if _, err := mcpSetupCommand(runtime.GOOS, filepath.Join(root, "missing"), validID); err == nil {
		t.Fatal("must reject missing scripts")
	}
}

func TestTunnelIDValidation(t *testing.T) {
	good := []string{"tunnel_0123456789abcdef0123456789abcdef",
		"tunnel_ab12_0123456789abcdef0123456789abcdef"}
	for _, id := range good {
		if !validTunnelID(id) {
			t.Fatalf("unexpected invalid Tunnel ID")
		}
	}
	bad := []string{"", "tunnel_", "tunnel_abc", "tunnel_0123456789ABCDEF0123456789abcdef",
		"tunnel_0123456789abcdef0123456789abcdef;curl",
		"tunnel_abcd_0123456789abcdef0123456789abcdef_more"}
	for _, id := range bad {
		if validTunnelID(id) {
			t.Fatalf("unsafe ID accepted: %q", id)
		}
	}
}

func TestChatGPTMCPRegistryIsOptionalAndNoSecretData(t *testing.T) {
	tool := chatGPTMCPTool{}.Snapshot(context.Background(), defaultBridgeConfig())
	if tool.Kind != "integration" || tool.Required {
		t.Fatalf("ChatGPT integration must be optional: %#v", tool)
	}
	if len(tool.Actions) != 1 || tool.Actions[0].ID != "setup" {
		t.Fatalf("unexpected MCP controls: %#v", tool.Actions)
	}
	if len(tool.Config.Schema) != 0 {
		t.Fatal("no Tunnel ID or runtime keys may enter the shared extension settings")
	}
	if strings.Contains(tool.Health.Summary, "连接正常") {
		t.Fatal("profile presence is not proof the Tunnel is connected")
	}
}

func TestMCPSetupRejectsUnauthorizedRequests(t *testing.T) {
	server := &bridgeServer{state: bridgeState{Token: "test-secret"}}
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		request := httptest.NewRequest(method, "/v1/mcp/setup", nil)
		recorder := httptest.NewRecorder()
		server.handleMCPSetup(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("method=%s got=%d", method, recorder.Code)
		}
	}
	cases := []string{
		`{}`,
		`{"tunnelId":"tunnel_"}`,
		`{"tunnelId":"tunnel_0123456789abcdef0123456789abcdef;evil"}`,
		`{"tunnelId":"tunnel_0123456789abcdef0123456789abcdef","apiKey":"sk-never"}`,
	}
	for _, body := range cases {
		request := httptest.NewRequest(http.MethodPost, "/v1/mcp/setup", strings.NewReader(body))
		request.Header.Set("X-VidOtter-Token", "test-secret")
		recorder := httptest.NewRecorder()
		server.handleMCPSetup(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("invalid body accepted: status=%d", recorder.Code)
		}
		if strings.Contains(recorder.Body.String(), "sk-never") {
			t.Fatal("secret must never appear in response")
		}
	}
}
