package vidotter

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTunnelServiceActionsAreFixed(t *testing.T) {
	for _, action := range []string{"start", "stop", "restart", "connect", "disconnect"} {
		if !validMCPServiceAction(action) {
			t.Fatal(action)
		}
	}
	for _, action := range []string{"", "delete", "start && evil", "connect;evil"} {
		if validMCPServiceAction(action) {
			t.Fatal(action)
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
