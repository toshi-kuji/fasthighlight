//go:build linux

package main

import (
	"syscall"
	"unsafe"
)

// isTerminal reports whether fd is a terminal. It checks whether the terminal settings can be read
// (ioctl TCGETS).
func isTerminal(fd uintptr) bool {
	var t syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TCGETS, uintptr(unsafe.Pointer(&t)))
	return err == 0
}
