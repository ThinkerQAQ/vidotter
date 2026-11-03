package maint

import (
	"bytes"
	"embed"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Windows GUI entrypoints are adapted from RemoteBrowserMCP PR #17.
//
//go:embed assets/VidOtter-WindowsLauncher.cs
var vidOtterWindowsLauncher embed.FS

// Qt IFW already uses a GUI-subsystem installer template. These WinExe
// launchers also provide branded, console-free configuration and management.
func buildWindowsInstallerLaunchers(stage, payload, temp string) error {
	if runtime.GOOS != "windows" {
		return errors.New("Windows GUI launcher compilation requires a Windows host")
	}
	iconPNG, err := os.ReadFile(filepath.Join(stage, "extension", "icons", "icon-128.png"))
	if err != nil {
		return fmt.Errorf("missing VidOtter icon: %w", err)
	}
	icon, err := windowsPNGIcon(iconPNG)
	if err != nil {
		return err
	}
	iconPath := filepath.Join(payload, "assets", "vidotter.ico")
	if err := os.MkdirAll(filepath.Dir(iconPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(iconPath, icon, 0644); err != nil {
		return err
	}
	csharp, err := vidOtterWindowsLauncher.ReadFile("assets/VidOtter-WindowsLauncher.cs")
	if err != nil {
		return err
	}
	source := filepath.Join(temp, "VidOtter-WindowsLauncher.cs")
	if err := os.WriteFile(source, csharp, 0644); err != nil {
		return err
	}
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = "C:\\Windows"
	}
	compiler := filepath.Join(systemRoot, "Microsoft.NET", "Framework64", "v4.0.30319", "csc.exe")
	if _, err := os.Stat(compiler); err != nil {
		return fmt.Errorf("Windows .NET Framework C# compiler required for GUI installer: %w", err)
	}
	for _, spec := range []struct{ name, define string }{
		{"VidOtter-Configure.exe", "CONFIGURE"},
		{"VidOtter-Manager.exe", "MAINTENANCE"},
	} {
		dest := filepath.Join(payload, "bin", spec.name)
		if spec.define == "MAINTENANCE" {
			dest = filepath.Join(payload, spec.name)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		args := []string{
			"/nologo", "/target:winexe", "/define:" + spec.define,
			"/reference:System.Windows.Forms.dll", "/reference:System.Drawing.dll",
			"/win32icon:" + iconPath, "/out:" + dest, source,
		}
		command := exec.Command(compiler, args...)
		output, err := command.CombinedOutput()
		if err != nil {
			return fmt.Errorf("compile %s: %w: %s", spec.name, err, strings.TrimSpace(string(output)))
		}
		if err := requireWindowsGUI(dest); err != nil {
			return err
		}
	}
	return nil
}

// ICO format supports PNG-encoded images; the resource preserves VidOtter's
// own 128px app icon, not another product's icon.
func windowsPNGIcon(png []byte) ([]byte, error) {
	signature := []byte{137, 80, 78, 71, 13, 10, 26, 10}
	if len(png) < 24 || !bytes.Equal(png[:8], signature) {
		return nil, errors.New("VidOtter icon is not a PNG")
	}
	width := binary.BigEndian.Uint32(png[16:20])
	height := binary.BigEndian.Uint32(png[20:24])
	if width != 128 || height != 128 {
		return nil, fmt.Errorf("expected 128x128 VidOtter icon, got %dx%d", width, height)
	}
	var b bytes.Buffer
	for _, v := range []uint16{0, 1, 1} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.Write([]byte{128, 128, 0, 0})
	for _, v := range []uint16{1, 32} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	for _, v := range []uint32{uint32(len(png)), 22} {
		_ = binary.Write(&b, binary.LittleEndian, v)
	}
	b.Write(png)
	return b.Bytes(), nil
}
