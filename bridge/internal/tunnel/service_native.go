package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	kservice "github.com/kardianos/service"
)

type controlData struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
}

type workerStatus struct {
	Connecting *bool  `json:"connecting,omitempty"`
	Label      string `json:"label"`
	Revision   string `json:"revision"`
}

func localWorkerStatus(root, action string) (workerStatus, bool) {
	var result workerStatus
	b, e := os.ReadFile(filepath.Join(root, "runtime", "control.json"))
	if e != nil {
		return result, false
	}
	var c controlData
	if json.Unmarshal(b, &c) != nil || c.Port < 1024 || c.Port > 65535 || len(c.Token) < 32 {
		return result, false
	}
	method := "GET"
	if action == "stop" || action == "connect" || action == "disconnect" {
		method = "POST"
	}
	req, e := http.NewRequest(method, fmt.Sprintf("http://127.0.0.1:%d/%s", c.Port, action), nil)
	if e != nil {
		return result, false
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	timeout := 2 * time.Second
	if action == "disconnect" {
		timeout = 6 * time.Second
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, e := client.Do(req)
	if e != nil {
		return result, false
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return result, false
	}
	ok := json.NewDecoder(response.Body).Decode(&result) == nil && result.Label == "vidotter-mcp-"+strings.TrimPrefix(serviceName(root), "VidOtter-MCP-")
	return result, ok
}
func localControl(root, action string) bool {
	_, ok := localWorkerStatus(root, action)
	return ok
}
func connectionAction(root, action string) error {
	if action != "connect" && action != "disconnect" {
		return errors.New("invalid Tunnel action")
	}
	s := nativeState(root)
	if !s.Managed {
		return errors.New("start the local service first")
	}
	if action == "connect" {
		if s.RestartRequired {
			return errors.New("restart to apply saved configuration first")
		}
		if err := validateSavedConnection(root); err != nil {
			return err
		}
	}
	if !localControl(root, action) {
		return errors.New("Tunnel operation failed; restart the service if upgrading an older worker")
	}
	return nil
}
func tunnelHealth(root string) bool {
	setup, err := readSetup(root)
	if err != nil || setup.Port < 1024 || setup.Port > 65535 {
		return false
	}
	client := &http.Client{Timeout: 1500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/readyz", setup.Port))
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func launchRuntime(root string) error {
	// OS-released installation lock also covers a login racing with manual Start.
	lock, err := os.OpenFile(filepath.Join(root, "runtime", "worker.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := protectPrivate(lock.Name(), false); err != nil {
		return err
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		if err := lockWorker(lock); err == nil {
			break
		}
		if localControl(root, "status") {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("another worker owns this installation")
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !localControl(root, "status") {
		return runWorker(root)
	}
	return nil
}
func runDetached(root string) error {
	if localControl(root, "status") {
		return nil
	}
	executable, e := os.Executable()
	if e != nil {
		return e
	}
	command := exec.Command(executable, "--mcp-worker")
	command.Dir = root
	command.Env = append(os.Environ(), "PUPPETEER_SKIP_DOWNLOAD=true")
	// Detached worker retains its own log, never writes secrets to console.
	file, e := os.OpenFile(filepath.Join(root, "runtime", "tunnel.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer file.Close()
	command.Stdout = file
	command.Stderr = file
	configureDetached(command)
	if e = command.Start(); e != nil {
		return e
	}
	e = command.Process.Release()
	if e != nil {
		return e
	}
	for i := 0; i < 30; i++ {
		if localControl(root, "status") {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("worker did not become ready; inspect local log")
}
func stopWorker(root string) error {
	if !localControl(root, "status") {
		return nil
	}
	if !localControl(root, "stop") {
		return errors.New("could not stop owned worker")
	}
	for i := 0; i < 30; i++ {
		if !localControl(root, "status") {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("worker did not stop")
}

type daemon struct {
	root     string
	stopping chan struct{}
	done     chan struct{}
}

func (d *daemon) Start(s kservice.Service) error {
	// Keep a single OS-managed foreground process and retry transient child exits.
	d.stopping = make(chan struct{})
	d.done = make(chan struct{})
	go func() {
		defer close(d.done)
		for {
			select {
			case <-d.stopping:
				return
			default:
			}
			_ = launchRuntime(d.root)
			select {
			case <-d.stopping:
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()
	return nil
}
func (d *daemon) Stop(s kservice.Service) error {
	// Handle Stop racing with startup, without ever signalling an unrelated PID.
	close(d.stopping)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		_ = localControl(d.root, "stop")
		select {
		case <-d.done:
			return nil
		case <-time.After(100 * time.Millisecond):
		}
	}
	return errors.New("tunnel worker did not stop before deadline")
}

func serviceName(root string) string {
	digest := sha256.Sum256([]byte(root))
	return "VidOtter-MCP-" + hex.EncodeToString(digest[:5])
}
func daemonConfig(root, account, scope string) (*kservice.Config, error) {
	exe, e := os.Executable()
	if e != nil {
		return nil, e
	}
	config := &kservice.Config{
		Name:             serviceName(root),
		DisplayName:      "VidOtter MCP",
		Description:      "Authorized local MCP Tunnel runtime",
		Executable:       exe,
		WorkingDirectory: root,
		Arguments:        []string{"--mcp-native-service"},
		UserName:         account,
		Option:           kservice.KeyValue{"KeepAlive": true, "RunAtLoad": true, "UserService": scope == "user"},
	}
	if runtime.GOOS == "windows" && scope == "user" {
		return nil, errors.New("Windows native user services require a system-service registration under a restricted user account")
	}
	if scope != "user" && scope != "system" {
		return nil, errors.New("scope must be user or system")
	}
	// Account validation occurs during install, not when inspecting the service.
	return config, nil
}
func nativeService(root, account, scope string) (kservice.Service, error) {
	cfg, e := daemonConfig(root, account, scope)
	if e != nil {
		return nil, e
	}
	return kservice.New(&daemon{root: root}, cfg)
}
func currentAccount() string {
	c, e := user.Current()
	if e != nil {
		return ""
	}
	if runtime.GOOS == "windows" {
		return c.Username
	}
	return c.Username
}
func installNativeService(root, account, scope, password string) error {
	setup, e := readSetup(root)
	if e != nil {
		return errors.New("configure the tunnel before installing a service")
	}
	if setup.Manager != "go-client" {
		return errors.New("unsupported service manager")
	}
	if localControl(root, "status") {
		return errors.New("stop the existing worker before changing service manager")
	}
	if scope == "system" && (account == "" || strings.EqualFold(account, "root")) {
		return errors.New("system service requires an explicitly selected unprivileged account")
	}
	if scope == "system" && runtime.GOOS != "windows" {
		if err := verifyServiceDataOwner(root, account); err != nil {
			return err
		}
	}
	if enabled, err := systemStartup().status(root, setup); err != nil {
		return err
	} else if enabled {
		return errors.New("disable login startup before installing a native service")
	}
	cfg, e := daemonConfig(root, account, scope)
	if e != nil {
		return e
	}
	// The password is passed only in memory to the Windows service-control manager.
	if runtime.GOOS == "windows" {
		if password == "" {
			return errors.New("Windows service-account password required through hidden prompt or --password-stdin")
		}
		if strings.EqualFold(account, "LocalSystem") || strings.HasPrefix(strings.ToUpper(account), "NT AUTHORITY\\") {
			return errors.New("built-in and privileged service identities are not permitted")
		}
		if !strings.EqualFold(account, currentAccount()) {
			return errors.New("Windows service account must match the account that owns the private configuration")
		}
		cfg.Option["Password"] = password
	}
	s, e := kservice.New(&daemon{root: root}, cfg)
	if e != nil {
		return e
	}
	if e = writeNativeScope(root, scope, account); e != nil {
		return e
	}
	if e = s.Install(); e != nil {
		return fmt.Errorf("native service installation failed: %w", e)
	}
	setup, e = readSetup(root)
	if e != nil {
		return e
	}
	setup.Manager = "go-service"
	b, e := jsonMarshal(setup)
	if e != nil {
		return e
	}
	if e = writePrivate(filepath.Join(root, "runtime", "setup.json"), b); e != nil {
		_ = s.Uninstall()
		return e
	}
	return nil
}
func controlNative(root, action string) error {
	setup, e := readSetup(root)
	if e != nil {
		if action == "stop" && os.IsNotExist(e) {
			return nil
		}
		return errors.New("configure the tunnel first")
	}
	switch setup.Manager {
	case "go-service":
		// Control the installed native system/user service; no Bash or PowerShell.
		// Only the platform manager itself decides the effective account.
		s, e := nativeService(root, "", nativeScope(root))
		if e != nil {
			return e
		}
		if action == "start" {
			return systemStartup().manualServiceStart(root, setup, s.Start)
		}
		return s.Stop()
	case "go-client":
		if action == "start" {
			return runDetached(root)
		}
		return stopWorker(root)
	default:
		// Unknown managers are never executed.
		return errors.New("unsupported service manager")
	}
}
func nativeScope(root string) string {
	b, e := os.ReadFile(filepath.Join(root, "runtime", "service.json"))
	if e != nil {
		return "user"
	}
	var m struct {
		Scope string `json:"scope"`
	}
	if json.Unmarshal(b, &m) != nil {
		return "user"
	}
	return m.Scope
}
func writeNativeScope(root, scope, account string) error {
	b, e := jsonMarshal(map[string]string{"scope": scope})
	if e != nil {
		return e
	}
	file := filepath.Join(root, "runtime", "service.json")
	if e = writePrivate(file, b); e != nil {
		return e
	}
	if scope == "system" && runtime.GOOS != "windows" {
		return assignServiceOwner(file, account)
	}
	return nil
}
func nativeState(root string) state {
	out := state{AutoConnect: true}
	setup, e := readSetup(root)
	if e != nil {
		if os.IsNotExist(e) {
			enabled, err := systemStartup().status(root, persistedSetup{Manager: "go-client"})
			if err == nil {
				out.Autostart = &enabled
			}
		}
		return out
	}
	out.ID = setup.ID
	out.Revision = setup.Revision
	out.Manager = setup.Manager
	out.AutoConnect = setup.autoConnect()
	enabled, startupErr := systemStartup().status(root, setup)
	if startupErr == nil {
		out.Autostart = &enabled
	} else {
		out.Error = "Cannot read system startup registration"
	}
	_, keyErr := savedKey(root)
	out.HasKey = keyErr == nil
	if setup.Manager == "go-client" || setup.Manager == "go-service" {
		worker, alive := localWorkerStatus(root, "status")
		out.Managed = alive
		out.Connecting = alive && (worker.Connecting == nil || *worker.Connecting)
		out.RestartRequired = alive && setup.Revision != worker.Revision
		out.Ready = alive && out.Connecting && !out.RestartRequired && tunnelHealth(root)
		if setup.Manager == "go-service" {
			srv, e := nativeService(root, "", nativeScope(root))
			if e != nil {
				out.Error = "Native service backend unavailable"
				return out
			}
			status, e := srv.Status()
			if e != nil {
				out.Error = "Native service not installed"
				out.Autostart = nil
				return out
			}
			out.Managed = status == kservice.StatusRunning
			out.Ready = out.Managed && alive && out.Connecting && !out.RestartRequired && tunnelHealth(root)
		}
		return out
	}
	return unsupportedState(root, out)
}
func unsupportedState(root string, s state) state {
	s.Error = "Unsupported service manager"
	return s
}
func serveNative(root string) error {
	scope := nativeScope(root)
	// Configuration UserName does not matter while serving.
	srv, e := nativeService(root, "", scope)
	if e != nil {
		return e
	}
	return srv.Run()
}
func terminateNative(root string) error {
	setup, e := readSetup(root)
	if e != nil || setup.Manager != "go-service" {
		return errors.New("native service is not configured")
	}
	if localControl(root, "status") {
		return errors.New("stop the native service before uninstalling it")
	}
	srv, e := nativeService(root, "", nativeScope(root))
	if e != nil {
		return e
	}
	if e = srv.Uninstall(); e != nil {
		return e
	}
	setup.Manager = "go-client"
	raw, e := jsonMarshal(setup)
	if e != nil {
		return e
	}
	return writePrivate(filepath.Join(root, "runtime", "setup.json"), raw)
}
func waitUntilReady(root string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for {
		if localControl(root, "status") && tunnelHealth(root) {
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("tunnel not ready; check credentials and logs")
		case <-time.After(400 * time.Millisecond):
		}
	}
}
