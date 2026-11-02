package vidotter

import (
	"context"
	"encoding/json"
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
	win, err := mcpSetupCommand("windows", root, validID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(win.Args[len(win.Args)-5:], []string{"-File", filepath.Join(scripts, "Setup-VidOtter-MCP.ps1"), "-TunnelId", validID, "-KeyFromStdin"}) {
		t.Fatalf("unexpected Windows setup args: %#v", win.Args)
	}
	replaced, err := mcpSetupCommand("windows", root,
		"tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true, validID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replaced.Args[len(replaced.Args)-3:], []string{
		"-ReplaceExisting", "-ExpectedTunnelId", validID}) {
		t.Fatalf("missing explicit Windows replacement arguments: %#v", replaced.Args)
	}
	if _, err := mcpSetupCommand("windows", root, validID, true, validID); err != nil {
		t.Fatal("same ID must support key-only rotation")
	}
	if strings.Contains(strings.Join(win.Args, " "), "sk-") {
		t.Fatal("setup must never receive credentials via its process arguments")
	}
	mac, err := mcpSetupCommand("darwin", root, validID, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(mac.Args[0]) != "bash" || !strings.Contains(mac.Args[len(mac.Args)-1], "setup-mcp.sh") {
		t.Fatalf("unexpected macOS setup command: %#v", mac.Args)
	}
	if _, err := mcpSetupCommand(runtime.GOOS, filepath.Join(root, "missing"), validID, false, ""); err == nil {
		t.Fatal("must reject missing scripts")
	}
}

func TestMCPSaveOnlyCommandIsolation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{"Setup-VidOtter-MCP.ps1", "setup-mcp.sh"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", script), []byte("# test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	id := "tunnel_0123456789abcdef0123456789abcdef"
	win, err := mcpSaveCommand("windows", root, id, true, id, true, true)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Join(win.Args, " ")
	for _, flag := range []string{"-SaveOnly", "-ReuseSavedKey", "-ReplaceExisting", "-KeyFromStdin"} {
		if !strings.Contains(args, flag) {
			t.Errorf("missing option %s: %s", flag, args)
		}
	}
	if strings.Contains(args, "FAKE_RUNTIME_KEY") {
		t.Fatal("secret must never be a command argument")
	}
	unix, err := mcpSaveCommand("linux", root, id, true, id, true, true)
	if err != nil {
		t.Fatal(err)
	}
	env := strings.Join(unix.Env, "\n")
	for _, variable := range []string{"VIDOTTER_MCP_SAVE_ONLY=1", "VIDOTTER_MCP_REUSE_KEY=1", "VIDOTTER_REPLACE_TUNNEL=1"} {
		if !strings.Contains(env, variable) {
			t.Errorf("missing environment option %s", variable)
		}
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
	if len(tool.Actions) != 2 || tool.Actions[0].ID != "setup" || tool.Actions[1].ID != "install-client" {
		t.Fatalf("unexpected MCP controls: %#v", tool.Actions)
	}
	if len(tool.Config.Schema) != 0 {
		t.Fatal("no Tunnel ID or runtime keys may enter the shared extension settings")
	}
	if strings.Contains(tool.Health.Summary, "连接正常") {
		t.Fatal("profile presence is not proof the Tunnel is connected")
	}
}

func TestMCPProfileRequiresValidTunnelID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	profile := mcpProfilePath(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME"))
	if err := os.MkdirAll(filepath.Dir(profile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("tunnel_id: invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid := chatGPTMCPTool{}.Snapshot(context.Background(), defaultBridgeConfig())
	if invalid.Config.Values["configured"] != false || invalid.Config.Values["tunnelId"] != "" {
		t.Fatalf("invalid profile must not count as configured: %#v", invalid.Config.Values)
	}
	id := "tunnel_0123456789abcdef0123456789abcdef"
	if err := os.WriteFile(profile, []byte("tunnel_id: "+id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	valid := chatGPTMCPTool{}.Snapshot(context.Background(), defaultBridgeConfig())
	if valid.Config.Values["configured"] != true || valid.Config.Values["tunnelId"] != id {
		t.Fatalf("valid profile should be detected: %#v", valid.Config.Values)
	}
}

func TestMCPSetupRequiresExplicitAndFreshTunnelReplacement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "local"))
	profile := mcpProfilePath(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_CONFIG_HOME"))
	if err := os.MkdirAll(filepath.Dir(profile), 0o700); err != nil {
		t.Fatal(err)
	}
	oldID := "tunnel_0123456789abcdef0123456789abcdef"
	nextID := "tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := os.WriteFile(profile, []byte("tunnel_id: "+oldID+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := &bridgeServer{state: bridgeState{Token: "test-secret"}}
	cases := []string{
		`{"tunnelId":"tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`,
		`{"tunnelId":"tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","replace":true}`,
		`{"tunnelId":"tunnel_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","replace":true,"previousTunnelId":"tunnel_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}`,
		`{"tunnelId":"tunnel_0123456789abcdef0123456789abcdef"}`,
	}
	for _, body := range cases {
		request := httptest.NewRequest(http.MethodPost, "/v1/mcp/setup", strings.NewReader(body))
		request.Header.Set("X-VidOtter-Token", "test-secret")
		recorder := httptest.NewRecorder()
		server.handleMCPSetup(recorder, request)
		if recorder.Code != http.StatusConflict {
			t.Fatalf("replacement must reject unconfirmed or stale IDs: status=%d body=%s", recorder.Code, recorder.Body.String())
		}
	}
	if got := tunnelIDFromProfile(profile); got != oldID {
		t.Fatalf("old profile modified: %s", got)
	}
	_ = nextID
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
		`{"tunnelId":"tunnel_0123456789abcdef0123456789abcdef"}{"tunnelId":"tunnel_0123456789abcdef0123456789abcdef"}`,
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

func TestMCPSetupCommandAllowsKeyOnlyRotation(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "Setup-VidOtter-MCP.ps1"), []byte("# test"), 0600); err != nil {
		t.Fatal(err)
	}
	id := "tunnel_0123456789abcdef0123456789abcdef"
	if _, err := mcpSetupCommand("windows", root, id, true, id); err != nil {
		t.Fatalf("key-only rotation rejected: %v", err)
	}
}

func TestMCPSetupValidatesPasswordWithoutEchoingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("PATH", home)
	s := &bridgeServer{state: bridgeState{Token: "test-token"}}
	for _, key := range []string{"", "short", "FAKE_VALID_KEY\nINJECTED", strings.Repeat("X", 513), "FAKE_VALID_KEY_FOR_FORM_123"} {
		data, _ := json.Marshal(map[string]any{"tunnelId": "tunnel_0123456789abcdef0123456789abcdef", "runtimeApiKey": key})
		r := httptest.NewRequest(http.MethodPost, "/v1/mcp/setup", strings.NewReader(string(data)))
		r.Header.Set("X-VidOtter-Token", "test-token")
		w := httptest.NewRecorder()
		s.handleMCPSetup(w, r)
		expected := http.StatusBadRequest
		if key == "FAKE_VALID_KEY_FOR_FORM_123" {
			expected = http.StatusConflict
		} // Valid password reached the missing-client check.
		if w.Code != expected {
			t.Fatalf("unexpected credential validation status: %d", w.Code)
		}
		if key != "" && strings.Contains(w.Body.String(), key) {
			t.Fatal("credential leaked into response")
		}
	}
}
