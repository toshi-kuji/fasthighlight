//go:build !unix

package main

import "os"

// fileID returns a string identifying the underlying file. It cannot be built on this OS, so files are
// compared by path (the caller compares them case-insensitively).
func fileID(fi os.FileInfo) (string, bool) { return "", false }
