package maint

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWindowsGUIInstallerLaunchers(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows WinExe test")
	}
	iconPath := filepath.Join("..", "..", "..", "..", "extension", "icons", "icon-128.png")
	icon, err := os.ReadFile(iconPath)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	asset := filepath.Join(stage, "extension", "icons", "icon-128.png")
	if err := os.MkdirAll(filepath.Dir(asset), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(asset, icon, 0644); err != nil {
		t.Fatal(err)
	}
	payload := t.TempDir()
	binary := filepath.Join(payload, "bin", "vidotter-windows-amd64.exe")
	if err := os.MkdirAll(filepath.Dir(binary), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("self-test-only"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := buildWindowsInstallerLaunchers(stage, payload, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"VidOtter-Configure.exe", "VidOtter-Manager.exe"} {
		exe := filepath.Join(payload, "bin", name)
		if name == "VidOtter-Manager.exe" {
			exe = filepath.Join(payload, name)
		}
		if err := requireWindowsGUI(exe); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if out, err := exec.Command(exe, "--self-test").CombinedOutput(); err != nil {
			t.Fatalf("%s self-test: %v: %s", name, err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(payload, "assets", "vidotter.ico")); err != nil {
		t.Fatalf("branded application icon not packaged: %v", err)
	}
}

func TestPNGIconFormatValidation(t *testing.T) {
	png := make([]byte, 24)
	copy(png, []byte{137, 80, 78, 71, 13, 10, 26, 10})
	binary.BigEndian.PutUint32(png[16:], 128)
	binary.BigEndian.PutUint32(png[20:], 128)
	ico, err := windowsPNGIcon(png)
	if err != nil || len(ico) != 22+len(png) {
		t.Fatalf("ICO wrap failed: %d %v", len(ico), err)
	}
	if _, err := windowsPNGIcon([]byte("not a PNG")); err == nil {
		t.Fatal("invalid PNG icon accepted")
	}
}
