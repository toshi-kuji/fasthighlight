//go:build darwin || freebsd || netbsd || openbsd || dragonfly

package main

import (
	"syscall"
	"unsafe"
)

// isTerminal reports whether fd is a terminal. It checks whether the terminal settings can be read
// (ioctl TIOCGETA).
func isTerminal(fd uintptr) bool {
	var t syscall.Termios
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGETA, uintptr(unsafe.Pointer(&t)))
	return err == 0
}
