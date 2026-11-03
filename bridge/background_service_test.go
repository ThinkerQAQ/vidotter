package vidotter

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBackgroundRequestsAreBounded(t *testing.T) {
	s := &bridgeServer{state: bridgeState{Token: "test-secret"}}
	for _, body := range []string{`{"action":"enable","target":"evil"}`, `{"action":"delete","target":"bridge"}`,
		`{"action":"enable","target":"bridge","command":"whoami"}`, `{"action":"enable","target":"bridge"}{}`} {
		req := httptest.NewRequest(http.MethodPost, "/v1/background", strings.NewReader(body))
		req.Header.Set("X-VidOtter-Token", "test-secret")
		out := httptest.NewRecorder()
		s.handleBackground(out, req)
		if out.Code != 400 {
			t.Fatalf("unbounded request accepted: %s, status %d", body, out.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/background", strings.NewReader(`{"action":"enable","target":"bridge"}`))
	out := httptest.NewRecorder()
	s.handleBackground(out, req)
	if out.Code != 401 {
		t.Fatal("background management requires authentication")
	}
	for _, platform := range []string{"windows", "linux", "darwin"} {
		for _, action := range []string{"status", "enable", "disable"} {
			if _, err := backgroundCommand(platform, t.TempDir(), action, "bridge"); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestDuplicateBridgeDoesNotTouchState(t *testing.T) {
	listener, err := net.Listen("tcp", bridgeAddress)
	if err != nil {
		t.Skip("Bridge port already occupied")
	}
	defer listener.Close()
	// Binding fails before random-token generation, job loading, recovery or persistence.
	if err := runBridge(); err == nil {
		t.Fatal("duplicate bridge accepted")
	}
}
