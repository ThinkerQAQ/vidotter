//go:build windows

package vidotter

import (
	"os/exec"
	"testing"
)

func TestWorkerCommandDoesNotCreateConsole(t *testing.T) {
	command, err := newWorkerCommand("test-token")
	if err != nil {
		t.Fatal(err)
	}
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow {
		t.Fatal("worker command must hide its window")
	}
	flags := command.SysProcAttr.CreationFlags
	if flags&createNoWindow == 0 || flags&createNewConsole != 0 {
		t.Fatalf("unexpected worker creation flags: %#x", flags)
	}
}

func TestMCPConfigurationNeverCreatesConsole(t *testing.T) {
	cmd := exec.Command("powershell.exe")
	hideMCPSetupWindow(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.HideWindow || cmd.SysProcAttr.CreationFlags&createNoWindow == 0 || cmd.SysProcAttr.CreationFlags&createNewConsole != 0 {
		t.Fatal("MCP setup must run without any console")
	}
}
