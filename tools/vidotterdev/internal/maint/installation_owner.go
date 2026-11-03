package maint

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// A user's Native Messaging registration and login tasks are singletons.
// Never silently make an installation in a second directory own them.
func sameInstallRoot(a, b, platform string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if platform == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func checkInstallOwnership(root string, registered []string, platform string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	for _, other := range registered {
		if strings.TrimSpace(other) == "" {
			continue
		}
		if !sameInstallRoot(root, other, platform) {
			return fmt.Errorf("VidOtter is already registered at %q; uninstall that installation before using %q (no registrations were changed)", other, root)
		}
	}
	return nil
}

var registryStringValue = regexp.MustCompile(`(?i)\bREG_(?:SZ|EXPAND_SZ)\s+([^\r\n]+)`)

func registeredDefaultValue(key string) (string, error) {
	out, err := exec.Command("reg.exe", "query", key, "/ve").CombinedOutput()
	if err != nil {
		// Missing keys are normal on first installation.
		if strings.Contains(strings.ToLower(string(out)), "unable to find") ||
			strings.Contains(strings.ToLower(string(out)), "cannot find") {
			return "", nil
		}
		return "", fmt.Errorf("read existing VidOtter registration %s: %w", key, err)
	}
	match := registryStringValue.FindSubmatch(out)
	if len(match) != 2 {
		return "", errors.New("existing VidOtter registry entry has no readable path")
	}
	return strings.TrimSpace(string(match[1])), nil
}

type scheduledTaskXML struct {
	Actions struct {
		Exec struct {
			WorkingDirectory string `xml:"WorkingDirectory"`
		} `xml:"Exec"`
	} `xml:"Actions"`
}

func registeredWindowsTaskRoot(task string) (string, error) {
	out, err := exec.Command("schtasks.exe", "/Query", "/TN", task, "/XML").CombinedOutput()
	if err != nil {
		// Task Scheduler uses status 1 for a missing task. The code below
		// intentionally does not modify, disable, or recreate any task.
		if strings.Contains(strings.ToLower(string(out)), "cannot find") ||
			strings.Contains(strings.ToLower(string(out)), "does not exist") ||
			strings.Contains(strings.ToLower(string(out)), "cannot find the file") {
			return "", nil
		}
		return "", fmt.Errorf("inspect scheduled task %s: %w", task, err)
	}
	owner, err := parseScheduledTaskOwner(out)
	if err != nil {
		return "", fmt.Errorf("read ownership for scheduled task %s: %w", task, err)
	}
	return owner, nil
}

// schtasks.exe emits UTF-8 text but labels its XML UTF-16, even on Windows.
// Strip that incorrect declaration before parsing the task's owner.
func parseScheduledTaskOwner(out []byte) (string, error) {
	trimmed := bytes.TrimSpace(out)
	if bytes.HasPrefix(trimmed, []byte("<?xml")) {
		end := bytes.Index(trimmed, []byte("?>"))
		if end < 0 {
			return "", errors.New("incomplete scheduled task XML header")
		}
		trimmed = trimmed[end+2:]
	}
	var value scheduledTaskXML
	if err := xml.Unmarshal(trimmed, &value); err != nil {
		return "", err
	}
	if value.Actions.Exec.WorkingDirectory == "" {
		return "", errors.New("scheduled task has no owner working directory")
	}
	return value.Actions.Exec.WorkingDirectory, nil
}

func installedRootsFromWindows() ([]string, error) {
	var roots []string
	for _, browser := range []string{`HKCU\Software\Google\Chrome\NativeMessagingHosts\com.vidotter.bridge`, `HKCU\Software\Microsoft\Edge\NativeMessagingHosts\com.vidotter.bridge`} {
		value, err := registeredDefaultValue(browser)
		if err != nil {
			return nil, err
		}
		if value != "" {
			roots = append(roots, filepath.Dir(filepath.Dir(filepath.Dir(value))))
		}
	}
	protocol, err := registeredDefaultValue(`HKCU\Software\Classes\vidotter\shell\open\command`)
	if err != nil {
		return nil, err
	}
	if protocol != "" {
		binary := ""
		if strings.HasPrefix(protocol, `"`) {
			if end := strings.Index(protocol[1:], `"`); end >= 0 {
				binary = protocol[1 : end+1]
			}
		} else if fields := strings.Fields(protocol); len(fields) > 0 {
			binary = fields[0]
		}
		if binary == "" {
			return nil, errors.New("existing VidOtter URL protocol has an invalid command")
		}
		roots = append(roots, filepath.Dir(filepath.Dir(binary)))
	}
	for _, name := range []string{"VidOtter-bridge", "VidOtter-tunnel"} {
		owner, err := registeredWindowsTaskRoot(name)
		if err != nil {
			return nil, err
		}
		if owner != "" {
			roots = append(roots, owner)
		}
	}
	return roots, nil
}

func installedRootsFromNativeManifests(home, platform string) ([]string, error) {
	roots := []string{}
	for _, folder := range nativeDirs(home) {
		file := filepath.Join(folder, hostName+".json")
		data, err := os.ReadFile(file)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var manifest nativeHost
		if err := json.Unmarshal(data, &manifest); err != nil || manifest.Path == "" {
			return nil, fmt.Errorf("existing Native Messaging manifest is invalid: %s", file)
		}
		if manifest.Name != hostName {
			return nil, fmt.Errorf("unexpected Native Messaging host name at %s", file)
		}
		roots = append(roots, filepath.Dir(filepath.Dir(manifest.Path)))
	}
	return roots, nil
}

func registeredInstallRoots() ([]string, error) {
	switch runtime.GOOS {
	case "windows":
		return installedRootsFromWindows()
	case "darwin", "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		return installedRootsFromNativeManifests(home, runtime.GOOS)
	}
	return nil, errors.New("unsupported platform")
}

func assertInstallOwnership(root string) error {
	roots, err := registeredInstallRoots()
	if err != nil {
		return err
	}
	return checkInstallOwnership(root, roots, runtime.GOOS)
}

func manageBackgroundLifecycle(root, action string) error {
	script := filepath.Join(root, "scripts", "manage-background.sh")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		script = filepath.Join(root, "scripts", "Manage-VidOtter-Background.ps1")
		cmd = exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass",
			"-File", script, "-Action", action)
	} else {
		cmd = exec.Command("bash", script, action)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("VidOtter background %s failed: %w: %s", action, err, strings.TrimSpace(string(output)))
	}
	return nil
}
