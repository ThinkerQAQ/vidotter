package maint

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrepareInstallerPreservesData(t *testing.T) {
	release := t.TempDir()
	for path, content := range map[string]string{
		"bin/vidotterdev":            "binary",
		"extension/manifest.json":    `{"version":"1.6.2"}`,
		"LICENSE":                    "license",
		"Data/config.toml":           "secret = 'keep-me'",
		"Data/jobs.json":             "private jobs",
		"extension/popup/popup.html": "<html></html>",
	} {
		filename := filepath.Join(release, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination := t.TempDir()
	if err := prepareInstaller(destination, release, "1.6.2", "linux"); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(destination, "packages", "com.thinkerqaq.vidotter")
	for _, path := range []string{"data/Data/config.toml", "data/Data/jobs.json"} {
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Fatalf("private Data must not be packaged: %s, %v", path, err)
		}
	}
	for _, path := range []string{"data/bin/vidotterdev", "data/LICENSE", "data/extension/popup/popup.html", "meta/package.xml", "meta/installscript.qs"} {
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(path))); err != nil {
			t.Fatalf("missing %s: %v", path, err)
		}
	}
	script, err := os.ReadFile(filepath.Join(base, "meta", "installscript.qs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `"install"`) || !strings.Contains(string(script), `"uninstall"`) {
		t.Fatal("installer must register and unregister the Native Messaging host")
	}
	cfg, err := os.ReadFile(filepath.Join(destination, "config", "config.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cfg), "<RemoveTargetDir>false</RemoveTargetDir>") {
		t.Fatal("uninstall must leave local Data alone")
	}
}

func TestPrepareInstallerWindowsExecutable(t *testing.T) {
	release := t.TempDir()
	if err := os.WriteFile(filepath.Join(release, "LICENSE"), []byte("public"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination := t.TempDir()
	if err := prepareInstaller(destination, release, "1.6.2", "windows"); err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(destination, "packages", "com.thinkerqaq.vidotter", "meta", "installscript.qs"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "vidotterdev.exe") {
		t.Fatal("Windows must use the .exe host")
	}
}

// The binarycreator invocation must receive a complete offline package.
// This does not substitute for running Qt's real binarycreator on each OS.
func TestInstallerCallsBinarycreator(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("POSIX test stub; native platform GUI acceptance is separate")
	}
	stage := t.TempDir()
	for path, content := range map[string]string{
		"bin/vidotterdev":          "go",
		"bin/vidotter-linux-amd64": "bridge",
		"extension/manifest.json":  `{"version":"1.6.2"}`,
		"LICENSE":                  "license",
	} {
		file := filepath.Join(stage, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	stub := filepath.Join(t.TempDir(), "binarycreator")
	command := "#!/bin/sh\nfor last do :; done\nprintf 'mock' > \"$last\"\n"
	if err := os.WriteFile(stub, []byte(command), 0o755); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "VidOtter-Setup-v1.6.2-linux-amd64.run")
	if err := Installer(stage, "linux", "amd64", stub, output); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "mock" {
		t.Fatalf("fake binarycreator did not run: %q", content)
	}
}
