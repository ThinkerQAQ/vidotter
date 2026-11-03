package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestNativeManagerRequiresUnprivilegedAccount(t *testing.T) {
	root := t.TempDir()
	if err := saveNative(root, "tunnel_TEST", "synthetic-key"); err != nil {
		t.Fatal(err)
	}
	if err := installNativeService(root, "root", "system", ""); err == nil {
		t.Fatal("root service account accepted")
	}
	if runtime.GOOS == "windows" {
		if err := installNativeService(root, "NT AUTHORITY\\SYSTEM", "system", "synthetic-password"); err == nil {
			t.Fatal("SYSTEM accepted")
		}
	}
	raw, err := os.ReadFile(filepath.Join(root, "runtime", "setup.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved persistedSetup
	if err = json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	saved.Manager = "scheduled-task"
	payload, _ := jsonMarshal(saved)
	if err = writePrivate(filepath.Join(root, "runtime", "setup.json"), payload); err != nil {
		t.Fatal(err)
	}
	if err = installNativeService(root, "restricted-user", "system", "synthetic-password"); err == nil {
		t.Fatal("legacy manager must be cleaned before migration")
	}
}
func TestControlPlaneMustMatchExactInstallation(t *testing.T) {
	root := t.TempDir()
	token := "synthetic-local-control-token-is-32-chars-long"
	label := "vidotter-mcp-" + serviceName(root)[len("VidOtter-MCP-"):]
	expected := label
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "{\"label\":%q}", expected)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	raw, _ := jsonMarshal(controlData{Port: port, Token: token})
	if err := writePrivate(filepath.Join(root, "runtime", "control.json"), raw); err != nil {
		t.Fatal(err)
	}
	if !localControl(root, "status") || !localControl(root, "stop") {
		t.Fatal("authenticated local control failed")
	}
	expected = "vidotter-mcp-other"
	if localControl(root, "stop") {
		t.Fatal("control accepted another installation")
	}
}
func TestDaemonStopsWithoutConfiguredRuntime(t *testing.T) {
	d := &daemon{root: t.TempDir()}
	if err := d.Start(nil); err != nil {
		t.Fatal(err)
	}
	if err := d.Stop(nil); err != nil {
		t.Fatalf("failed to stop a freshly starting worker: %v", err)
	}
}
