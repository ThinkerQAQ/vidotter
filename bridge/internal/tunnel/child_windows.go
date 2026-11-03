//go:build windows

package tunnel

import (
	"golang.org/x/sys/windows"
	"os/exec"
	"sync"
	"unsafe"
)

// A private job owns exactly this Tunnel and its MCP descendants. Closing the
// job also handles an unexpected supervisor exit without PID enumeration.
func launchChild(cmd *exec.Cmd) (func() error, error) {
	hideWindow(cmd)
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		windows.CloseHandle(process)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	var once sync.Once
	var stopped error
	return func() error { once.Do(func() { stopped = windows.CloseHandle(job) }); return stopped }, nil
}
