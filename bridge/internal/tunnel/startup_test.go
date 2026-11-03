package tunnel

// Adapted from ThinkerQAQ/remote-browser-mcp client at 789d085 (#17, #18), Apache-2.0.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreferencesPreserveConnectionAndApplyOnlyVerifiedStartup(t *testing.T) {
	root := t.TempDir()
	if err := saveNative(root, "tunnel_TEST", "synthetic-key"); err != nil {
		t.Fatal(err)
	}
	first, _ := readSetup(root)
	enabled := false
	var commands []string
	b := startupBackend{platform: "windows", executable: `C:\Program Files\Example's App\client.exe`, account: `USER\example`}
	b.run = func(name string, args ...string) ([]byte, error) {
		command := strings.Join(args, " ")
		commands = append(commands, command)
		if strings.Contains(command, "Register-ScheduledTask") {
			enabled = true
		}
		if strings.Contains(command, "Disable-ScheduledTask") {
			enabled = false
		}
		if enabled {
			return []byte("true"), nil
		}
		return []byte("false"), nil
	}
	yes, no := true, false
	if err := savePreferencesWith(b, root, first.ID, "", &yes, &no); err != nil {
		t.Fatal(err)
	}
	s, _ := readSetup(root)
	key, _ := savedKey(root)
	if s.ID != first.ID || key != "synthetic-key" || s.autoConnect() || s.Autostart == nil || !*s.Autostart || s.Revision == first.Revision {
		t.Fatalf("preferences not persisted: %+v", s)
	}
	for _, command := range commands {
		if strings.Contains(command, "Start-ScheduledTask") || strings.Contains(command, "synthetic-key") {
			t.Fatal("Save started service or leaked secret")
		}
	}
	revision := s.Revision
	if err := savePreferencesWith(b, root, s.ID, "", &yes, &no); err != nil {
		t.Fatal(err)
	}
	s, _ = readSetup(root)
	if s.Revision != revision {
		t.Fatal("identical preference save invalidated worker")
	}
	if err := savePreferencesWith(b, root, s.ID, "", &no, nil); err != nil {
		t.Fatal(err)
	}
	s, _ = readSetup(root)
	if s.Revision != revision || s.autoConnect() {
		t.Fatal("login startup changed Tunnel preference or revision")
	}
	if err := saveNative(root, "tunnel_UPDATED", ""); err != nil {
		t.Fatal(err)
	}
	s, _ = readSetup(root)
	if s.autoConnect() || s.Autostart == nil || *s.Autostart {
		t.Fatal("connection update lost preferences")
	}
}

func TestStartupFailureIsNotPersisted(t *testing.T) {
	root := t.TempDir()
	if err := saveNative(root, "tunnel_TEST", "synthetic-key"); err != nil {
		t.Fatal(err)
	}
	b := startupBackend{platform: "windows", run: func(_ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "Register-ScheduledTask") {
			return nil, errors.New("denied")
		}
		return []byte("false"), nil
	}}
	yes := true
	before, _ := os.ReadFile(filepath.Join(root, "runtime", "setup.json"))
	if savePreferencesWith(b, root, "tunnel_CHANGED", "replacement-key", &yes, nil) == nil {
		t.Fatal("registration failure swallowed")
	}
	s, _ := readSetup(root)
	if s.Autostart != nil {
		t.Fatal("failed registration persisted as enabled")
	}
	after, _ := os.ReadFile(filepath.Join(root, "runtime", "setup.json"))
	key, _ := savedKey(root)
	if string(before) != string(after) || key != "synthetic-key" {
		t.Fatal("failed registration changed saved connection")
	}
}

func TestUnixStartupRegistrationDoesNotRunWorker(t *testing.T) {
	for _, platform := range []string{"linux", "darwin"} {
		t.Run(platform, func(t *testing.T) {
			root, home := t.TempDir(), t.TempDir()
			b := startupBackend{platform: platform, home: home, executable: `/Apps/A "quoted" $app%/client`, uid: "501"}
			var commands []string
			b.run = func(name string, args ...string) ([]byte, error) {
				commands = append(commands, name+" "+strings.Join(args, " "))
				return nil, nil
			}
			s := persistedSetup{Manager: "go-client"}
			if err := b.apply(root, s, true); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(b.file(root, s))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "--mcp-worker") || strings.Contains(string(raw), "synthetic-key") {
				t.Fatal("invalid login registration")
			}
			for _, cmd := range commands {
				if strings.Contains(cmd, "bootstrap") || strings.Contains(cmd, "--now") || strings.Contains(cmd, " start ") {
					t.Fatal("registration starts worker")
				}
			}
			if err := b.apply(root, s, false); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(b.file(root, s), filepath.Clean(home)+string(os.PathSeparator)) {
				t.Fatal("registration escaped user home")
			}
		})
	}
}

func TestOldSetupKeepsAutomaticTunnelConnection(t *testing.T) {
	if !(persistedSetup{}).autoConnect() {
		t.Fatal("upgrade changed existing connection behavior")
	}
}

func TestManualLaunchdServiceStartPreservesDisabledLoginPreference(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			root, home := t.TempDir(), t.TempDir()
			s := persistedSetup{Manager: "go-service"}
			b := startupBackend{platform: "darwin", home: home, uid: "501"}
			file := b.file(root, s)
			if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(file, []byte("plist"), 0600); err != nil {
				t.Fatal(err)
			}
			var actions []string
			b.run = func(name string, args ...string) ([]byte, error) {
				if args[0] == "print-disabled" {
					return []byte(`"` + b.name(root, s) + `" => true`), nil
				}
				actions = append(actions, args[0])
				return nil, nil
			}
			err := b.manualServiceStart(root, s, func() error {
				actions = append(actions, "start")
				if fail {
					return errors.New("failed load")
				}
				return nil
			})
			if (err != nil) != fail || strings.Join(actions, ",") != "enable,start,disable" {
				t.Fatalf("incorrect manual start: %v %v", actions, err)
			}
		})
	}
}
