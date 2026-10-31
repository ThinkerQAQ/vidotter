package vidotter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBrowserAutomationDefaultDeniedAndLocallyControlled(t *testing.T) {
	config := defaultBridgeConfig()
	if config.BrowserAutomationEnabled || config.BrowserInteractionEnabled {
		t.Fatal("browser automation must be opt-in")
	}
	f := configToFile(config)
	if f.BrowserAutomation.Enabled || f.BrowserAutomation.Interactive {
		t.Fatal("default TOML must deny automation")
	}
	config.BrowserAutomationEnabled = true
	config.BrowserInteractionEnabled = true
	f = configToFile(config)
	roundtrip := configFromFile(f)
	if !roundtrip.BrowserAutomationEnabled || !roundtrip.BrowserInteractionEnabled {
		t.Fatal("local flags not persisted")
	}
	roundtrip.BrowserAutomationEnabled = false
	normal, err := normalizeBridgeConfig(roundtrip)
	if err != nil || normal.BrowserInteractionEnabled {
		t.Fatal("disable must revoke click permission")
	}

	server := &bridgeServer{state: bridgeState{Token: "private-local-bridge-token"}, config: defaultBridgeConfig()}
	request := func(method, path string, body string, auth bool) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		if auth {
			r.Header.Set("X-VidOtter-Token", "private-local-bridge-token")
		}
		w := httptest.NewRecorder()
		switch path {
		case "/v1/browser/automation":
			server.handleBrowserAutomation(w, r)
		default:
			server.handleBrowserAutomationCall(w, r)
		}
		return w
	}
	if w := request("GET", "/v1/browser/automation", "", false); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous tool registry %d", w.Code)
	}
	if w := request("POST", "/v1/browser/automation/call", `{"Name":"browser_open","URL":"https://example.com"}`, false); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous browser tool executed %d", w.Code)
	}
	if w := request("POST", "/v1/browser/automation/call", `{"Name":"browser_open","URL":"https://example.com"}`, true); w.Code != http.StatusConflict {
		t.Fatalf("browser tool must be opt-in, got %d", w.Code)
	}
	if w := request("POST", "/v1/browser/automation", `{"Action":"enable-interaction"}`, true); w.Code != http.StatusConflict {
		t.Fatalf("cannot enable clicking without installed+enabled browser component, got %d", w.Code)
	}
	if w := request("GET", "/v1/browser/automation", "", true); w.Code != http.StatusOK {
		t.Fatalf("component status %d", w.Code)
	} else {
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out["enabled"] != false || out["interactive"] != false {
			t.Fatal("component was unexpectedly enabled")
		}
	}
}
func TestBrowserAutomationURLIsPublicAndToolWhitelisted(t *testing.T) {
	for _, raw := range []string{"http://127.0.0.1/private", "file:///etc/hosts", "https://localhost/admin", "https://example.com:8443/"} {
		if err := validateMCPMediaURL(context.Background(), raw); err == nil {
			t.Errorf("private or unusual destination allowed: %q", raw)
		}
	}
	if _, err := (&browserAutomationProxy{}).call(context.Background(), browserAutomationCall{Name: "browser_unrestricted_js"}); err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatal("unlisted browser operations must be blocked before any child starts")
	}
	if _, err := (&browserAutomationProxy{}).call(context.Background(), browserAutomationCall{Name: "browser_open", URL: "http://localhost/admin"}); err == nil {
		t.Fatal("private URL must be rejected before starting the browser component")
	}
}

// Exercise the VidOtter-to-MCP adapter against a local in-memory tool server.
// This does not start another browser/CDP connection or trigger user consent.
func TestBrowserAutomationProxyForwardsMCPWithoutCDP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-browser", Version: "1"}, nil)
	type closeParams struct {
		PageID int `json:"pageId"`
	}
	mcp.AddTool(server, &mcp.Tool{Name: "close_page", Description: "Close an owned tab"},
		func(_ context.Context, _ *mcp.CallToolRequest, in closeParams) (*mcp.CallToolResult, struct{}, error) {
			if in.PageID != 37 {
				t.Errorf("unexpected page ID %d", in.PageID)
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "closed owned test page"}}}, struct{}{}, nil
		})
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-vidotter", Version: "1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	proxy := &browserAutomationProxy{session: clientSession}
	defer proxy.close()
	out, err := proxy.call(ctx, browserAutomationCall{Name: "browser_close", PageID: 37})
	if err != nil || !strings.Contains(out, "closed owned test page") {
		t.Fatalf("browser MCP adapter failed without CDP: out=%q err=%v", out, err)
	}
}

func TestBrowserAutomationProxyCooldownAvoidsRepeatedBrowserConnection(t *testing.T) {
	proxy := &browserAutomationProxy{reconnectAfter: time.Now().Add(browserAutomationCooldown)}
	out, err := proxy.call(context.Background(), browserAutomationCall{Name: "browser_status"})
	if out != "" || err == nil || !strings.Contains(err.Error(), "cooldown") {
		t.Fatalf("failed proxy must not reconnect immediately, out=%q err=%v", out, err)
	}
}
