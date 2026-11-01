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
	if !strings.Contains(string(script), `"install", "--root", "@TargetDir@"`) || !strings.Contains(string(script), `"uninstall", "--root", "@TargetDir@"`) {
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
	if err := Installer(stage, "linux", "amd64", stub, output, ""); err != nil {
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

func TestInstallerOptionalUpdateRepository(t *testing.T) {
	repo := t.TempDir()
	release := t.TempDir()
	if err := os.WriteFile(filepath.Join(release, "LICENSE"), []byte("license"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := prepareInstallerWithUpdateURL(repo, release, "1.6.2", "linux", "file:///tmp/vidotter-updates"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(repo, "config", "config.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<RemoteRepositories>") || !strings.Contains(string(data), "file:///tmp/vidotter-updates") {
		t.Fatalf("installer is missing update repository: %s", data)
	}
	if err := prepareInstallerWithUpdateURL(t.TempDir(), release, "1.6.2", "linux", "http://bad.example.com"); err == nil {
		t.Fatal("insecure public update repository must be rejected")
	}
}

func TestInstallerUpdateShortcuts(t *testing.T) {
	stage := t.TempDir()
	if err := os.WriteFile(filepath.Join(stage, "LICENSE"), []byte("license"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, pair := range []struct{ os, expected string }{
		{"windows", "Update VidOtter.lnk"},
		{"linux", "vidotter-update.desktop"},
	} {
		out := t.TempDir()
		if err := prepareInstallerWithUpdateURL(out, stage, "1.6.2", pair.os, "file:///tmp/vidotter-feed"); err != nil {
			t.Fatal(err)
		}
		script, err := os.ReadFile(filepath.Join(out, "packages", "com.thinkerqaq.vidotter", "meta", "installscript.qs"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(script), pair.expected) {
			t.Fatalf("%s update shortcut missing: %s", pair.os, script)
		}
		if pair.os == "windows" && !strings.Contains(string(script), "addStopProcessForUpdateRequest") {
			t.Fatal("Windows updates must request Bridge shutdown")
		}
	}
}

// Every platform gets one-product GUI and a finish-page activation guide.
// Windows additionally ships a re-openable setup helper and Start shortcuts.
// No test opens a browser or enables a Tunnel.
func TestPrepareInstallerFirstRunOnboarding(t *testing.T) {
	release := t.TempDir()
	if err := os.WriteFile(filepath.Join(release, "LICENSE"), []byte("Apache-2.0"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"windows", "darwin", "linux"} {
		t.Run(platform, func(t *testing.T) {
			dir := t.TempDir()
			if err := prepareInstallerWithUpdateURL(dir, release, "1.6.2", platform, ""); err != nil {
				t.Fatal(err)
			}
			base := filepath.Join(dir, "packages", "com.thinkerqaq.vidotter")
			cfg, err := os.ReadFile(filepath.Join(dir, "config", "config.xml"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(cfg), "<ControlScript>controller.qs</ControlScript>") {
				t.Fatal("missing top-level Qt controller")
			}
			controller, err := os.ReadFile(filepath.Join(dir, "config", "controller.qs"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(controller), "setDefaultPageVisible(QInstaller.ComponentSelection, false)") {
				t.Fatal("redundant component selection must be hidden")
			}
			ui, err := os.ReadFile(filepath.Join(base, "meta", "first-run.ui"))
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range []string{"openExtensionCheckBox"} {
				if !strings.Contains(string(ui), id) {
					t.Fatalf("missing checkbox %q", id)
				}
			}
			script, err := os.ReadFile(filepath.Join(base, "meta", "installscript.qs"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(script), "Connect ChatGPT MCP.lnk") || strings.Contains(string(ui), "configureMcpCheckBox") {
				t.Fatal("ChatGPT activation must be configured from VidOtter Settings")
			}
			if !strings.Contains(string(script), "addWizardPageItem") ||
				!strings.Contains(string(script), "finishButtonClicked.connect") {
				t.Fatal("installer finish page lacks activation hook")
			}
			powershell := filepath.Join(base, "data", "scripts", "VidOtter-FirstRun.ps1")
			if platform == "windows" {
				if _, err := os.Stat(powershell); err != nil {
					t.Fatal(err)
				}
				for _, shortcut := range []string{"Install Browser Extension.lnk"} {
					if !strings.Contains(string(script), shortcut) {
						t.Fatalf("missing Start menu item %s", shortcut)
					}
				}
			} else if _, err := os.Stat(powershell); !os.IsNotExist(err) {
				t.Fatalf("unexpected Windows-specific script on %s", platform)
			}
		})
	}
}
