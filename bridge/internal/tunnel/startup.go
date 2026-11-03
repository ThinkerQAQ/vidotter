package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func (b startupBackend) powershell(script string) ([]byte, error) {
	return b.run("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[Text.UTF8Encoding]::new($false); "+script)
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
		var script string
		if setup.Manager == "go-service" {
			script = "(Get-CimInstance Win32_Service -Filter " + psQuote("Name='"+name+"'") + " -ErrorAction Stop).StartMode | ConvertTo-Json -Compress"
		} else {
			script = "$t=Get-ScheduledTask -TaskPath '\\' -ErrorAction Stop | Where-Object TaskName -eq " + psQuote(name) + "; if ($null -eq $t) {'false'} else {$t.Settings.Enabled | ConvertTo-Json -Compress}"
		}
		out, err := b.powershell(script)
		if err != nil {
			return false, err
		}
		if setup.Manager == "go-service" {
			return strings.TrimSpace(string(out)) == `"Auto"`, nil
		}
		var enabled bool
		if json.Unmarshal([]byte(strings.TrimSpace(string(out))), &enabled) != nil {
			return false, errors.New("invalid Task Scheduler status")
		}
		return enabled, nil
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
		if setup.Manager == "go-service" {
			mode := "Manual"
			if enable {
				mode = "Automatic"
			}
			_, err := b.powershell("Set-Service -Name " + psQuote(name) + " -StartupType " + mode)
			return err
		}
		if !enable {
			_, err := b.powershell("$t=Get-ScheduledTask -TaskPath '\\' -ErrorAction Stop | Where-Object TaskName -eq " + psQuote(name) + "; if ($null -ne $t) { $t | Disable-ScheduledTask | Out-Null }")
			return err
		}
		// Interactive, least-privilege logon task: no stored password and no
		// unsupported Windows 'user service'. The hidden wrapper waits for the worker.
		command := "& " + psQuote(b.executable) + " --mcp-worker; exit $LASTEXITCODE"
		args := `-NoProfile -NonInteractive -WindowStyle Hidden -Command "` + strings.ReplaceAll(command, `"`, `\"`) + `"`
		script := "$a=New-ScheduledTaskAction -Execute 'powershell.exe' -Argument " + psQuote(args) + " -WorkingDirectory " + psQuote(root) + "; " +
			"$p=New-ScheduledTaskPrincipal -UserId " + psQuote(b.account) + " -LogonType Interactive -RunLevel Limited; " +
			"$t=New-ScheduledTaskTrigger -AtLogOn -User " + psQuote(b.account) + "; " +
			"$s=New-ScheduledTaskSettingsSet -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries; " +
			"Register-ScheduledTask -TaskName " + psQuote(name) + " -Action $a -Principal $p -Trigger $t -Settings $s -Force | Out-Null"
		_, err := b.powershell(script)
		return err
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
