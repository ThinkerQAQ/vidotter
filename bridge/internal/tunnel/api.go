package tunnel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

type state struct {
	Revision        string `json:"revision"`
	RestartRequired bool   `json:"restart_required"`
	ID              string `json:"id"`
	HasKey          bool   `json:"has_key"`
	Managed         bool   `json:"managed_process_running"`
	Ready           bool   `json:"tunnel_ready"`
	Manager         string `json:"manager"`
	Autostart       *bool  `json:"autostart_enabled"`
	AutoConnect     bool   `json:"tunnel_auto_connect"`
	Connecting      bool   `json:"tunnel_connecting"`
	Error           string `json:"error,omitempty"`
}

// State contains only local status and identifiers, never credentials.
type State = state

var tunnelIDPattern = regexp.MustCompile(`^tunnel_[A-Za-z0-9_-]+$`)
var actionMu sync.Mutex

func sessionToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func tunnel(root string) string {
	name := "tunnel-client"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	// Prefer the verified client delivered with this installation, then the existing user installation.
	for _, candidate := range []string{filepath.Join(filepath.Dir(filepath.Dir(root)), "bin", name), filepath.Join(userHome(), ".local", "bin", name)} {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found
	}
	return filepath.Join(userHome(), ".local", "bin", name)
}
func userHome() string         { home, _ := os.UserHomeDir(); return home }
func Status(root string) State { actionMu.Lock(); defer actionMu.Unlock(); return nativeState(root) }
func Save(root, id, key string, startup, connect *bool) error {
	if !actionMu.TryLock() {
		return errors.New("another MCP operation is in progress")
	}
	defer actionMu.Unlock()
	release, err := configurationLock(root)
	if err != nil {
		return err
	}
	defer release()
	return savePreferences(root, id, key, startup, connect)
}
func save(root, id, key string) error { return saveNative(root, id, key) }
func readState(root string) state     { return nativeState(root) }
func service(root, action string) error {
	if action != "start" && action != "stop" && action != "restart" {
		return errors.New("invalid service action")
	}
	if action != "stop" {
		if err := validateSavedConnection(root); err != nil {
			return err
		}
	}
	if action == "restart" {
		if !nativeState(root).Managed {
			return errors.New("start the service before restarting it")
		}
		if err := controlNative(root, "stop"); err != nil {
			return err
		}
		deadline := time.Now().Add(10 * time.Second)
		for localControl(root, "status") {
			if time.Now().After(deadline) {
				return errors.New("service has not stopped; restart cancelled")
			}
			time.Sleep(100 * time.Millisecond)
		}
		return controlNative(root, "start")
	}
	return controlNative(root, action)
}

func Service(root, action string) error {
	if !actionMu.TryLock() {
		return errors.New("another MCP operation is in progress")
	}
	defer actionMu.Unlock()
	if action == "start" || action == "restart" || action == "connect" {
		if err := RequireClient(root); err != nil {
			return err
		}
	}
	if action == "connect" || action == "disconnect" {
		return connectionAction(root, action)
	}
	return service(root, action)
}
func RunWorker(root string) error        { return launchRuntime(root) }
func RunNativeService(root string) error { return serveNative(root) }
func InstallService(root, account, scope, password string) error {
	return installNativeService(root, account, scope, password)
}
func UninstallService(root string) error { return terminateNative(root) }
func Startup(root string, enable bool) error {
	s, err := readSetup(root)
	if err != nil {
		return err
	}
	return Save(root, s.ID, "", &enable, nil)
}
func Lifecycle(root, action string) error {
	s, err := readSetup(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	switch action {
	case "restore":
		if s.Autostart != nil {
			return systemStartup().apply(root, s, *s.Autostart)
		}
		return nil
	case "uninstall":
		enabled, err := systemStartup().status(root, s)
		if err != nil {
			return err
		}
		s.Autostart = &enabled
		raw, _ := json.Marshal(s)
		if err = writePrivate(filepath.Join(root, "runtime", "setup.json"), raw); err != nil {
			return err
		}
		if err = systemStartup().apply(root, s, false); err != nil {
			return err
		}
		if err = controlNative(root, "stop"); err != nil {
			return err
		}
		if s.Manager == "go-service" {
			return terminateNative(root)
		}
		return systemStartup().remove(root, s)
	default:
		return errors.New("unsupported MCP lifecycle action")
	}
}
