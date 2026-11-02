package maint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingleUserInstallOwnership(t *testing.T) {
	first := filepath.Join(t.TempDir(), "A", "VidOtter")
	second := filepath.Join(t.TempDir(), "B", "VidOtter")
	for _, tc := range []struct {
		name         string
		registered   []string
		wantConflict bool
	}{
		{"fresh install", nil, false},
		{"in-place upgrade", []string{first, first}, false},
		{"second install blocked", []string{first, second}, true},
		{"second install blocked even if no first registration", []string{second}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := checkInstallOwnership(first, tc.registered, "linux")
			if (err != nil) != tc.wantConflict {
				t.Fatalf("conflict=%v, want=%v: %v", err != nil, tc.wantConflict, err)
			}
		})
	}
	if !sameInstallRoot(`C:\Users\User\VidOtter`, `c:\users\user\vidotter`, "windows") {
		t.Fatal("Windows path comparisons must ignore case")
	}
	if sameInstallRoot(first, second, "linux") {
		t.Fatal("different installation directories must never share ownership")
	}
}

func TestScheduledTaskOwnerParsing(t *testing.T) {
	// schtasks.exe /XML declares UTF-16 but emits UTF-8 on Windows.
	data := []byte(`<?xml version="1.0" encoding="UTF-16"?>
<Task><Actions Context="Author"><Exec><Command>powershell.exe</Command><WorkingDirectory>C:\Users\A\VidOtter</WorkingDirectory></Exec></Actions></Task>`)
	got, err := parseScheduledTaskOwner(data)
	if err != nil || got != `C:\Users\A\VidOtter` {
		t.Fatalf("task owner: %q, %v", got, err)
	}
	for _, invalid := range []string{
		`<Task><Actions><Exec><Command>powershell.exe</Command></Exec></Actions></Task>`,
		`<?xml encoding="UTF-16"`,
		`not xml`,
	} {
		if _, err := parseScheduledTaskOwner([]byte(invalid)); err == nil {
			t.Fatal("ambiguous task owner accepted")
		}
	}
}

func TestBrowserManifestOwnerAndNonOwner(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "installed", "VidOtter")
	file := filepath.Join(nativeDirs(home)[0], hostName+".json")
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	config := nativeHost{Name: hostName, Path: filepath.Join(root, "bin", "vidotter-linux-amd64")}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	registered, err := installedRootsFromNativeManifests(home, "linux")
	if err != nil || len(registered) != 1 || registered[0] != root {
		t.Fatalf("manifest owner mismatch: %#v, %v", registered, err)
	}
	if err := checkInstallOwnership(root, registered, "linux"); err != nil {
		t.Fatalf("same root upgrade rejected: %v", err)
	}
	if err := checkInstallOwnership(filepath.Join(home, "second"), registered, "linux"); err == nil {
		t.Fatal("second install overwrote Native Messaging registration")
	}
	config.Name = "another-host"
	data, _ = json.Marshal(config)
	os.WriteFile(file, data, 0600)
	if _, err := installedRootsFromNativeManifests(home, "linux"); err == nil {
		t.Fatal("unexpected existing host accepted")
	}
}

func TestUninstallLifecycleScriptsNeverPurgePrivateData(t *testing.T) {
	for _, rel := range []string{
		"../../../../scripts/Manage-VidOtter-Background.ps1",
		"../../../../scripts/manage-background.sh",
	} {
		script, err := os.ReadFile(rel)
		if err != nil {
			t.Fatal(err)
		}
		for _, term := range []string{"uninstall", "restore", "preferences"} {
			if !strings.Contains(string(script), term) {
				t.Errorf("%s missing %s lifecycle handling", rel, term)
			}
		}
	}
}
