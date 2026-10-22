//go:build !windows

package vidotter

import "os/exec"

func configureSidecarCommand(command *exec.Cmd) {
	command.Stdin = nil
}
