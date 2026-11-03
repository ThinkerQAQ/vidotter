package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestIdenticalSavePreservesRevisionAndMaskIsRejected(t *testing.T) {
	root := t.TempDir()
	if err := save(root, "tunnel_valid", "synthetic-runtime-key"); err != nil {
		t.Fatal(err)
	}
	first, _ := readSetup(root)
	for _, key := range []string{"", "synthetic-runtime-key"} {
		if err := save(root, "tunnel_valid", key); err != nil {
			t.Fatal(err)
		}
		after, _ := readSetup(root)
		if first.Revision == "" || after.Revision != first.Revision {
			t.Fatal("unchanged save changed revision")
		}
	}
	if err := save(root, "tunnel_valid", "****************"); err == nil {
		t.Fatal("mask accepted as secret")
	}
	if err := save(root, "tunnel_valid", "new-synthetic-runtime-key"); err != nil {
		t.Fatal(err)
	}
	after, _ := readSetup(root)
	if after.Revision == first.Revision {
		t.Fatal("key rotation did not change revision")
	}
}
func TestReadinessTracksRunningRevisionWithoutExposingKey(t *testing.T) {
	root := t.TempDir()
	if err := save(root, "tunnel_valid", "synthetic-runtime-key"); err != nil {
		t.Fatal(err)
	}
	setup, _ := readSetup(root)
	active := setup.Revision
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(200)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 32) {
			w.WriteHeader(403)
			return
		}
		_ = json.NewEncoder(w).Encode(workerStatus{Label: "vidotter-mcp-" + strings.TrimPrefix(serviceName(root), "VidOtter-MCP-"), Revision: active})
	}))
	defer server.Close()
	parsed, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(parsed.Port())
	setup.Port = port
	raw, _ := json.Marshal(setup)
	if err := writePrivate(filepath.Join(root, "runtime", "setup.json"), raw); err != nil {
		t.Fatal(err)
	}
	control, _ := json.Marshal(controlData{Port: port, Token: strings.Repeat("t", 32)})
	if err := writePrivate(filepath.Join(root, "runtime", "control.json"), control); err != nil {
		t.Fatal(err)
	}
	if s := readState(root); !s.Managed || !s.Ready || s.RestartRequired {
		t.Fatalf("initial state: %+v", s)
	}
	if err := save(root, "tunnel_updated", ""); err != nil {
		t.Fatal(err)
	}
	if s := readState(root); !s.Managed || s.Ready || !s.RestartRequired {
		t.Fatalf("stale worker marked ready: %+v", s)
	}
	setup, _ = readSetup(root)
	active = setup.Revision
	state := readState(root)
	if !state.Ready || state.RestartRequired {
		t.Fatalf("new revision not ready: %+v", state)
	}
	raw, _ = json.Marshal(state)
	if strings.Contains(string(raw), "synthetic-runtime-key") {
		t.Fatal("key exposed to UI")
	}
	if err := os.WriteFile(filepath.Join(root, "runtime", "credentials.json"), []byte(`{broken`), 0600); err != nil {
		t.Fatal(err)
	}
	if readState(root).HasKey {
		t.Fatal("corrupt credential reported as usable")
	}
	if err := service(root, "start"); err == nil {
		t.Fatal("started with corrupt credentials")
	}
}
func TestRestartRequiresSavedConfigurationAndRunningService(t *testing.T) {
	root := t.TempDir()
	if service(root, "restart") == nil {
		t.Fatal("restart without configuration accepted")
	}
	if err := save(root, "tunnel_valid", "synthetic-runtime-key"); err != nil {
		t.Fatal(err)
	}
	if service(root, "restart") == nil {
		t.Fatal("restart of stopped service accepted")
	}
}
