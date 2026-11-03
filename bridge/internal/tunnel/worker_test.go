package tunnel

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A synthetic tunnel subprocess exercises the real supervisor without any
// account, network service, browser, API key or production configuration.
func TestTunnelChildHelper(t *testing.T) {
	config := ""
	for i, arg := range os.Args {
		if arg == "--config" && i+1 < len(os.Args) {
			config = os.Args[i+1]
		}
	}
	if config == "" {
		t.Skip("synthetic subprocess only")
	}
	raw, err := os.ReadFile(config)
	if err != nil {
		os.Exit(31)
	}
	var settings struct {
		Control struct {
			ID string `json:"tunnel_id"`
		} `json:"control_plane"`
		Health struct {
			Address string `json:"listen_addr"`
		} `json:"health"`
	}
	if json.Unmarshal(raw, &settings) != nil {
		os.Exit(32)
	}
	hash := sha256.Sum256([]byte(os.Getenv("OPENAI_API_KEY")))
	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/identity", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"id": settings.Control.ID, "keyHash": hex.EncodeToString(hash[:])})
	})
	if http.ListenAndServe(settings.Health.Address, mux) != nil {
		os.Exit(33)
	}
	os.Exit(0)
}
func waitWorker(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("worker state did not converge")
}
func TestWorkerStandbyReconnectAndStartupSnapshot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Data", "mcp")
	t.Setenv("VIDOTTER_TUNNEL_TEST_HELPER", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(filepath.Dir(filepath.Dir(root)), "bin", "tunnel-client")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, binary, 0700); err != nil {
		t.Fatal(err)
	}
	if err := saveNative(root, "tunnel_FIRST", "synthetic-first-key"); err != nil {
		t.Fatal(err)
	}
	setup, _ := readSetup(root)
	setup.AutoConnect = new(bool)
	setup.Tunnel = target
	setup.TunnelArgs = []string{"-test.run=^TestTunnelChildHelper$", "--"}
	raw, _ := json.Marshal(setup)
	if err := writePrivate(filepath.Join(root, "runtime", "setup.json"), raw); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- launchRuntime(root) }()
	defer func() {
		localControl(root, "stop")
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("worker leaked")
		}
	}()
	waitWorker(t, func() bool {
		state, ok := localWorkerStatus(root, "status")
		return ok && state.Connecting != nil && !*state.Connecting
	})
	// A second login/manual start cannot create another worker for this install.
	if err := launchRuntime(root); err != nil {
		t.Fatal(err)
	}
	if !localControl(root, "connect") {
		t.Fatal("connect failed")
	}
	identity := func() map[string]string {
		client := http.Client{Timeout: time.Second}
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/identity", setup.Port))
		if err != nil {
			return nil
		}
		defer response.Body.Close()
		var result map[string]string
		_ = json.NewDecoder(response.Body).Decode(&result)
		return result
	}
	waitWorker(t, func() bool { return identity()["id"] == "tunnel_FIRST" })
	// Saving rotation does not change the active profile or secret on reconnect.
	// Use the worker control directly: public Connect blocks unapplied revisions.
	if err := saveNative(root, "tunnel_SECOND", "synthetic-second-key"); err != nil {
		t.Fatal(err)
	}
	if !localControl(root, "disconnect") {
		t.Fatal("disconnect failed")
	}
	state, ok := localWorkerStatus(root, "status")
	if !ok || state.Connecting == nil || *state.Connecting {
		t.Fatal("disconnect stopped local service")
	}
	if !localControl(root, "connect") {
		t.Fatal("worker reconnect failed")
	}
	waitWorker(t, func() bool { return identity()["id"] == "tunnel_FIRST" })
	expected := sha256.Sum256([]byte("synthetic-first-key"))
	if identity()["keyHash"] != hex.EncodeToString(expected[:]) {
		t.Fatal("reconnect adopted an unsafely rotated credential")
	}
	// Neither status nor control records may contain secrets.
	statusRaw, _ := json.Marshal(state)
	if strings.Contains(string(statusRaw), "synthetic-") {
		t.Fatal("secret in worker state")
	}
	if !localControl(root, "stop") {
		t.Fatal("stop failed")
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	// Put a completion back for the deferred cleanup wait.
	finished <- nil
}
