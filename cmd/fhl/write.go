package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// writeStatus is the result of writing one file.
type writeStatus int

const (
	written   writeStatus = iota // written
	unchanged                    // not written because the contents are identical
	skipped                      // not written because N was answered at the overwrite prompt
)

// writer writes files according to the overwrite rules. It is called from a single goroutine, in input order.
type writer struct {
	force       bool
	interactive bool          // whether run from a terminal (whether it can ask before overwriting)
	answers     *bufio.Reader // reads the answers to the prompts
	prompt      io.Writer     // writes the prompts
	all         bool          // after a was answered
}

// write writes data to path. If the file already exists, the overwrite rules apply.
func (w *writer) write(path string, data []byte) (writeStatus, error) {
	old, err := os.ReadFile(path)
	exists := false
	switch {
	case err == nil:
		if bytes.Equal(old, data) {
			return unchanged, nil
		}
		exists = true
	case errors.Is(err, fs.ErrNotExist):
	default:
		if fi, serr := os.Stat(path); serr == nil && fi.IsDir() {
			return 0, fmt.Errorf("fhl: %s is a folder; choose another -o", path)
		}
		// A file that exists but cannot be read cannot be compared, so it counts as a different file.
		// (If it cannot be written either, an error is reported below.)
		_, lerr := os.Lstat(path)
		exists = lerr == nil
	}
	if exists && !w.force && !w.all {
		if !w.interactive {
			return 0, fmt.Errorf("fhl: %s already exists; use --force to overwrite it", path)
		}
		switch w.ask(path) {
		case "y":
		case "a":
			w.all = true
		default:
			return skipped, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, fmt.Errorf("fhl: cannot write %s: %v", path, cause(err))
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return 0, fmt.Errorf("fhl: cannot write %s: %v", path, cause(err))
	}
	return written, nil
}

// ask asks whether path may be overwritten and returns "y", "n" or "a". If no answer can be read, it returns "n".
func (w *writer) ask(path string) string {
	fmt.Fprintf(w.prompt, "fhl: overwrite %s? [y/N/a] ", path)
	line, err := w.answers.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(w.prompt)
		return "n"
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return "y"
	case "a", "all":
		return "a"
	}
	return "n"
}

// maxPending lets tests observe the maximum number of jobs not yet written out.
var maxPending atomic.Int64

// pipeline converts jobs with n goroutines (convert) and passes them to handle one at a time, in input order.
// The jobs not yet passed to handle (those being converted and those finished but waiting) are limited to 4 times n.
// A converted result is released right after it is passed to handle.
func pipeline[R any](jobs []*job, n int, convert func(*job) R, handle func(*job, R)) {
	limit := 4 * n
	results := make([]chan R, len(jobs))
	for i := range results {
		results[i] = make(chan R, 1)
	}
	sem := make(chan struct{}, limit)
	work := make(chan int)
	var pending atomic.Int64
	go func() {
		for i := range jobs {
			sem <- struct{}{}
			if p := pending.Add(1); p > maxPending.Load() {
				maxPending.Store(p)
			}
			work <- i
		}
		close(work)
	}()
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range work {
				results[i] <- convert(jobs[i])
			}
		}()
	}
	for i, j := range jobs {
		r := <-results[i]
		results[i] = nil
		handle(j, r)
		pending.Add(-1)
		<-sem
	}
	wg.Wait()
}
