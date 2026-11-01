package maint

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// UpdateRepository produces the standard Qt IFW repository from a complete,
// platform-matching release stage. It is intended for release CI (not end users).
// The result is consumed by the installed Maintenance Tool's update operation.
func UpdateRepository(stage, targetOS, targetArch, repogen, output string) error {
	if stage == "" || output == "" {
		return errors.New("update-repository requires --stage and --output")
	}
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	if targetOS != runtime.GOOS {
		return fmt.Errorf("Qt IFW repogen must run on target host: host=%s target=%s", runtime.GOOS, targetOS)
	}
	if repogen == "" {
		repogen = "repogen"
	}
	binary, err := exec.LookPath(repogen)
	if err != nil {
		return fmt.Errorf("Qt IFW repogen not found: %w", err)
	}
	stage, err = filepath.Abs(stage)
	if err != nil {
		return err
	}
	ver, err := version(stage)
	if err != nil {
		return err
	}
	osName := targetOS
	if osName == "darwin" {
		osName = "macos"
	}
	suffix := ""
	if targetOS == "windows" {
		suffix = ".exe"
	}
	for _, path := range []string{
		"bin/vidotterdev" + suffix,
		fmt.Sprintf("bin/vidotter-%s-%s%s", osName, targetArch, suffix),
		"extension/manifest.json",
		"LICENSE",
	} {
		stat, err := os.Stat(filepath.Join(stage, filepath.FromSlash(path)))
		if err != nil || stat.IsDir() {
			return fmt.Errorf("update stage missing %s: %v", path, err)
		}
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("update repository already exists: %s (use a new output directory)", output)
	} else if !os.IsNotExist(err) {
		return err
	}
	staging, err := os.MkdirTemp("", "vidotter-repogen-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	if err := prepareInstaller(staging, stage, ver, targetOS); err != nil {
		return err
	}
	if err := ensureDir(filepath.Dir(output)); err != nil {
		return err
	}
	cmd := exec.Command(binary, "-p", filepath.Join(staging, "packages"), output)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	fmt.Println("+", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		os.RemoveAll(output) // ensure partial repos are not published
		return fmt.Errorf("repogen failed: %w", err)
	}
	if _, err := os.Stat(filepath.Join(output, "Updates.xml")); err != nil {
		os.RemoveAll(output)
		return fmt.Errorf("repogen produced no Updates.xml: %w", err)
	}
	fmt.Println("Update repository:", output)
	return nil
}
