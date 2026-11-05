package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"context"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"
)

// Login registration is independent of the currently running worker. In
// particular, saving a preference never starts or stops that worker.
type startupBackend struct {
	platform, home, executable, uid, account string
	run                                      func(string, ...string) ([]byte, error)
}

func startupCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	hideWindow(cmd)
	if runtime.GOOS == "windows" {
		for _, value := range os.Environ() {
			if !strings.EqualFold(strings.SplitN(value, "=", 2)[0], "PSModulePath") {
				cmd.Env = append(cmd.Env, value)
			}
		}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("startup registration command failed (%s); check system permissions/session", name)
	}
	return out, nil
}

func systemStartup() startupBackend {
	home, _ := os.UserHomeDir()
	exe, _ := os.Executable()
	return startupBackend{runtime.GOOS, home, exe, fmt.Sprint(os.Getuid()), currentAccount(), startupCommand}
}

func (b startupBackend) name(root string, setup persistedSetup) string {
	if setup.Manager == "go-service" {
		return serviceName(root)
	}
	return serviceName(root) + "-Login"
}
func (b startupBackend) file(root string, setup persistedSetup) string {
	name := b.name(root, setup)
	if b.platform == "darwin" {
		if setup.Manager == "go-service" && nativeScope(root) == "system" {
			return filepath.Join("/Library/LaunchDaemons", name+".plist")
		}
		return filepath.Join(b.home, "Library/LaunchAgents", name+".plist")
	}
	base := filepath.Join(b.home, ".config/systemd/user")
	if setup.Manager == "go-service" && nativeScope(root) == "system" {
		base = "/etc/systemd/system"
	}
	return filepath.Join(base, name+".service")
}
func (b startupBackend) domain(root string, setup persistedSetup) string {
	if setup.Manager == "go-service" && nativeScope(root) == "system" {
		return "system"
	}
	return "gui/" + b.uid
}
func (b startupBackend) systemctl(root string, setup persistedSetup, args ...string) ([]byte, error) {
	if setup.Manager != "go-service" || nativeScope(root) == "user" {
		args = append([]string{"--user"}, args...)
	}
	return b.run("systemctl", args...)
}
func (b startupBackend) status(root string, setup persistedSetup) (bool, error) {
	name := b.name(root, setup)
	switch b.platform {
	case "windows":
		return b.windowsStartupStatus(root, setup)
	case "linux":
		out, err := b.systemctl(root, setup, "is-enabled", name+".service")
		value := strings.TrimSpace(string(out))
		if value == "enabled" {
			return true, nil
		}
		// is-enabled deliberately exits nonzero for disabled/not-found units.
		if value == "disabled" || value == "not-found" {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("unexpected systemd startup state: %s", value)
	case "darwin":
		if _, err := os.Stat(b.file(root, setup)); os.IsNotExist(err) {
			return false, nil
		} else if err != nil {
			return false, err
		}
		out, err := b.run("launchctl", "print-disabled", b.domain(root, setup))
		if err != nil {
			return false, err
		}
		return !strings.Contains(string(out), `"`+name+`" => true`), nil
	}
	return false, errors.New("login startup is unsupported on this platform")
}

func xmlText(s string) string {
	var out strings.Builder
	_ = xml.EscapeText(&out, []byte(s))
	return out.String()
}
func unitQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "%", "%%")
	return `"` + s + `"`
}
func (b startupBackend) apply(root string, setup persistedSetup, enable bool) error {
	if setup.Manager != "go-client" && setup.Manager != "go-service" {
		return errors.New("unsupported service manager")
	}
	if strings.ContainsAny(root+b.executable, "\r\n\x00") {
		return errors.New("unsupported installation path")
	}
	name := b.name(root, setup)
	action := "disable"
	if enable {
		action = "enable"
	}
	if b.platform == "windows" {
		return b.windowsStartupApply(root, setup, enable)
	}
	file := b.file(root, setup)
	if enable && setup.Manager == "go-client" {
		content := ""
		switch b.platform {
		case "linux":
			content = "[Unit]\nDescription=VidOtter MCP login worker\n[Service]\nType=simple\nWorkingDirectory=" + unitQuote(root) + "\nExecStart=" + unitQuote(strings.ReplaceAll(b.executable, "$", "$$")) + " --mcp-worker\nRestart=no\n[Install]\nWantedBy=default.target\n"
		case "darwin":
			content = `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict><key>Label</key><string>` + name + `</string><key>ProgramArguments</key><array><string>` + xmlText(b.executable) + `</string><string>--mcp-worker</string></array><key>WorkingDirectory</key><string>` + xmlText(root) + `</string><key>RunAtLoad</key><true/><key>KeepAlive</key><false/></dict></plist>`
		default:
			return errors.New("unsupported startup platform")
		}
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			return err
		}
		if err := writePrivate(file, []byte(content)); err != nil {
			return err
		}
	}
	switch b.platform {
	case "linux":
		if _, err := b.systemctl(root, setup, "daemon-reload"); err != nil {
			return err
		}
		if !enable {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				return nil
			}
		}
		_, err := b.systemctl(root, setup, action, name+".service")
		return err
	case "darwin":
		// Do not bootstrap here: RunAtLoad would start the worker during Save.
		_, err := b.run("launchctl", action, b.domain(root, setup)+"/"+name)
		return err
	}
	return errors.New("unsupported startup platform")
}

func (s persistedSetup) autoConnect() bool { return s.AutoConnect == nil || *s.AutoConnect }

// launchd's disabled override also blocks a manual load. Temporarily permit
// that load, then restore the login preference without stopping the service.
func (b startupBackend) manualServiceStart(root string, setup persistedSetup, start func() error) (result error) {
	if b.platform != "darwin" {
		return start()
	}
	enabled, err := b.status(root, setup)
	if err != nil {
		return err
	}
	if enabled {
		return start()
	}
	if err := b.apply(root, setup, true); err != nil {
		return err
	}
	defer func() {
		if err := b.apply(root, setup, false); err != nil {
			result = errors.Join(result, err)
		}
	}()
	return start()
}

func savePreferences(root, id, key string, startup, connect *bool) error {
	return savePreferencesWith(systemStartup(), root, id, key, startup, connect)
}
func savePreferencesWith(b startupBackend, root, id, key string, startup, connect *bool) (result error) {
	// Roll back connection files if the platform rejects startup registration.
	// User data, browser profiles and logs are outside this transaction.
	snapshots := map[string][]byte{}
	for _, name := range []string{"config.toml", "runtime/setup.json", "runtime/credentials.json", "runtime/tunnel.yaml"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		snapshots[name] = content
	}
	var undo func() error
	defer func() {
		if result == nil {
			return
		}
		if undo != nil {
			if err := undo(); err != nil {
				result = fmt.Errorf("%w; startup rollback failed: %v", result, err)
			}
		}
		for name, content := range snapshots {
			file := filepath.Join(root, name)
			var err error
			if content == nil {
				err = os.Remove(file)
				if os.IsNotExist(err) {
					err = nil
				}
			} else {
				err = writePrivate(file, content)
			}
			if err != nil {
				result = fmt.Errorf("%w; configuration rollback failed", result)
			}
		}
	}()

	// Validate and preserve credentials before touching OS registrations.
	if err := saveNative(root, id, key); err != nil {
		return err
	}
	s, err := readSetup(root)
	if err != nil {
		return err
	}
	if startup != nil {
		before, err := b.status(root, s)
		if err != nil {
			return err
		}
		if before != *startup {
			undo = func() error { return b.apply(root, s, before) }
			if err := b.apply(root, s, *startup); err != nil {
				return err
			}
		}
		actual, err := b.status(root, s)
		if err != nil || actual != *startup {
			return errors.New("could not verify system startup registration")
		}
		// Persist only after the platform confirms the change.
		s.Autostart = startup
	}
	if connect != nil && s.autoConnect() != *connect {
		s.AutoConnect = connect
		s.Revision = sessionToken()
	}
	raw, err := jsonMarshal(s)
	if err != nil {
		return err
	}
	return writePrivate(filepath.Join(root, "runtime", "setup.json"), raw)
}

// windowsLoginTask is a small native schtasks.exe adapter; it never invokes a
// scripting interpreter and refuses to take ownership of unrelated tasks.
func (b startupBackend) windowsLoginTask(root string, setup persistedSetup) (bool, bool, bool, error) {
	name := b.name(root, setup)
	data, err := b.run("schtasks.exe", "/query", "/tn", name, "/xml")
	if err != nil {
		msg := strings.ToLower(string(data))
		if strings.Contains(msg, "cannot find") || strings.Contains(msg, "does not exist") ||
			strings.Contains(msg, "cannot be found") || strings.Contains(msg, "找不到") ||
			strings.Contains(msg, "不存在") {
			return false, false, false, nil
		}
		return false, false, false, err
	}
	xmlData := windowsUTF8TaskXML(data)
	var task struct {
		Actions struct {
			Exec []struct {
				Command   string `xml:"Command"`
				Arguments string `xml:"Arguments"`
			} `xml:"Exec"`
		} `xml:"Actions"`
		Settings struct {
			Enabled string `xml:"Enabled"`
		} `xml:"Settings"`
	}
	if err := xml.Unmarshal(xmlData, &task); err != nil {
		return false, false, false, fmt.Errorf("invalid scheduled task XML: %w", err)
	}
	if len(task.Actions.Exec) != 1 {
		return false, false, false, errors.New("scheduled task not owned by this installation")
	}
	a := task.Actions.Exec[0]
	command := strings.Trim(a.Command, string(rune(34)))
	args := strings.ToLower(a.Arguments)
	worker := "serve-worker"
	if true {
		worker = "--mcp-worker"
	}
	owned := strings.EqualFold(command, b.executable) && strings.Contains(args, worker)
	legacy := !owned && strings.HasSuffix(strings.ToLower(command), "powershell.exe") &&
		strings.Contains(args, strings.ToLower(b.executable)) && strings.Contains(args, worker)
	// Older Remote Desktop used a VBScript wrapper. Only migrate the
	// exact launcher path from this installation, never another user's task.

	if !owned && !legacy {
		return false, false, false, errors.New("scheduled task not owned by this installation")
	}
	return true, !strings.EqualFold(strings.TrimSpace(task.Settings.Enabled), "false"), legacy, nil
}
func windowsUTF8TaskXML(data []byte) []byte {
	if len(data) >= 2 && ((data[0] == 0xff && data[1] == 0xfe) || (data[0] == 0xfe && data[1] == 0xff)) {
		little := data[0] == 0xff
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			if little {
				units = append(units, binary.LittleEndian.Uint16(data[i:i+2]))
			} else {
				units = append(units, binary.BigEndian.Uint16(data[i:i+2]))
			}
		}
		data = []byte(string(utf16.Decode(units)))
	} else if len(data) >= 4 && data[1] == 0 && data[3] == 0 {
		units := make([]uint16, 0, len(data)/2)
		for i := 0; i+1 < len(data); i += 2 {
			units = append(units, binary.LittleEndian.Uint16(data[i:i+2]))
		}
		data = []byte(string(utf16.Decode(units)))
	}
	return []byte(strings.Replace(string(data), "encoding=\"UTF-16\"", "encoding=\"UTF-8\"", 1))
}

const windowsRunKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func (b startupBackend) windowsRunCommand(root string) string {
	executable := b.executable
	// Prefer GUI-subsystem launcher for login; no console flashes.
	if strings.EqualFold(filepath.Base(filepath.Dir(executable)), "bin") {
		candidate := filepath.Join(filepath.Dir(executable), "vidotter-host.exe")
		if _, err := os.Stat(candidate); err == nil {
			executable = candidate
		}
	}
	return fmt.Sprintf("%q %s", executable, "--mcp-worker")
}

func (b startupBackend) windowsRunPresent(root string, setup persistedSetup) (bool, error) {
	name := b.name(root, setup)
	out, err := b.run("reg.exe", "query", windowsRunKey, "/v", name)
	if err != nil {
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "unable to find") ||
			strings.Contains(msg, "cannot find") ||
			strings.Contains(msg, "not found") ||
			strings.Contains(msg, "找不到") ||
			strings.Contains(msg, "无法找到") {
			return false, nil
		}
		return false, fmt.Errorf("query HKCU Run entry: %w", err)
	}
	want := b.windowsRunCommand(root)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.SplitN(strings.TrimSpace(line), "REG_SZ", 2)
		if len(fields) != 2 || strings.TrimSpace(fields[0]) != name {
			continue
		}
		value := strings.TrimSpace(fields[1])
		if !strings.EqualFold(value, want) {
			return false, errors.New("another installation owns the login startup entry")
		}
		return true, nil
	}
	return false, errors.New("unexpected registry Run entry data")
}
func (b startupBackend) windowsStartupStatus(root string, setup persistedSetup) (bool, error) {
	name := b.name(root, setup)
	if setup.Manager == "go-service" {
		out, err := b.run("sc.exe", "qc", name)
		if err != nil {
			return false, err
		}
		text := strings.ToUpper(string(out))
		if strings.Contains(text, "AUTO_START") {
			return true, nil
		}
		if strings.Contains(text, "DEMAND_START") || strings.Contains(text, "DISABLED") {
			return false, nil
		}
		return false, errors.New("unknown Windows service startup state")
	}
	present, err := b.windowsRunPresent(root, setup)
	if err != nil {
		return false, err
	}
	if present {
		return true, nil
	}
	_, enabled, _, err := b.windowsLoginTask(root, setup)
	return enabled, err
}
func (b startupBackend) windowsStartupApply(root string, setup persistedSetup, enable bool) error {
	name := b.name(root, setup)
	if setup.Manager == "go-service" {
		mode := "demand"
		if enable {
			mode = "auto"
		}
		_, err := b.run("sc.exe", "config", name, "start=", mode)
		return err
	}
	exists, enabled, _, err := b.windowsLoginTask(root, setup)
	if err != nil {
		return err
	}
	present, err := b.windowsRunPresent(root, setup)
	if err != nil {
		return err
	}
	// Disable an owned old Task Scheduler entry first, so upgrades cannot
	// run a PowerShell/VBS task concurrently with the native Run entry.
	if exists && enabled {
		if _, err := b.run("schtasks.exe", "/change", "/tn", name, "/disable"); err != nil {
			return fmt.Errorf("disable old login task before migration: %w", err)
		}
	}
	if enable {
		if present {
			return nil
		}
		_, err := b.run("reg.exe", "add", windowsRunKey, "/v", name, "/t", "REG_SZ",
			"/d", b.windowsRunCommand(root), "/f")
		return err
	}
	if present {
		_, err := b.run("reg.exe", "delete", windowsRunKey, "/v", name, "/f")
		return err
	}
	return nil
}
