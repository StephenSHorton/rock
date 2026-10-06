//go:build linux

package tui

import (
	"os"

	"golang.org/x/sys/unix"
)

// ttyErase returns the terminal's erase character (stty erase): the byte
// its Backspace key sends.
func ttyErase() (byte, bool) {
	t, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		return 0, false
	}
	return t.Cc[unix.VERASE], true
}
