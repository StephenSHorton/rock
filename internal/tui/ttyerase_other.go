//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package tui

func ttyErase() (byte, bool) { return 0, false }
