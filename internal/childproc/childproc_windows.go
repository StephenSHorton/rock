//go:build windows

package childproc

import (
	"os/exec"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW: the child gets a console that is
// never shown and is not Rock's console.
const createNoWindow = 0x08000000

func isolate(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= createNoWindow
	cmd.SysProcAttr.HideWindow = true
}

// Flags reports the creation flags Isolate sets (tests).
func Flags(cmd *exec.Cmd) uint32 {
	if cmd == nil || cmd.SysProcAttr == nil {
		return 0
	}
	return cmd.SysProcAttr.CreationFlags
}
