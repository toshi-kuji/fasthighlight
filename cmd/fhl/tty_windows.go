//go:build windows

package main

import "syscall"

// isTerminal reports whether fd is a console (whether GetConsoleMode succeeds).
func isTerminal(fd uintptr) bool {
	var mode uint32
	return syscall.GetConsoleMode(syscall.Handle(fd), &mode) == nil
}
