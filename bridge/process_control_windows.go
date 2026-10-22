//go:build windows

package vidotter

import (
	"os/exec"
	"strconv"
	"syscall"
)

func stopProcess(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	stop := exec.Command("taskkill.exe", "/PID", strconv.Itoa(command.Process.Pid), "/T", "/F")
	stop.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow, HideWindow: true}
	return stop.Run()
}
