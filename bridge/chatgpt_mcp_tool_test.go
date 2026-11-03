package vidotter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

func TestMCPSetupValidatesPasswordWithoutEchoingIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("LOCALAPPDATA", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("PATH", home)
	s := &bridgeServer{state: bridgeState{Token: "test-token"}}
	for _, key := range []string{"", "short", "FAKE_VALID_KEY\nINJECTED", strings.Repeat("X", 4097), "FAKE_VALID_KEY_FOR_FORM_123"} {
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
