//go:build unix

package main

import (
	"fmt"
	"os"
	"syscall"
)

// fileID returns a string identifying the underlying file (device and inode numbers). It is the same
// comparison as os.SameFile, turned into a string so that many files can be compared at once.
func fileID(fi os.FileInfo) (string, bool) {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return "", false
	}
	return fmt.Sprint(st.Dev, ":", st.Ino), true
}
