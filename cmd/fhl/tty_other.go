//go:build !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly && !windows

package main

// isTerminal always returns false on an OS with no way to detect a terminal (so nothing is overwritten
// without checking).
func isTerminal(fd uintptr) bool { return false }
