package maint

import (
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

// Installer wraps an already packaged, platform-specific release with the Qt
// Installer Framework. It does not rebuild the Bridge or download dependencies.
// binarycreator is a build-time tool; users do not need Qt installed.
func Installer(stage, targetOS, targetArch, binarycreator, output string) error {
	if stage == "" {
		return errors.New("installer requires --stage pointing to an extracted matching release")
	}
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	if targetOS != runtime.GOOS {
		return fmt.Errorf("Qt IFW binarycreator creates host-platform installers: host=%s target=%s", runtime.GOOS, targetOS)
	}
	if binarycreator == "" {
		binarycreator = "binarycreator"
	}
	executable, err := exec.LookPath(binarycreator)
	if err != nil {
		return fmt.Errorf("Qt Installer Framework binarycreator not found (%s): %w", binarycreator, err)
	}
	stage, err = filepath.Abs(stage)
	if err != nil {
		return err
	}
	ver, err := version(stage)
	if err != nil {
		return fmt.Errorf("invalid release stage: %w", err)
	}
	platform := targetOS
	if platform == "darwin" {
		platform = "macos"
	}
	if platform != "windows" && platform != "macos" && platform != "linux" {
		return fmt.Errorf("unsupported installer OS: %s", targetOS)
	}
	suffix := ""
	if targetOS == "windows" {
		suffix = ".exe"
	}
	for _, f := range []string{
		"bin/vidotterdev" + suffix,
		fmt.Sprintf("bin/vidotter-%s-%s%s", platform, targetArch, suffix),
		"extension/manifest.json",
		"LICENSE",
	} {
		if info, err := os.Stat(filepath.Join(stage, filepath.FromSlash(f))); err != nil || info.IsDir() {
			return fmt.Errorf("release missing required file %s: %v", f, err)
		}
	}
	if output == "" {
		extension := ".run"
		if targetOS == "windows" {
			extension = ".exe"
		} else if targetOS == "darwin" {
			extension = ".dmg"
		}
		output = filepath.Join(filepath.Dir(stage), fmt.Sprintf("VidOtter-Setup-v%s-%s-%s%s", ver, platform, targetArch, extension))
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if err := ensureDir(filepath.Dir(output)); err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "vidotter-ifw-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := prepareInstaller(dir, stage, ver, targetOS); err != nil {
		return err
	}
	args := []string{"--offline-only", "-p", filepath.Join(dir, "packages"), "-c", filepath.Join(dir, "config", "config.xml"), output}
	fmt.Println("+", executable, strings.Join(args, " "))
	command := exec.Command(executable, args...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("binarycreator failed: %w", err)
	}
	fmt.Println("Installer:", output)
	return nil
}

type installerConfig struct {
	XMLName             xml.Name `xml:"Installer"`
	Name                string   `xml:"Name"`
	Version             string   `xml:"Version"`
	Title               string   `xml:"Title"`
	Publisher           string   `xml:"Publisher"`
	StartMenuDir        string   `xml:"StartMenuDir"`
	TargetDir           string   `xml:"TargetDir"`
	MaintenanceToolName string   `xml:"MaintenanceToolName"`
	RemoveTargetDir     string   `xml:"RemoveTargetDir"`
}

type installerLicense struct {
	Name string `xml:"name,attr"`
	File string `xml:"file,attr"`
}

type installerLicenses struct {
	License installerLicense `xml:"License"`
}

type installerPackage struct {
	XMLName     xml.Name          `xml:"Package"`
	DisplayName string            `xml:"DisplayName"`
	Description string            `xml:"Description"`
	Version     string            `xml:"Version"`
	ReleaseDate string            `xml:"ReleaseDate"`
	Default     string            `xml:"Default"`
	Script      string            `xml:"Script"`
	Licenses    installerLicenses `xml:"Licenses"`
}

// prepareInstaller only repackages verified release files. Data/ is never
// included in a managed package, so upgrades cannot overwrite local state.
func prepareInstaller(destination, stage, ver, targetOS string) error {
	config := installerConfig{
		Name: "VidOtter", Version: ver, Title: "VidOtter Setup",
		Publisher: "ThinkerQAQ", StartMenuDir: "VidOtter",
		TargetDir: "@HomeDir@/VidOtter", MaintenanceToolName: "VidOtter-Maintenance",
		RemoveTargetDir: "false",
	}
	if err := installerXML(filepath.Join(destination, "config", "config.xml"), config); err != nil {
		return err
	}
	pkg := installerPackage{
		DisplayName: "VidOtter", Description: "VidOtter browser extension and local Bridge",
		Version: ver, ReleaseDate: time.Now().UTC().Format("2006-01-02"),
		Default: "true", Script: "installscript.qs",
		Licenses: installerLicenses{License: installerLicense{Name: "Apache-2.0 (VidOtter original code)", File: "license.txt"}},
	}
	meta := filepath.Join(destination, "packages", "com.thinkerqaq.vidotter", "meta")
	if err := installerXML(filepath.Join(meta, "package.xml"), pkg); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(stage, "LICENSE"), filepath.Join(meta, "license.txt")); err != nil {
		return err
	}
	bridgeInstaller := "vidotterdev"
	if targetOS == "windows" {
		bridgeInstaller += ".exe"
	}
	// IFW runs extracted binaries after deployment, and executes the undo
	// action before removing them. The Go installer retains user Data/.
	script := fmt.Sprintf(`function Component() {}

Component.prototype.createOperations = function() {
    component.createOperations();
    component.addOperation("Execute",
        "@TargetDir@/bin/%s", "install",
        "UNDOEXECUTE", "@TargetDir@/bin/%s", "uninstall");
}
`, bridgeInstaller, bridgeInstaller)
	if err := os.WriteFile(filepath.Join(meta, "installscript.qs"), []byte(script), 0o644); err != nil {
		return err
	}
	payload := filepath.Join(destination, "packages", "com.thinkerqaq.vidotter", "data")
	return copyTree(stage, payload, func(relative string, isDir bool) bool {
		name := filepath.ToSlash(relative)
		return name != "Data" && !strings.HasPrefix(name, "Data/") &&
			name != ".build" && !strings.HasPrefix(name, ".build/")
	})
}

func installerXML(path string, value any) error {
	data, err := xml.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(xml.Header), append(data, '\n')...), 0o644)
}
