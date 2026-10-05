//go:build !windows

package childproc

import "os/exec"

func isolate(*exec.Cmd) {}

// Flags reports the creation flags Isolate sets (tests). Always 0 off Windows.
func Flags(*exec.Cmd) uint32 { return 0 }
